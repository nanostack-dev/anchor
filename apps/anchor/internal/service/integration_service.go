package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"anchor/internal/domain/integration"
	"anchor/internal/integration/provider"
	clerkprovider "anchor/internal/integration/provider/clerk"
	"anchor/internal/repository"
	serviceconfig "anchor/internal/service/config"

	"github.com/nanostack-dev/pgkit/pglock"
	"github.com/nanostack-dev/pgkit/pgqueue"

	apierror "github.com/nanostack-dev/nanostack-framework/pkg/apierror"
	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/rs/zerolog"
)

const (
	entityTypeUnknown      = "unknown"
	auditFieldEventID      = "event_id"
	auditFieldEventType    = "event_type"
	auditFieldProductID    = "product_id"
	auditFieldProviderType = "provider_type"
	auditFieldQueueAttempt = "queue_attempt"
	auditFieldQueueMax     = "queue_max_attempts"
	auditFieldStatus       = "status"
)

type integrationQueuePayload struct {
	EventID string `json:"event_id"`
}

// integrationReconcileQueuePayload is the payload for a per-instance reconcile job.
// When IntegrationInstanceID is empty the job is a scheduler that enqueues per-instance jobs.
type integrationReconcileQueuePayload struct {
	IntegrationInstanceID string `json:"integration_instance_id,omitempty"`
	IsScheduler           bool   `json:"is_scheduler,omitempty"`
}

const defaultAuditLogLimit int64 = 100

const (
	integrationReconcileCommandBatchSize  = 200
	integrationVerificationTimeout        = 30 * time.Second
	integrationVerificationPersistTimeout = 5 * time.Second
)

type verificationRun struct {
	cancel context.CancelFunc
}

// IntegrationService manages integration instances and processes inbound webhooks.
type IntegrationService interface {
	CreateInstance(
		ctx context.Context,
		input integration.CreateInstanceInput,
	) (integration.Instance, error)
	GetInstance(
		ctx context.Context,
		input integration.GetInstanceInput,
	) (*integration.Instance, error)
	UpdateInstance(
		ctx context.Context,
		input integration.UpdateInstanceInput,
	) (integration.Instance, error)
	DeleteInstance(
		ctx context.Context,
		input integration.DeleteInstanceInput,
	) error
	ListInstances(
		ctx context.Context,
		input integration.ListInstancesInput,
	) ([]integration.Instance, error)
	ListAuditLogs(
		ctx context.Context,
		input integration.ListAuditLogsInput,
	) ([]integration.AuditLog, error)

	// IngestWebhook validates the webhook, persists the event as PENDING,
	// and returns immediately. Processing happens asynchronously.
	IngestWebhook(
		ctx context.Context,
		input integration.IngestWebhookInput,
	) (integration.Event, error)

	// ProcessQueueJob processes a claimed pgqueue job.
	ProcessQueueJob(ctx context.Context, job pgqueue.Job) error

	// ProcessReconcileQueueJob processes a reconciliation queue job. When the job
	// payload has IsScheduler=true it performs a full scheduler across all Clerk instances;
	// otherwise it reconciles a single instance identified by IntegrationInstanceID.
	ProcessReconcileQueueJob(ctx context.Context, job pgqueue.Job) error
}

var (
	ErrIntegrationInstanceNotFound = apierror.NewWithStatus(
		"INTEGRATION_INSTANCE_NOT_FOUND",
		"Integration instance not found",
		http.StatusNotFound,
	)
	ErrIntegrationInstanceAlreadyExists = apierror.NewBadRequest(
		"INTEGRATION_INSTANCE_ALREADY_EXISTS",
		"An integration instance for this provider already exists on this product",
	)
	ErrIntegrationProviderNotRegistered = apierror.NewBadRequest(
		"INTEGRATION_PROVIDER_NOT_REGISTERED",
		"The specified integration provider is not registered",
	)
	ErrIntegrationProviderNotWebhookIngestor = apierror.NewBadRequest(
		"INTEGRATION_PROVIDER_NOT_WEBHOOK_INGESTOR",
		"The specified integration provider does not accept inbound webhooks",
	)
	ErrIntegrationWebhookValidationFailed = apierror.NewWithStatus(
		"INTEGRATION_WEBHOOK_VALIDATION_FAILED",
		"Webhook signature validation failed",
		http.StatusUnauthorized,
	)
	ErrIntegrationInstanceDisabled = apierror.NewBadRequest(
		"INTEGRATION_INSTANCE_DISABLED",
		"The integration instance is disabled",
	)
	ErrIntegrationInstanceConfiguring = apierror.NewBadRequest(
		"INTEGRATION_INSTANCE_CONFIGURING",
		"The integration instance is still configuring",
	)
	ErrIntegrationInstanceUnhealthy = apierror.NewBadRequest(
		"INTEGRATION_INSTANCE_UNHEALTHY",
		"The integration instance is not ready to process webhook events",
	)
	ErrIntegrationWebhookSecretRequired = apierror.NewBadRequest(
		"INTEGRATION_WEBHOOK_SECRET_REQUIRED",
		"Webhook secret is required when integration instance is active",
	)
	ErrIntegrationWebhookEventIDMissing = apierror.NewWithStatus(
		"INTEGRATION_WEBHOOK_EVENT_ID_MISSING",
		"Webhook event id header is required",
		http.StatusUnauthorized,
	)
)

type integrationService struct {
	instanceRepo              repository.IntegrationInstanceRepository
	eventRepo                 repository.IntegrationEventRepository
	auditLogRepo              repository.IntegrationAuditLogRepository
	registry                  *provider.Registry
	queue                     *pgqueue.Client
	lock                      *pglock.Client
	db                        *sql.DB
	logger                    zerolog.Logger
	reconcileScheduleInterval time.Duration
	verificationMu            sync.Mutex
	verificationRuns          map[string]*verificationRun
}

func NewIntegrationService(
	instanceRepo repository.IntegrationInstanceRepository,
	eventRepo repository.IntegrationEventRepository,
	auditLogRepo repository.IntegrationAuditLogRepository,
	registry *provider.Registry,
	queue *pgqueue.Client,
	lock *pglock.Client,
	db *sql.DB,
	logger zerolog.Logger,
	coreCfg *serviceconfig.CoreConfig,
) IntegrationService {
	return &integrationService{
		instanceRepo:              instanceRepo,
		eventRepo:                 eventRepo,
		auditLogRepo:              auditLogRepo,
		registry:                  registry,
		queue:                     queue,
		lock:                      lock,
		db:                        db,
		reconcileScheduleInterval: parseScheduleInterval(logger, coreCfg),
		verificationRuns:          map[string]*verificationRun{},
		logger: logger.With().
			Str("component", "integration_service").Logger(),
	}
}

// ---------------------------------------------------------------------------
// Instance CRUD
// ---------------------------------------------------------------------------

func (s *integrationService) CreateInstance(
	ctx context.Context,
	input integration.CreateInstanceInput,
) (integration.Instance, error) {
	logger := s.logger.With().Str("operation", "CreateInstance").Logger()

	if valErr := validateStruct(input); valErr != nil {
		return integration.Instance{}, valErr
	}

	// Verify provider is registered.
	prov, provErr := s.registry.GetProvider(string(input.ProviderType))
	if provErr != nil {
		logger.Warn().
			Str(fieldProviderTypeKey, string(input.ProviderType)).
			Msg("provider not registered")
		return integration.Instance{}, ErrIntegrationProviderNotRegistered
	}

	// Validate config if supplied.
	configJSON := input.ConfigJSON
	if configJSON == nil {
		configJSON = json.RawMessage("{}")
	}
	normalizedConfig, prepErr := s.prepareConfigForStorage(ctx, prov, configJSON)
	if prepErr != nil {
		return integration.Instance{}, newInvalidConfigError(prepErr)
	}
	configJSON = normalizedConfig
	if cfgErr := prov.ValidateConfig(ctx, configJSON); cfgErr != nil {
		return integration.Instance{}, newInvalidConfigError(cfgErr)
	}

	created, err := jetx.WithTxReturn(s.db, func(tx *sql.Tx) (integration.Instance, error) {
		return s.createInstanceTx(ctx, logger, tx, input, prov, configJSON)
	})
	if err != nil {
		return integration.Instance{}, err
	}

	// If a Clerk API key was provided, check whether we need to start the scheduler.
	// This runs after the transaction commits so that the new instance is visible
	// to the count query inside maybeStartReconcileScheduler.
	if hasClerkAPIKey(created.ConfigJSON) && created.ProviderType == integration.ProviderTypeClerk {
		s.maybeStartReconcileScheduler(ctx, logger)
	}

	// For providers that support active connection verification (e.g. SMTP), probe the
	// endpoint asynchronously so the HTTP response isn't blocked on network I/O.
	s.startVerifyAndActivate(logger, created, prov)

	return created, nil
}

func (s *integrationService) createInstanceTx(
	ctx context.Context,
	logger zerolog.Logger,
	tx *sql.Tx,
	input integration.CreateInstanceInput,
	prov provider.Provider,
	configJSON json.RawMessage,
) (integration.Instance, error) {
	txOpts := &jetx.DBOptions{Tx: tx}

	// Check for duplicate (product + provider uniqueness).
	existing, findErr := s.instanceRepo.FindByProductAndProvider(
		ctx,
		input.TenantID,
		input.ProductID,
		string(input.ProviderType),
		txOpts,
	)
	if findErr != nil {
		logger.Error().Err(findErr).
			Msg("failed to check for existing instance")
		return integration.Instance{}, findErr
	}
	if existing != nil {
		return integration.Instance{},
			ErrIntegrationInstanceAlreadyExists
	}

	now := time.Now()
	defaultReason := integration.DefaultConfiguringReason
	instance := integration.Instance{
		PlatformTenantID: input.TenantID,
		ProductID:        input.ProductID,
		ProviderType:     input.ProviderType,
		ConfigJSON:       configJSON,
		ConfigVersion:    prov.LatestConfigVersion(),
		IsEnabled:        true,
		LastError:        &defaultReason,
		Status:           integration.StatusConfiguring,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	instance.GenerateID()

	created, createErr := s.instanceRepo.Create(ctx, instance, txOpts)
	if createErr != nil {
		logger.Error().Err(createErr).
			Msg("failed to create integration instance")
		return integration.Instance{}, createErr
	}

	hasKey := hasClerkAPIKey(created.ConfigJSON)

	if enqueueErr := s.enqueueReconcileJobIfRequired(
		ctx, logger, created, hasKey, tx,
	); enqueueErr != nil {
		return integration.Instance{}, enqueueErr
	}

	logger.Info().
		Str("instance_id", created.ID).
		Str(fieldProductIDKey, input.ProductID).
		Str(fieldProviderTypeKey, string(input.ProviderType)).
		Msg("integration instance created")

	s.writeAuditLog(ctx, logger, integration.AuditLog{
		IntegrationInstanceID: created.ID,
		Action:                integration.AuditActionIntegrationCreated,
		Severity:              integration.AuditSeveritySuccess,
		Message:               "Integration instance created",
		MetadataJSON: provider.MustMarshalJSON(map[string]any{
			fieldProductIDKey:    created.ProductID,
			fieldProviderTypeKey: string(created.ProviderType),
			fieldStatusKey:       string(created.Status),
		}),
	}, txOpts)

	return created, nil
}

func (s *integrationService) GetInstance(
	ctx context.Context,
	input integration.GetInstanceInput,
) (*integration.Instance, error) {
	logger := s.logger.With().Str("operation", "GetInstance").Logger()

	if valErr := validateStruct(input); valErr != nil {
		return nil, valErr
	}

	instance, findErr := s.instanceRepo.FindByID(
		ctx, input.TenantID, input.ID, nil,
	)
	if findErr != nil {
		logger.Error().Err(findErr).
			Str("instance_id", input.ID).
			Msg("failed to find instance")
		return nil, findErr
	}
	if instance == nil {
		return nil, ErrIntegrationInstanceNotFound
	}

	return instance, nil
}

func (s *integrationService) UpdateInstance(
	ctx context.Context,
	input integration.UpdateInstanceInput,
) (integration.Instance, error) {
	logger := s.logger.With().Str("operation", "UpdateInstance").Logger()

	if valErr := validateStruct(input); valErr != nil {
		return integration.Instance{}, valErr
	}

	var hadClerkAPIKey bool
	var hasClerkAPIKeyNow bool

	updated, err := jetx.WithTxReturn(
		s.db,
		func(tx *sql.Tx) (integration.Instance, error) {
			txOpts := &jetx.DBOptions{Tx: tx}

			existing, findErr := s.instanceRepo.FindByID(
				ctx, input.TenantID, input.ID, txOpts,
			)
			if findErr != nil {
				logger.Error().Err(findErr).
					Str("instance_id", input.ID).
					Msg("failed to find instance for update")
				return integration.Instance{}, findErr
			}
			if existing == nil {
				return integration.Instance{},
					ErrIntegrationInstanceNotFound
			}

			prov, provErr := s.registry.GetProvider(string(existing.ProviderType))
			if provErr != nil {
				return integration.Instance{}, ErrIntegrationProviderNotRegistered
			}

			hadClerkAPIKey = hasClerkAPIKey(existing.ConfigJSON)

			if applyErr := s.applyInstanceUpdates(
				ctx, existing, input, prov,
			); applyErr != nil {
				return integration.Instance{}, applyErr
			}

			result, updateErr := s.instanceRepo.Update(
				ctx, input.TenantID, *existing, txOpts,
			)
			if updateErr != nil {
				logger.Error().Err(updateErr).
					Str("instance_id", input.ID).
					Msg("failed to update instance")
				return integration.Instance{}, updateErr
			}

			hasClerkAPIKeyNow = hasClerkAPIKey(result.ConfigJSON)
			// Trigger reconcile when a key is added, removed, OR changed (rotation).
			// When configJSON is provided and a key is present we always reconcile;
			// when the key is absent we only reconcile if it was previously present
			// (to clean up / mark as unconfigured).
			shouldTriggerReconcile := input.ConfigJSON != nil && (hasClerkAPIKeyNow || hadClerkAPIKey)

			if enqueueErr := s.enqueueReconcileJobIfRequired(
				ctx,
				logger,
				result,
				shouldTriggerReconcile,
				tx,
			); enqueueErr != nil {
				return integration.Instance{}, enqueueErr
			}

			logger.Info().
				Str("instance_id", result.ID).
				Msg("integration instance updated")

			s.writeAuditLog(
				ctx,
				logger,
				integration.AuditLog{
					IntegrationInstanceID: result.ID,
					Action:                integration.AuditActionConfigUpdated,
					Severity:              integration.AuditSeverityInfo,
					Message:               "Integration configuration updated",
					MetadataJSON: provider.MustMarshalJSON(
						map[string]any{
							fieldStatusKey:   string(result.Status),
							"config_version": result.ConfigVersion,
						},
					),
				},
				txOpts,
			)

			return result, nil
		},
	)
	if err != nil {
		return integration.Instance{}, err
	}

	// Scheduler lifecycle management — runs after the transaction commits so the
	// updated state is visible to the count query.
	s.manageReconcileSchedulerOnUpdate(ctx, logger, updated, hadClerkAPIKey, hasClerkAPIKeyNow, input.ConfigJSON)

	// Re-verify connection when config changes (e.g. SMTP credentials rotated).
	if input.ConfigJSON != nil {
		prov, provErr := s.registry.GetProvider(string(updated.ProviderType))
		if provErr == nil {
			s.startVerifyAndActivate(logger, updated, prov)
		}
	}

	return updated, nil
}

// manageReconcileSchedulerOnUpdate starts or cancels the reconcile scheduler when an instance's
// Clerk API key changes. Must be called after the update transaction commits.
func (s *integrationService) manageReconcileSchedulerOnUpdate(
	ctx context.Context,
	logger zerolog.Logger,
	updated integration.Instance,
	hadKey, hasKeyNow bool,
	configJSON json.RawMessage,
) {
	if updated.ProviderType != integration.ProviderTypeClerk || configJSON == nil {
		return
	}
	if !hadKey && hasKeyNow {
		s.maybeStartReconcileScheduler(ctx, logger)
	} else if hadKey && !hasKeyNow {
		s.maybeCancelReconcileScheduler(ctx, logger)
	}
}

func (s *integrationService) applyInstanceUpdates(
	ctx context.Context,
	existing *integration.Instance,
	input integration.UpdateInstanceInput,
	prov provider.Provider,
) error {
	s.applyWebhookSecretUpdate(existing, input.WebhookSecret)
	s.applyEnabledUpdate(existing, input.IsEnabled)

	if err := s.applyConfigUpdate(ctx, existing, input.ConfigJSON, prov); err != nil {
		return err
	}

	if err := validateInstanceIngestionState(existing); err != nil {
		return err
	}

	return nil
}

func validateInstanceIngestionState(existing *integration.Instance) error {
	if existing.Status != integration.StatusActive {
		return nil
	}

	if existing.WebhookSecret == nil || strings.TrimSpace(*existing.WebhookSecret) == "" {
		return ErrIntegrationWebhookSecretRequired
	}

	return nil
}

func (s *integrationService) applyWebhookSecretUpdate(
	existing *integration.Instance,
	webhookSecret *string,
) {
	if webhookSecret == nil {
		return
	}

	existing.WebhookSecret = webhookSecret
	if existing.Status == integration.StatusConfiguring {
		existing.Status = integration.StatusActive
		existing.LastError = nil
	}
}

func (s *integrationService) applyEnabledUpdate(
	existing *integration.Instance,
	isEnabled *bool,
) {
	if isEnabled == nil {
		return
	}

	existing.IsEnabled = *isEnabled
	if !existing.IsEnabled {
		existing.Status = integration.StatusInactive
		return
	}

	if existing.Status == integration.StatusInactive {
		existing.Status = integration.StatusActive
	}
}

func (s *integrationService) applyConfigUpdate(
	ctx context.Context,
	existing *integration.Instance,
	configJSON json.RawMessage,
	prov provider.Provider,
) error {
	if configJSON == nil {
		return nil
	}

	normalizedConfig, prepErr := s.prepareConfigForUpdatedStorage(ctx, prov, existing.ConfigJSON, configJSON)
	if prepErr != nil {
		return newInvalidConfigError(prepErr)
	}
	if cfgErr := prov.ValidateConfig(ctx, normalizedConfig); cfgErr != nil {
		return newInvalidConfigError(cfgErr)
	}

	existing.ConfigJSON = normalizedConfig
	existing.ConfigVersion = prov.LatestConfigVersion()

	return nil
}

func (s *integrationService) DeleteInstance(
	ctx context.Context,
	input integration.DeleteInstanceInput,
) error {
	logger := s.logger.With().Str("operation", "DeleteInstance").Logger()

	if valErr := validateStruct(input); valErr != nil {
		return valErr
	}

	// Verify it exists first.
	existing, findErr := s.instanceRepo.FindByID(
		ctx, input.TenantID, input.ID, nil,
	)
	if findErr != nil {
		logger.Error().Err(findErr).
			Str("instance_id", input.ID).
			Msg("failed to find instance for deletion")
		return findErr
	}
	if existing == nil {
		return ErrIntegrationInstanceNotFound
	}

	hadKey := hasClerkAPIKey(existing.ConfigJSON) && existing.ProviderType == integration.ProviderTypeClerk

	delErr := jetx.WithTx(
		s.db, func(tx *sql.Tx) error {
			txOpts := &jetx.DBOptions{Tx: tx}

			s.writeAuditLog(
				ctx,
				logger,
				integration.AuditLog{
					IntegrationInstanceID: input.ID,
					Action:                integration.AuditActionIntegrationDeleted,
					Severity:              integration.AuditSeverityWarning,
					Message:               "Integration instance deleted",
					MetadataJSON: provider.MustMarshalJSON(
						map[string]any{
							fieldProductIDKey: existing.ProductID,
						},
					),
				},
				txOpts,
			)

			return s.instanceRepo.DeleteByID(
				ctx, input.TenantID, input.ID, txOpts,
			)
		},
	)
	if delErr != nil {
		logger.Error().Err(delErr).
			Str("instance_id", input.ID).
			Msg("failed to delete instance")
		return delErr
	}

	logger.Info().
		Str("instance_id", input.ID).
		Str(fieldProductIDKey, input.ProductID).
		Msg("integration instance deleted")

	// If the deleted instance had a Clerk API key, check whether the scheduler
	// needs to be cancelled (no remaining instances with keys).
	if hadKey {
		s.maybeCancelReconcileScheduler(ctx, logger)
	}

	return nil
}

func (s *integrationService) ListInstances(
	ctx context.Context,
	input integration.ListInstancesInput,
) ([]integration.Instance, error) {
	logger := s.logger.With().Str("operation", "ListInstances").Logger()

	if valErr := validateStruct(input); valErr != nil {
		return nil, valErr
	}

	instances, listErr := s.instanceRepo.ListByProduct(
		ctx, input.TenantID, input.ProductID, nil,
	)
	if listErr != nil {
		logger.Error().Err(listErr).
			Str(fieldProductIDKey, input.ProductID).
			Msg("failed to list instances")
		return nil, listErr
	}

	return instances, nil
}

func (s *integrationService) ListAuditLogs(
	ctx context.Context,
	input integration.ListAuditLogsInput,
) ([]integration.AuditLog, error) {
	logger := s.logger.With().Str("operation", "ListAuditLogs").Logger()

	if valErr := validateStruct(input); valErr != nil {
		return nil, valErr
	}

	limit := input.Limit
	if limit <= 0 {
		limit = defaultAuditLogLimit
	}

	instance, findErr := s.instanceRepo.FindByID(
		ctx, input.TenantID, input.IntegrationInstanceID, nil,
	)
	if findErr != nil {
		logger.Error().Err(findErr).
			Str("integration_instance_id", input.IntegrationInstanceID).
			Msg("failed to find integration instance for audit logs")
		return nil, findErr
	}

	if instance == nil || instance.ProductID != input.ProductID {
		return nil, ErrIntegrationInstanceNotFound
	}

	logs, listErr := s.auditLogRepo.ListByInstanceScoped(
		ctx,
		input.TenantID,
		input.ProductID,
		input.IntegrationInstanceID,
		limit,
		input.Offset,
		nil,
	)
	if listErr != nil {
		logger.Error().Err(listErr).
			Str("integration_instance_id", input.IntegrationInstanceID).
			Msg("failed to list integration audit logs")
		return nil, listErr
	}

	return logs, nil
}

// ---------------------------------------------------------------------------
// Webhook Ingestion (fast path — validate + persist PENDING)
// ---------------------------------------------------------------------------

func (s *integrationService) IngestWebhook(
	ctx context.Context,
	input integration.IngestWebhookInput,
) (integration.Event, error) {
	logger := s.logger.With().
		Str("operation", "IngestWebhook").
		Str(fieldProviderTypeKey, string(input.ProviderType)).
		Str(fieldProductIDKey, input.ProductID).
		Logger()

	// 1-4: Resolve provider, instance, validate webhook signature.
	prov, instance, resolveErr := s.resolveWebhookTarget(ctx, logger, input)
	if resolveErr != nil {
		return integration.Event{}, resolveErr
	}

	// 5. Parse headers and build the event shell (no command execution).
	event, _, parseErr := s.buildWebhookEvent(
		ctx, logger, prov, instance, input,
	)
	if parseErr != nil {
		return integration.Event{}, parseErr
	}

	// 6. Persist as PENDING inside a transaction (with idempotency check).
	return s.persistPendingEvent(ctx, logger, instance, event)
}

// persistPendingEvent stores the event as PENDING, handling idempotency.
func (s *integrationService) persistPendingEvent(
	ctx context.Context,
	logger zerolog.Logger,
	instance *integration.Instance,
	event integration.Event,
) (integration.Event, error) {
	return jetx.WithTxReturn(
		s.db,
		func(tx *sql.Tx) (integration.Event, error) {
			txOpts := &jetx.DBOptions{Tx: tx}

			// Idempotency: check for duplicate by (instance_id, external_event_id).
			if existing, err := s.findDuplicateEvent(ctx, logger, instance, event, txOpts); err != nil {
				return integration.Event{}, err
			} else if existing != nil {
				return *existing, nil
			}

			created, createErr := s.eventRepo.CreateInternal(ctx, event, txOpts)
			if createErr != nil {
				logger.Error().Err(createErr).Msg("failed to store integration event")
				return integration.Event{}, createErr
			}

			payload, payloadErr := json.Marshal(integrationQueuePayload{EventID: created.ID})
			if payloadErr != nil {
				logger.Error().Err(payloadErr).Msg("failed to marshal queue payload")
				return integration.Event{}, payloadErr
			}

			_, enqueueErr := s.queue.EnqueueTx(ctx, tx, pgqueue.EnqueueParams{
				QueueName:   integrationQueueName,
				Payload:     payload,
				MaxAttempts: integrationMaxAttempts,
			})
			if enqueueErr != nil {
				logger.Error().Err(enqueueErr).Msg("failed to enqueue integration event")
				return integration.Event{}, enqueueErr
			}

			s.writeAuditLog(
				ctx,
				logger,
				integration.AuditLog{
					IntegrationInstanceID: instance.ID,
					Action:                integration.AuditActionWebhookReceived,
					Severity:              integration.AuditSeverityInfo,
					Message:               "Webhook accepted and queued for processing",
					MetadataJSON: provider.MustMarshalJSON(
						map[string]any{
							fieldEventIDKey:     created.ID,
							"external_event_id": created.ExternalEventID,
							fieldEventTypeKey:   created.EventType,
						},
					),
				},
				txOpts,
			)

			logger.Info().
				Str(fieldEventIDKey, created.ID).
				Str(fieldEventTypeKey, created.EventType).
				Msg("webhook ingested and queued as PENDING")

			return created, nil
		},
	)
}

// ---------------------------------------------------------------------------
// Async Event Processing (called by worker)
// ---------------------------------------------------------------------------

// ProcessQueueJob processes a claimed pgqueue job.
func (s *integrationService) ProcessQueueJob(ctx context.Context, job pgqueue.Job) error {
	logger := s.logger.With().Str("operation", "ProcessQueueJob").Logger()
	return s.processOneEvent(ctx, logger, &job)
}

// ProcessReconcileQueueJob processes a claimed reconciliation pgqueue job.
//
// When the payload has IsScheduler=true the job is a periodic scheduler: it lists all
// Clerk instances with an API key, enqueues a per-instance reconcile job for
// each one, then re-enqueues itself with AvailableAt = now + schedulerInterval so
// that reconciliation keeps running without an in-process ticker.
//
// When IsScheduler=false (or unset) the job reconciles a single instance identified
// by IntegrationInstanceID.
func (s *integrationService) ProcessReconcileQueueJob(ctx context.Context, job pgqueue.Job) error {
	logger := s.logger.With().Str("operation", "ProcessReconcileQueueJob").Logger()

	var payload integrationReconcileQueuePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return pgqueue.NonRetryable(fmt.Errorf("invalid reconcile queue payload: %w", err))
	}

	if payload.IsScheduler {
		return s.runReconcileScheduler(ctx, logger)
	}

	if payload.IntegrationInstanceID == "" {
		return pgqueue.NonRetryable(errors.New("reconcile queue payload missing integration_instance_id"))
	}

	return s.runInstanceReconcile(ctx, logger, payload.IntegrationInstanceID)
}

// runReconcileScheduler lists all Clerk instances that have an API key, enqueues a
// per-instance reconcile job for each, then re-schedules the next scheduler job via
// pgqueue so the cycle continues without an in-process goroutine.
func (s *integrationService) runReconcileScheduler(ctx context.Context, logger zerolog.Logger) error {
	instances, err := s.instanceRepo.ListByProviderInternal(
		ctx,
		string(integration.ProviderTypeClerk),
		nil,
	)
	if err != nil {
		return err
	}

	enqueued := 0
	var firstErr error

	for _, instance := range instances {
		if !hasClerkAPIKey(instance.ConfigJSON) {
			continue
		}

		payload, marshalErr := json.Marshal(integrationReconcileQueuePayload{
			IntegrationInstanceID: instance.ID,
		})
		if marshalErr != nil {
			return marshalErr
		}

		if _, enqueueErr := s.queue.Enqueue(ctx, pgqueue.EnqueueParams{
			QueueName:   integrationReconcileQueueName,
			Payload:     payload,
			MaxAttempts: integrationMaxAttempts,
		}); enqueueErr != nil {
			logger.Error().Err(enqueueErr).
				Str("integration_instance_id", instance.ID).
				Msg("failed to enqueue instance reconcile job during scheduler; continuing")
			if firstErr == nil {
				firstErr = enqueueErr
			}
			continue
		}

		enqueued++
	}

	logger.Info().
		Int("instances_total", len(instances)).
		Int("jobs_enqueued", enqueued).
		Msg("reconcile scheduler completed")

	// Re-schedule the next scheduler job. This makes the scheduler self-sustaining via
	// pgqueue without any in-process goroutine or external cron.
	nextAt := time.Now().Add(s.reconcileScheduleInterval)
	if reEnqueueErr := enqueueReconcileSchedulerJob(ctx, s.queue, &nextAt); reEnqueueErr != nil {
		logger.Error().Err(reEnqueueErr).
			Time("next_at", nextAt).
			Msg("failed to re-schedule reconcile scheduler job")
		// Return the re-schedule error so pgqueue can retry the scheduler job itself.
		return reEnqueueErr
	}

	if firstErr != nil {
		logger.Warn().Err(firstErr).Msg("scheduler completed with some enqueue errors (swallowed to avoid fork bomb)")
	}

	return nil
}

// runInstanceReconcile reconciles a single integration instance.
func (s *integrationService) runInstanceReconcile(
	ctx context.Context,
	logger zerolog.Logger,
	instanceID string,
) error {
	instance, findErr := s.instanceRepo.FindByIDInternal(ctx, instanceID, nil)
	if findErr != nil {
		return findErr
	}
	if instance == nil {
		return pgqueue.NonRetryable(
			fmt.Errorf("integration instance not found: %s", instanceID),
		)
	}

	prov, provErr := s.registry.GetProvider(string(instance.ProviderType))
	if provErr != nil {
		return pgqueue.NonRetryable(
			fmt.Errorf("provider %s not registered", instance.ProviderType),
		)
	}

	clerkProv, ok := prov.(*clerkprovider.Provider)
	if !ok {
		// Only Clerk supports reconciliation; silently skip other providers.
		return nil
	}

	// Reconcile produces canonical commands that are dispatched through the
	// WebhookIngestor capability (Clerk implements both).
	webhookProv, isIngestor := prov.(provider.WebhookIngestor)
	if !isIngestor {
		return pgqueue.NonRetryable(
			fmt.Errorf("provider %s reconciles but is not a webhook ingestor", instance.ProviderType),
		)
	}

	commands, recErr := clerkProv.Reconcile(ctx, instance.ConfigJSON)
	if recErr != nil {
		return recErr
	}
	if len(commands) == 0 {
		logger.Info().
			Str("integration_instance_id", instance.ID).
			Msg("integration reconcile produced no commands")
		return nil
	}

	batchesProcessed, execErr := s.executeCommandsInBatches(
		ctx,
		logger,
		webhookProv,
		instance,
		commands,
		integrationReconcileCommandBatchSize,
	)
	if execErr != nil {
		return execErr
	}

	return jetx.WithTx(s.db, func(tx *sql.Tx) error {
		txOpts := &jetx.DBOptions{Tx: tx}
		s.writeAuditLog(ctx, logger, integration.AuditLog{
			IntegrationInstanceID: instance.ID,
			Action:                integration.AuditActionReconcileCompleted,
			Severity:              integration.AuditSeveritySuccess,
			Message:               "Integration reconciliation completed",
			MetadataJSON: provider.MustMarshalJSON(map[string]any{
				"commands": len(commands),
				"batches":  batchesProcessed,
			}),
		}, txOpts)

		return nil
	})
}

// ---------------------------------------------------------------------------
// Scheduler lifecycle helpers
// ---------------------------------------------------------------------------

// countClerkInstancesWithKey returns the number of Clerk instances that have an
// API key configured. The count is performed without tenant scoping because it
// is used by system-internal paths (startup seeding, scheduler start/stop logic).
func (s *integrationService) countClerkInstancesWithKey(ctx context.Context) (int, error) {
	instances, err := s.instanceRepo.ListByProviderInternal(
		ctx, string(integration.ProviderTypeClerk), nil,
	)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, inst := range instances {
		if hasClerkAPIKey(inst.ConfigJSON) {
			count++
		}
	}

	return count, nil
}

const (
	maxSchedulerJobsToInspect = 1000
)

// hasSchedulerJob checks if there are any scheduler jobs currently pending or processing.
func (s *integrationService) hasSchedulerJob(ctx context.Context) (bool, error) {
	for _, status := range []pgqueue.JobStatus{pgqueue.StatusPending, pgqueue.StatusProcessing} {
		jobs, err := s.queue.ListJobs(ctx, pgqueue.ListJobsParams{
			QueueName: integrationReconcileQueueName,
			Status:    status,
			Limit:     maxSchedulerJobsToInspect,
		})
		if err != nil {
			return false, err
		}
		for _, job := range jobs {
			var payload integrationReconcileQueuePayload
			if jsonErr := json.Unmarshal(job.Payload, &payload); jsonErr == nil && payload.IsScheduler {
				return true, nil
			}
		}
	}
	return false, nil
}

// maybeStartReconcileScheduler seeds a reconcile scheduler job if this is the very first Clerk
// instance with an API key. It uses a distributed advisory lock so that concurrent
// requests and multi-replica deployments do not duplicate the scheduler seed.
func (s *integrationService) maybeStartReconcileScheduler(ctx context.Context, logger zerolog.Logger) {
	acquired, err := s.lock.TryWithLock(ctx, lockKeyReconcileSchedulerSeed, func(ctx context.Context, _ *sql.Tx) error {
		n, countErr := s.countClerkInstancesWithKey(ctx)
		if countErr != nil {
			return countErr
		}
		if n == 0 {
			return nil
		}

		// Check if a scheduler is already queued or processing.
		hasScheduler, err := s.hasSchedulerJob(ctx)
		if err != nil {
			return err
		}
		if hasScheduler {
			return nil
		}

		return enqueueReconcileSchedulerJob(ctx, s.queue, nil)
	})
	if err != nil {
		logger.Error().Err(err).Msg("failed to start reconcile scheduler after first key added")
		return
	}
	if !acquired {
		logger.Debug().Msg("reconcile scheduler start lock not acquired; another replica is handling it")
		return
	}

	logger.Info().Msg("reconcile scheduler started after first Clerk API key was added")
}

// maybeCancelReconcileScheduler deletes all pending scheduler jobs from the queue when no Clerk
// instance has an API key any more. It is called after a key is removed or an
// instance with a key is deleted.
func (s *integrationService) maybeCancelReconcileScheduler(ctx context.Context, logger zerolog.Logger) {
	n, err := s.countClerkInstancesWithKey(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("failed to count Clerk instances with key when checking scheduler cancellation")
		return
	}
	if n > 0 {
		// Other instances still have keys; scheduler must keep running.
		return
	}

	// No instances with keys remain — delete all pending scheduler jobs.
	const maxSchedulerJobsToDelete = 1000
	jobs, listErr := s.queue.ListJobs(ctx, pgqueue.ListJobsParams{
		QueueName: integrationReconcileQueueName,
		Status:    pgqueue.StatusPending,
		Limit:     maxSchedulerJobsToDelete,
	})
	if listErr != nil {
		logger.Error().Err(listErr).Msg("failed to list pending scheduler jobs for cancellation")
		return
	}

	deleted := 0
	for _, job := range jobs {
		var payload integrationReconcileQueuePayload
		if jsonErr := json.Unmarshal(job.Payload, &payload); jsonErr != nil || !payload.IsScheduler {
			continue
		}

		if delErr := s.queue.DeleteJob(ctx, job.ID); delErr != nil {
			logger.Error().Err(delErr).
				Int64("job_id", job.ID).
				Msg("failed to delete pending scheduler job")
			continue
		}

		deleted++
	}

	logger.Info().
		Int("scheduler_jobs_deleted", deleted).
		Msg("reconcile scheduler cancelled after last Clerk API key was removed")
}

// enqueueReconcileSchedulerJob enqueues a scheduler job into the reconcile queue.
// availableAt=nil means immediate; otherwise the job is deferred to that time.
func enqueueReconcileSchedulerJob(ctx context.Context, queue *pgqueue.Client, availableAt *time.Time) error {
	payload, err := json.Marshal(integrationReconcileQueuePayload{IsScheduler: true})
	if err != nil {
		return err
	}

	_, err = queue.Enqueue(ctx, pgqueue.EnqueueParams{
		QueueName:   integrationReconcileQueueName,
		Payload:     payload,
		AvailableAt: availableAt,
		MaxAttempts: integrationMaxAttempts,
	})

	return err
}

// processOneEvent handles a single event: resolve instance/provider, execute
// commands, mark PROCESSED or schedule retry / mark FAILED.
//
// The method uses a two-phase transaction approach:
//   - Phase 1 (command tx): Mark PROCESSING, resolve context, execute commands,
//     mark PROCESSED. On success this tx commits and we're done.
//   - Phase 2 (failure tx): If phase 1 fails, a *separate* tx persists
//     the failure/retry status and audit logs, so they survive the rollback
//     of the command tx.
func (s *integrationService) processOneEvent(
	ctx context.Context,
	logger zerolog.Logger,
	job *pgqueue.Job,
) error {
	var payload integrationQueuePayload
	if unmarshalErr := json.Unmarshal(job.Payload, &payload); unmarshalErr != nil {
		return pgqueue.NonRetryable(fmt.Errorf("invalid queue payload: %w", unmarshalErr))
	}
	if payload.EventID == "" {
		return pgqueue.NonRetryable(errors.New("queue payload missing event_id"))
	}

	event, findErr := s.eventRepo.FindByIDInternal(ctx, payload.EventID, nil)
	if findErr != nil {
		return findErr
	}
	if event == nil {
		return pgqueue.NonRetryable(fmt.Errorf("integration event not found: %s", payload.EventID))
	}

	eventLogger := logger.With().
		Str(fieldEventIDKey, event.ID).
		Str(fieldEventTypeKey, event.EventType).
		Int(fieldQueueAttemptKey, job.Attempts).
		Int(fieldQueueMaxAttemptsKey, job.MaxAttempts).
		Int64("queue_job_id", job.ID).
		Logger()

	// Phase 1: attempt command execution in a single transaction.
	// Capture the resolved instance for use in phase 2 failure recording.
	var resolvedInstance *integration.Instance

	procErr := jetx.WithTx(s.db, func(tx *sql.Tx) error {
		txOpts := &jetx.DBOptions{Tx: tx}
		instance, txErr := s.executeEventTransaction(ctx, eventLogger, event, job, txOpts)
		if instance != nil {
			resolvedInstance = instance
		}
		return txErr
	})

	if procErr == nil {
		return nil
	}

	// Phase 2: command tx rolled back. Persist failure/retry state in a
	// separate transaction so it is durably committed regardless of the
	// error returned to pgqueue.
	return s.handleEventFailure(ctx, eventLogger, event, job, resolvedInstance, procErr)
}

// executeEventTransaction runs Phase 1 of event processing inside a transaction:
// mark PROCESSING → resolve instance & provider → parse → execute commands → mark PROCESSED.
// Returns the resolved instance (even on error) so the caller can use it for failure recording.
func (s *integrationService) executeEventTransaction(
	ctx context.Context,
	logger zerolog.Logger,
	event *integration.Event,
	job *pgqueue.Job,
	txOpts *jetx.DBOptions,
) (*integration.Instance, error) {
	// Mark as PROCESSING.
	if statusErr := s.eventRepo.UpdateStatusInternal(
		ctx, event.ID, integration.EventStatusProcessing, nil, txOpts,
	); statusErr != nil {
		return nil, statusErr
	}

	// Resolve the instance (unscoped — worker has no tenant context).
	instance, instanceErr := s.instanceRepo.FindByIDInternal(ctx, event.IntegrationInstanceID, txOpts)
	if instanceErr != nil {
		return nil, fmt.Errorf("failed to find integration instance: %w", instanceErr)
	}
	if instance == nil {
		return nil, fmt.Errorf("integration instance not found for event: %s", event.IntegrationInstanceID)
	}

	// Resolve provider.
	prov, provErr := s.registry.GetProvider(string(instance.ProviderType))
	if provErr != nil {
		return instance, fmt.Errorf("provider %s not registered", instance.ProviderType)
	}
	webhookProv, ok := prov.(provider.WebhookIngestor)
	if !ok {
		return instance, fmt.Errorf(
			"provider %s does not support webhook ingestion", instance.ProviderType,
		)
	}

	// Check instance can ingest.
	if !instance.CanIngest() {
		return instance, fmt.Errorf("instance blocked: %s", instance.IngestionBlockReason())
	}

	// Parse the event payload.
	parsedEvent, parseErr := webhookProv.ParseEvent(ctx, "", event.PayloadJSON)
	if parseErr != nil {
		return instance, parseErr
	}

	// Build canonical commands.
	commands, cmdErr := webhookProv.ToStandardsCommands(ctx, event.EventType, parsedEvent)
	if cmdErr != nil {
		return instance, cmdErr
	}

	// Execute commands.
	if execErr := s.executeCommands(ctx, logger, webhookProv, instance, commands, txOpts); execErr != nil {
		return instance, execErr
	}

	// Mark PROCESSED.
	if markErr := s.eventRepo.UpdateStatusInternal(
		ctx, event.ID, integration.EventStatusProcessed, nil, txOpts,
	); markErr != nil {
		return instance, markErr
	}

	s.writeAuditLog(ctx, logger, integration.AuditLog{
		IntegrationInstanceID: instance.ID,
		Action:                integration.AuditActionWebhookProcessed,
		Severity:              integration.AuditSeveritySuccess,
		Message:               "Webhook event processed successfully",
		MetadataJSON: provider.MustMarshalJSON(map[string]any{
			fieldEventIDKey:          event.ID,
			fieldEventTypeKey:        event.EventType,
			"commands":               len(commands),
			fieldQueueAttemptKey:     job.Attempts,
			fieldQueueMaxAttemptsKey: job.MaxAttempts,
		}),
	}, txOpts)

	logger.Info().
		Int("commands", len(commands)).
		Msg("event processed successfully")

	return instance, nil
}

// handleEventFailure decides whether to retry or permanently fail the event.
// It runs in its own transaction so that failure metadata is committed even
// though the command-execution transaction was rolled back.
func (s *integrationService) handleEventFailure(
	ctx context.Context,
	logger zerolog.Logger,
	event *integration.Event,
	job *pgqueue.Job,
	instance *integration.Instance,
	cause error,
) error {
	failErr := jetx.WithTx(s.db, func(tx *sql.Tx) error {
		txOpts := &jetx.DBOptions{Tx: tx}

		if job.Attempts < job.MaxAttempts {
			return s.scheduleRetry(ctx, logger, event, job, instance, cause, txOpts)
		}

		// Exhausted retries — mark FAILED.
		s.markEventFailed(ctx, event.ID, cause, txOpts)

		if instance != nil {
			s.writeAuditLog(ctx, logger, integration.AuditLog{
				IntegrationInstanceID: instance.ID,
				Action:                integration.AuditActionWebhookFailed,
				Severity:              integration.AuditSeverityError,
				Message:               "Webhook event failed after exhausting retries",
				MetadataJSON: provider.MustMarshalJSON(map[string]any{
					fieldEventIDKey:          event.ID,
					fieldEventTypeKey:        event.EventType,
					fieldQueueAttemptKey:     job.Attempts,
					fieldQueueMaxAttemptsKey: job.MaxAttempts,
					"error":                  cause.Error(),
				}),
			}, txOpts)
		}

		logger.Warn().
			Int(fieldQueueAttemptKey, job.Attempts).
			Int(fieldQueueMaxAttemptsKey, job.MaxAttempts).
			Msg("event permanently failed after max retries")

		return nil
	})

	if failErr != nil {
		logger.Error().Err(failErr).Msg("failed to persist event failure state")
	}

	// Tell pgqueue whether this job should be retried or not.
	if job.Attempts < job.MaxAttempts {
		return cause
	}
	return pgqueue.NonRetryable(cause)
}

// scheduleRetry reverts event status to PENDING and delegates retry timing to pgqueue.
// It runs inside the failure transaction so the status update is committed.
func (s *integrationService) scheduleRetry(
	ctx context.Context,
	logger zerolog.Logger,
	event *integration.Event,
	job *pgqueue.Job,
	instance *integration.Instance,
	cause error,
	txOpts *jetx.DBOptions,
) error {
	backoff := pgqueue.ExponentialBackoff(
		integrationBackoffBase,
		job.Attempts,
		integrationBackoffMax,
	)

	errMsg := cause.Error()
	if retryErr := s.eventRepo.UpdateStatusInternal(
		ctx,
		event.ID,
		integration.EventStatusPending,
		&errMsg,
		txOpts,
	); retryErr != nil {
		logger.Error().Err(retryErr).Msg("failed to schedule retry")
		return retryErr
	}

	if instance != nil {
		s.writeAuditLog(ctx, logger, integration.AuditLog{
			IntegrationInstanceID: instance.ID,
			Action:                integration.AuditActionEventRetryScheduled,
			Severity:              integration.AuditSeverityWarning,
			Message:               "Webhook event scheduled for retry",
			MetadataJSON: provider.MustMarshalJSON(map[string]any{
				fieldEventIDKey:          event.ID,
				fieldEventTypeKey:        event.EventType,
				fieldQueueAttemptKey:     job.Attempts,
				fieldQueueMaxAttemptsKey: job.MaxAttempts,
				"backoff_seconds":        int(backoff.Seconds()),
				"error":                  cause.Error(),
			}),
		}, txOpts)
	}

	logger.Info().
		Int(fieldQueueAttemptKey, job.Attempts).
		Dur("backoff", backoff).
		Msg("event scheduled for retry")

	return nil
}

// ---------------------------------------------------------------------------
// Webhook Target Resolution
// ---------------------------------------------------------------------------

// resolveWebhookTarget looks up the provider and instance, validates status
// and webhook signature. Returns the provider as a WebhookIngestor; providers
// that do not implement that capability are rejected.
func (s *integrationService) resolveWebhookTarget(
	ctx context.Context,
	logger zerolog.Logger,
	input integration.IngestWebhookInput,
) (provider.WebhookIngestor, *integration.Instance, error) {
	prov, provErr := s.registry.GetProvider(string(input.ProviderType))
	if provErr != nil {
		logger.Warn().Msg("provider not registered")
		return nil, nil, ErrIntegrationProviderNotRegistered
	}

	webhookProv, ok := prov.(provider.WebhookIngestor)
	if !ok {
		logger.Warn().
			Str(fieldProviderTypeKey, string(input.ProviderType)).
			Msg("provider is not a webhook ingestor")
		return nil, nil, ErrIntegrationProviderNotWebhookIngestor
	}

	// Webhook endpoints are unauthenticated — no tenant context is available.
	// Use the unscoped lookup which relies on the UNIQUE(product_id, provider_type) constraint.
	instance, findErr := s.instanceRepo.FindByProductAndProviderInternal(
		ctx,
		input.ProductID,
		string(input.ProviderType),
		nil,
	)
	if findErr != nil {
		logger.Error().Err(findErr).
			Msg("failed to find integration instance")
		return nil, nil, findErr
	}
	if instance == nil {
		return nil, nil, ErrIntegrationInstanceNotFound
	}

	if !instance.CanIngest() {
		blockReason := instance.IngestionBlockReason()
		s.writeAuditLog(
			ctx,
			logger,
			integration.AuditLog{
				IntegrationInstanceID: instance.ID,
				Action:                integration.AuditActionWebhookFailed,
				Severity:              integration.AuditSeverityWarning,
				Message:               "Webhook rejected due to integration lifecycle state",
				MetadataJSON: provider.MustMarshalJSON(
					map[string]any{
						fieldStatusKey: string(instance.Status),
						"is_enabled":   instance.IsEnabled,
						"reason":       blockReason,
					},
				),
			},
			nil,
		)

		logger.Warn().
			Str("instance_id", instance.ID).
			Str(fieldStatusKey, string(instance.Status)).
			Bool("is_enabled", instance.IsEnabled).
			Str("reason", blockReason).
			Msg("instance not ready for ingestion, rejecting webhook")

		if !instance.IsEnabled {
			return nil, nil, ErrIntegrationInstanceDisabled
		}
		if instance.Status == integration.StatusConfiguring {
			return nil, nil, ErrIntegrationInstanceConfiguring
		}
		return nil, nil, ErrIntegrationInstanceUnhealthy
	}

	if instance.WebhookSecret != nil {
		sigErr := webhookProv.ValidateWebhook(
			ctx,
			input.Payload,
			input.Headers,
			*instance.WebhookSecret,
		)
		if sigErr != nil {
			s.writeAuditLog(
				ctx,
				logger,
				integration.AuditLog{
					IntegrationInstanceID: instance.ID,
					Action:                integration.AuditActionSignatureInvalid,
					Severity:              integration.AuditSeverityError,
					Message:               "Webhook signature validation failed",
					MetadataJSON: provider.MustMarshalJSON(
						map[string]any{
							fieldProviderTypeKey: string(input.ProviderType),
						},
					),
				},
				nil,
			)

			logger.Warn().Err(sigErr).
				Str("instance_id", instance.ID).
				Msg("webhook validation failed")
			return nil, nil, ErrIntegrationWebhookValidationFailed
		}
	}

	return webhookProv, instance, nil
}

// buildWebhookEvent parses the payload and constructs the event domain object.
func (s *integrationService) buildWebhookEvent(
	ctx context.Context,
	logger zerolog.Logger,
	prov provider.WebhookIngestor,
	instance *integration.Instance,
	input integration.IngestWebhookInput,
) (integration.Event, any, error) {
	parsedEvent, parseErr := prov.ParseEvent(ctx, "", input.Payload)
	if parseErr != nil {
		s.writeAuditLog(
			ctx,
			logger,
			integration.AuditLog{
				IntegrationInstanceID: instance.ID,
				Action:                integration.AuditActionEventParseFailed,
				Severity:              integration.AuditSeverityError,
				Message:               "Webhook payload parsing failed",
				MetadataJSON: provider.MustMarshalJSON(
					map[string]any{
						fieldProviderTypeKey: string(instance.ProviderType),
					},
				),
			},
			nil,
		)

		logger.Error().Err(parseErr).
			Str("instance_id", instance.ID).
			Msg("failed to parse webhook event")
		return integration.Event{}, nil,
			apierror.NewBadRequest(
				"INTEGRATION_EVENT_PARSE_FAILED",
				fmt.Sprintf(
					"Failed to parse webhook event: %s",
					parseErr.Error(),
				),
			)
	}

	eventType := prov.ExtractEventType(input.Payload)
	if eventType == "" {
		eventType = entityTypeUnknown
	}

	externalEventID, eventIDErr := extractExternalEventID(instance.ProviderType, input.Headers)
	if eventIDErr != nil {
		s.writeAuditLog(
			ctx,
			logger,
			integration.AuditLog{
				IntegrationInstanceID: instance.ID,
				Action:                integration.AuditActionSignatureInvalid,
				Severity:              integration.AuditSeverityError,
				Message:               "Webhook missing required event id header",
				MetadataJSON: provider.MustMarshalJSON(
					map[string]any{
						fieldProviderTypeKey: string(instance.ProviderType),
					},
				),
			},
			nil,
		)

		logger.Warn().Err(eventIDErr).
			Str("instance_id", instance.ID).
			Str(fieldProviderTypeKey, string(instance.ProviderType)).
			Msg("webhook missing required event id")

		return integration.Event{}, nil, ErrIntegrationWebhookEventIDMissing
	}

	headersJSON, _ := json.Marshal(input.Headers)
	now := time.Now()

	event := integration.Event{
		IntegrationInstanceID: instance.ID,
		ExternalEventID:       externalEventID,
		EventType:             eventType,
		PayloadJSON:           input.Payload,
		HeadersJSON:           headersJSON,
		Status:                integration.EventStatusPending,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	event.GenerateID()

	return event, parsedEvent, nil
}

// ---------------------------------------------------------------------------
// Connection verification
// ---------------------------------------------------------------------------

func (s *integrationService) startVerifyAndActivate(
	logger zerolog.Logger,
	inst integration.Instance,
	prov provider.Provider,
) {
	if _, ok := prov.(provider.ConnectionVerifier); !ok {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), integrationVerificationTimeout)
	run := &verificationRun{cancel: cancel}

	s.verificationMu.Lock()
	if previousRun := s.verificationRuns[inst.ID]; previousRun != nil {
		previousRun.cancel()
	}
	s.verificationRuns[inst.ID] = run
	s.verificationMu.Unlock()

	go s.verifyAndActivate(ctx, logger, inst, prov, run)
}

func (s *integrationService) isCurrentVerificationRun(instanceID string, run *verificationRun) bool {
	s.verificationMu.Lock()
	defer s.verificationMu.Unlock()

	return s.verificationRuns[instanceID] == run
}

func (s *integrationService) finishVerificationRun(instanceID string, run *verificationRun) {
	s.verificationMu.Lock()
	defer s.verificationMu.Unlock()

	if s.verificationRuns[instanceID] == run {
		delete(s.verificationRuns, instanceID)
	}
}

// verifyAndActivate calls VerifyConnection on providers that implement
// ConnectionVerifier, then updates the instance status to ACTIVE or ERROR.
// Run in a goroutine — never blocks the HTTP response. Verification is latest-
// wins per instance so repeated saves do not pile up stale verification writes.
func (s *integrationService) verifyAndActivate(
	ctx context.Context,
	logger zerolog.Logger,
	inst integration.Instance,
	prov provider.Provider,
	run *verificationRun,
) {
	defer func() {
		run.cancel()
		s.finishVerificationRun(inst.ID, run)
	}()

	verifier, ok := prov.(provider.ConnectionVerifier)
	if !ok {
		return
	}

	verifyErr := verifier.VerifyConnection(ctx, &inst)
	if !s.isCurrentVerificationRun(inst.ID, run) {
		logger.Debug().Str("instance_id", inst.ID).
			Msg("verifyAndActivate: skipped superseded verification result")
		return
	}

	persistCtx, cancel := context.WithTimeout(context.Background(), integrationVerificationPersistTimeout)
	defer cancel()

	// Re-fetch the live instance so we update current state, not a stale snapshot.
	live, findErr := s.instanceRepo.FindByIDInternal(persistCtx, inst.ID, nil)
	if findErr != nil || live == nil {
		logger.Error().Err(findErr).Str("instance_id", inst.ID).
			Msg("verifyAndActivate: failed to re-fetch instance after verification")
		return
	}
	if !s.isCurrentVerificationRun(inst.ID, run) {
		logger.Debug().Str("instance_id", inst.ID).
			Msg("verifyAndActivate: skipped superseded verification update")
		return
	}

	if verifyErr != nil {
		errMsg := verifyErr.Error()
		live.Status = integration.StatusError
		live.LastError = &errMsg
	} else {
		live.Status = integration.StatusActive
		live.LastError = nil
	}

	if _, updateErr := s.instanceRepo.Update(persistCtx, live.PlatformTenantID, *live, nil); updateErr != nil {
		logger.Error().Err(updateErr).Str("instance_id", inst.ID).
			Msg("verifyAndActivate: failed to persist verification result")
	}
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func (s *integrationService) prepareConfigForStorage(
	ctx context.Context,
	prov provider.Provider,
	configJSON json.RawMessage,
) (json.RawMessage, error) {
	storageProvider, ok := prov.(provider.ConfigStorageProvider)
	if !ok {
		return configJSON, nil
	}

	normalized, err := storageProvider.PrepareConfigForStorage(ctx, configJSON)
	if err != nil {
		return nil, err
	}

	return normalized, nil
}

func (s *integrationService) prepareConfigForUpdatedStorage(
	ctx context.Context,
	prov provider.Provider,
	currentConfigJSON json.RawMessage,
	configJSON json.RawMessage,
) (json.RawMessage, error) {
	storageProvider, ok := prov.(provider.ConfigUpdateStorageProvider)
	if ok {
		return storageProvider.PrepareUpdatedConfigForStorage(ctx, currentConfigJSON, configJSON)
	}

	return s.prepareConfigForStorage(ctx, prov, configJSON)
}

func (s *integrationService) enqueueReconcileJobIfRequired(
	ctx context.Context,
	logger zerolog.Logger,
	instance integration.Instance,
	shouldEnqueue bool,
	tx *sql.Tx,
) error {
	if !shouldEnqueue {
		return nil
	}
	if instance.ProviderType != integration.ProviderTypeClerk {
		return nil
	}

	payload, err := json.Marshal(integrationReconcileQueuePayload{
		IntegrationInstanceID: instance.ID,
	})
	if err != nil {
		return err
	}

	if _, enqueueErr := s.queue.EnqueueTx(ctx, tx, pgqueue.EnqueueParams{
		QueueName:   integrationReconcileQueueName,
		Payload:     payload,
		MaxAttempts: integrationMaxAttempts,
	}); enqueueErr != nil {
		logger.Error().Err(enqueueErr).
			Str("integration_instance_id", instance.ID).
			Msg("failed to enqueue integration reconcile job")
		return enqueueErr
	}

	logger.Info().
		Str("integration_instance_id", instance.ID).
		Str(fieldProviderTypeKey, string(instance.ProviderType)).
		Msg("integration reconcile job enqueued")

	return nil
}

func hasClerkAPIKey(configJSON json.RawMessage) bool {
	if len(configJSON) == 0 || string(configJSON) == "{}" {
		return false
	}

	var cfg struct {
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return false
	}

	return strings.TrimSpace(cfg.APIKey) != ""
}

// markEventFailed sets the event status to FAILED with an error message.
func (s *integrationService) markEventFailed(
	ctx context.Context,
	eventID string,
	cause error,
	txOpts *jetx.DBOptions,
) {
	errMsg := cause.Error()
	_ = s.eventRepo.UpdateStatusInternal(ctx, eventID, integration.EventStatusFailed, &errMsg, txOpts)
}

func (s *integrationService) findDuplicateEvent(
	ctx context.Context,
	logger zerolog.Logger,
	instance *integration.Instance,
	event integration.Event,
	txOpts *jetx.DBOptions,
) (*integration.Event, error) {
	existing, err := s.eventRepo.FindByExternalEventIDInternal(
		ctx,
		instance.ID,
		event.ExternalEventID,
		txOpts,
	)
	if err != nil {
		logger.Error().Err(err).Msg("failed to check for duplicate event")
		return nil, err
	}
	if existing != nil {
		logger.Debug().
			Str("external_event_id", event.ExternalEventID).
			Msg("duplicate event, skipping")
	}

	return existing, nil
}

func (s *integrationService) writeAuditLog(
	ctx context.Context,
	logger zerolog.Logger,
	auditLog integration.AuditLog,
	txOpts *jetx.DBOptions,
) {
	if auditLog.ID == "" {
		auditLog.GenerateID()
	}
	if auditLog.CreatedAt.IsZero() {
		auditLog.CreatedAt = time.Now()
	}
	if auditLog.Severity == "" {
		auditLog.Severity = integration.AuditSeverityInfo
	}
	if auditLog.Message == "" {
		auditLog.Message = "Integration activity recorded"
	}

	if _, auditErr := s.auditLogRepo.Create(ctx, auditLog, txOpts); auditErr != nil {
		logger.Error().Err(auditErr).
			Str("integration_instance_id", auditLog.IntegrationInstanceID).
			Str("action", auditLog.Action).
			Msg("failed to create integration audit log")
	}
}

// executeCommands delegates each canonical command to the owning provider for
// execution. The provider is responsible for dispatching unknown command types.
func (s *integrationService) executeCommands(
	ctx context.Context,
	logger zerolog.Logger,
	prov provider.WebhookIngestor,
	instance *integration.Instance,
	commands []provider.Command,
	txOpts *jetx.DBOptions,
) error {
	for _, cmd := range commands {
		if err := prov.ExecuteCommand(ctx, logger, instance, cmd, txOpts); err != nil {
			return err
		}
	}
	return nil
}

func (s *integrationService) executeCommandsInBatches(
	ctx context.Context,
	logger zerolog.Logger,
	prov provider.WebhookIngestor,
	instance *integration.Instance,
	commands []provider.Command,
	batchSize int,
) (int, error) {
	if len(commands) == 0 {
		return 0, nil
	}
	if batchSize <= 0 {
		batchSize = len(commands)
	}

	batchesProcessed := 0
	for start := 0; start < len(commands); start += batchSize {
		end := start + batchSize
		if end > len(commands) {
			end = len(commands)
		}

		batchIndex := batchesProcessed + 1
		batchCommands := commands[start:end]

		txErr := jetx.WithTx(s.db, func(tx *sql.Tx) error {
			txOpts := &jetx.DBOptions{Tx: tx}
			if err := s.executeCommands(ctx, logger, prov, instance, batchCommands, txOpts); err != nil {
				return err
			}

			s.writeAuditLog(ctx, logger, integration.AuditLog{
				IntegrationInstanceID: instance.ID,
				Action:                integration.AuditActionReconcileBatch,
				Severity:              integration.AuditSeverityInfo,
				Message:               "Integration reconciliation batch processed",
				MetadataJSON: provider.MustMarshalJSON(map[string]any{
					"batch_index": batchIndex,
					"batch_size":  len(batchCommands),
					"range_start": start,
					"range_end":   end,
				}),
			}, txOpts)

			return nil
		})
		if txErr != nil {
			return batchesProcessed, txErr
		}

		batchesProcessed++
		logger.Info().
			Str("integration_instance_id", instance.ID).
			Int("batch_index", batchIndex).
			Int("batch_size", len(batchCommands)).
			Int("commands_total", len(commands)).
			Msg("integration reconciliation batch committed")
	}

	return batchesProcessed, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func extractExternalEventID(
	providerType integration.ProviderType,
	headers map[string]string,
) (string, error) {
	if providerType == integration.ProviderTypeClerk {
		externalEventID := headers["svix-id"]
		if externalEventID == "" {
			return "", errors.New("missing svix-id header")
		}
		return externalEventID, nil
	}

	return ids.MustNew("evt"), nil
}

// newInvalidConfigError creates a standardized bad-request error for invalid
// provider configuration.
func newInvalidConfigError(cause error) error {
	return apierror.NewBadRequest(
		"INTEGRATION_INVALID_CONFIG",
		fmt.Sprintf("Invalid provider config: %s", cause.Error()),
	)
}
