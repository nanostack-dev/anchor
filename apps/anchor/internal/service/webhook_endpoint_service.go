package service

import (
	"context"
	"fmt"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/secrets"
	"github.com/nanostack-dev/pgkit/pgqueue"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"anchor/internal/domain/webhook"
	"anchor/internal/repository"
	"anchor/internal/security/encryption"
)

// EndpointWithSecret carries a plaintext signing secret alongside the endpoint.
// It is produced exactly twice in the lifetime of a secret: at endpoint
// creation and at rotation. Nothing else ever returns the plaintext.
type EndpointWithSecret struct {
	Endpoint webhook.Endpoint
	// PlaintextSecret is shown once and then unrecoverable.
	PlaintextSecret string
}

// DeliveryDetail is a delivery plus its attempt log.
type DeliveryDetail struct {
	Delivery webhook.Delivery
	Event    webhook.Event
	Attempts []webhook.Attempt
}

// TestEventResult is a synthetic send: the event that was queued and the
// deliveries it produced. The delivery ids are the point — they let the caller
// poll the outcome of this exact send instead of guessing which row in the log
// belongs to it.
type TestEventResult struct {
	Event      webhook.Event
	Deliveries []webhook.Delivery
}

// WebhookEndpointService owns the product-facing lifecycle of webhook
// subscriptions: CRUD, enable/disable, secret rotation, ping, and the delivery
// log with manual replay.
type WebhookEndpointService interface {
	List(ctx context.Context, input webhook.ListEndpointsInput) ([]webhook.Endpoint, error)
	Get(ctx context.Context, input webhook.GetEndpointInput) (*webhook.Endpoint, error)
	Create(ctx context.Context, input webhook.CreateEndpointInput) (EndpointWithSecret, error)
	Update(ctx context.Context, input webhook.UpdateEndpointInput) (webhook.Endpoint, error)
	Delete(ctx context.Context, input webhook.DeleteEndpointInput) error
	SetEnabled(
		ctx context.Context, input webhook.SetEndpointEnabledInput,
	) (webhook.Endpoint, error)
	RotateSecret(ctx context.Context, input webhook.RotateSecretInput) (EndpointWithSecret, error)
	SendTestEvent(
		ctx context.Context, input webhook.SendTestEventInput,
	) (TestEventResult, error)
	ListDeliveries(
		ctx context.Context, input webhook.ListDeliveriesInput,
	) ([]webhook.DeliveryWithEvent, error)
	GetDelivery(ctx context.Context, input webhook.GetDeliveryInput) (*DeliveryDetail, error)
	RetryDelivery(
		ctx context.Context, input webhook.RetryDeliveryInput,
	) (webhook.Delivery, error)
}

type webhookEndpointService struct {
	endpointRepo  repository.WebhookEndpointRepository
	secretRepo    repository.WebhookEndpointSecretRepository
	eventRepo     repository.WebhookEventRepository
	deliveryRepo  repository.WebhookDeliveryRepository
	emitter       WebhookEmitter
	fanout        WebhookFanoutService
	queue         *pgqueue.Client
	cipher        *secrets.VersionedCipher
	transactor    transactor.Transactor
	allowInsecure bool
	logger        zerolog.Logger
}

type WebhookEndpointServiceParams struct {
	fx.In
	EndpointRepo      repository.WebhookEndpointRepository
	SecretRepo        repository.WebhookEndpointSecretRepository
	EventRepo         repository.WebhookEventRepository
	DeliveryRepo      repository.WebhookDeliveryRepository
	Emitter           WebhookEmitter
	Fanout            WebhookFanoutService
	Queue             *pgqueue.Client
	EncryptionService *encryption.Service
	HTTPClient        *WebhookHTTPClient
	Transactor        transactor.Transactor
	Logger            zerolog.Logger
}

func NewWebhookEndpointService(
	params WebhookEndpointServiceParams,
) (WebhookEndpointService, error) {
	cipher, err := params.EncryptionService.NewCipher(WebhookSigningSecretContext)
	if err != nil {
		return nil, fmt.Errorf("build webhook signing secret cipher: %w", err)
	}

	return &webhookEndpointService{
		endpointRepo:  params.EndpointRepo,
		secretRepo:    params.SecretRepo,
		eventRepo:     params.EventRepo,
		deliveryRepo:  params.DeliveryRepo,
		emitter:       params.Emitter,
		fanout:        params.Fanout,
		queue:         params.Queue,
		cipher:        cipher,
		transactor:    params.Transactor,
		allowInsecure: params.HTTPClient.AllowsInsecureTargets(),
		logger:        params.Logger.With().Str("component", "webhook_endpoint_service").Logger(),
	}, nil
}

// ---------------------------------------------------------------------------
// Endpoint CRUD
// ---------------------------------------------------------------------------

func (s *webhookEndpointService) List(
	ctx context.Context, input webhook.ListEndpointsInput,
) ([]webhook.Endpoint, error) {
	if err := validateStruct(input); err != nil {
		return nil, err
	}

	return s.endpointRepo.ListByProduct(ctx, input.ProductID)
}

func (s *webhookEndpointService) Get(
	ctx context.Context, input webhook.GetEndpointInput,
) (*webhook.Endpoint, error) {
	if err := validateStruct(input); err != nil {
		return nil, err
	}

	return s.endpointRepo.FindByID(ctx, input.ProductID, input.EndpointID)
}

func (s *webhookEndpointService) Create(
	ctx context.Context, input webhook.CreateEndpointInput,
) (EndpointWithSecret, error) {
	logger := s.logger.With().Str("operation", "Create").Logger()

	if err := validateStruct(input); err != nil {
		return EndpointWithSecret{}, err
	}
	if err := webhook.ValidateTargetURL(input.URL, s.allowInsecure); err != nil {
		return EndpointWithSecret{}, NewInvalidWebhookURLError(err.Error())
	}
	if err := validateSubscriptions(input.EventTypes); err != nil {
		return EndpointWithSecret{}, err
	}

	plaintext, err := webhook.GenerateSecret()
	if err != nil {
		return EndpointWithSecret{}, NewWebhookSecretGenerationError(err)
	}
	encrypted, err := s.cipher.EncryptString(plaintext)
	if err != nil {
		return EndpointWithSecret{}, NewWebhookSecretGenerationError(err)
	}

	now := time.Now()
	endpoint := webhook.Endpoint{
		ProductID:   input.ProductID,
		URL:         input.URL,
		Description: input.Description,
		EventTypes:  input.EventTypes,
		Status:      webhook.EndpointStatusEnabled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	endpoint.GenerateID()

	var created webhook.Endpoint
	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		var createErr error
		created, createErr = s.endpointRepo.Create(txCtx, endpoint)
		if createErr != nil {
			logger.Error().Err(createErr).Msg("failed to create webhook endpoint")
			return createErr
		}

		secret := webhook.Secret{
			EndpointID:      created.ID,
			EncryptedSecret: encrypted,
			Status:          webhook.SecretStatusActive,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		secret.GenerateID()

		if _, secretErr := s.secretRepo.Create(txCtx, secret); secretErr != nil {
			logger.Error().Err(secretErr).Msg("failed to create webhook signing secret")
			return secretErr
		}

		return nil
	})
	if err != nil {
		return EndpointWithSecret{}, err
	}

	logger.Info().Str("webhook_endpoint_id", created.ID).
		Str("product_id", created.ProductID).Msg("webhook endpoint created")

	return EndpointWithSecret{Endpoint: created, PlaintextSecret: plaintext}, nil
}

func (s *webhookEndpointService) Update(
	ctx context.Context, input webhook.UpdateEndpointInput,
) (webhook.Endpoint, error) {
	logger := s.logger.With().Str("operation", "Update").Logger()

	if err := validateStruct(input); err != nil {
		return webhook.Endpoint{}, err
	}
	if input.URL != nil {
		if err := webhook.ValidateTargetURL(*input.URL, s.allowInsecure); err != nil {
			return webhook.Endpoint{}, NewInvalidWebhookURLError(err.Error())
		}
	}
	if input.EventTypes != nil {
		if err := validateSubscriptions(*input.EventTypes); err != nil {
			return webhook.Endpoint{}, err
		}
	}

	var updated webhook.Endpoint
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		existing, findErr := s.endpointRepo.FindByID(txCtx, input.ProductID, input.EndpointID)
		if findErr != nil {
			return findErr
		}
		if existing == nil {
			return fault.ErrNotFound
		}

		next := *existing
		if input.URL != nil {
			next.URL = *input.URL
		}
		if input.Description != nil {
			next.Description = *input.Description
		}
		if input.EventTypes != nil {
			next.EventTypes = *input.EventTypes
		}

		var updateErr error
		updated, updateErr = s.endpointRepo.Update(txCtx, input.ProductID, next)
		if updateErr != nil {
			logger.Error().Err(updateErr).Msg("failed to update webhook endpoint")
		}

		return updateErr
	})

	return updated, err
}

func (s *webhookEndpointService) Delete(
	ctx context.Context, input webhook.DeleteEndpointInput,
) error {
	if err := validateStruct(input); err != nil {
		return err
	}

	return s.transactor.InTx(ctx, func(txCtx context.Context) error {
		existing, findErr := s.endpointRepo.FindByID(txCtx, input.ProductID, input.EndpointID)
		if findErr != nil {
			return findErr
		}
		if existing == nil {
			return fault.ErrNotFound
		}

		return s.endpointRepo.DeleteByID(txCtx, input.ProductID, input.EndpointID)
	})
}

// SetEnabled backs the enable and disable sub-resources. They are sub-resources
// rather than a PATCH on `status` because that produces cleaner permissions and
// unambiguous audit entries — and because re-enabling an AUTO_DISABLED endpoint
// must also clear the streak that disabled it.
func (s *webhookEndpointService) SetEnabled(
	ctx context.Context, input webhook.SetEndpointEnabledInput,
) (webhook.Endpoint, error) {
	if err := validateStruct(input); err != nil {
		return webhook.Endpoint{}, err
	}

	var updated webhook.Endpoint
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		existing, findErr := s.endpointRepo.FindByID(txCtx, input.ProductID, input.EndpointID)
		if findErr != nil {
			return findErr
		}
		if existing == nil {
			return fault.ErrNotFound
		}

		next := *existing
		if input.Enabled {
			next.Status = webhook.EndpointStatusEnabled
			next.DisabledReason = ""
			next.ConsecutiveFailureCount = 0
			next.FirstFailureAt = nil
		} else {
			next.Status = webhook.EndpointStatusDisabled
			next.DisabledReason = "Disabled by a platform administrator"
		}

		var updateErr error
		updated, updateErr = s.endpointRepo.Update(txCtx, input.ProductID, next)

		return updateErr
	})

	return updated, err
}

// ---------------------------------------------------------------------------
// Secret rotation
// ---------------------------------------------------------------------------

// RotateSecret inserts a new ACTIVE secret and marks the previous one EXPIRING.
// Both signatures ride in the space-delimited signature header until the old
// one expires, so a receiver can roll over without coordination or downtime.
func (s *webhookEndpointService) RotateSecret(
	ctx context.Context, input webhook.RotateSecretInput,
) (EndpointWithSecret, error) {
	logger := s.logger.With().Str("operation", "RotateSecret").Logger()

	if err := validateStruct(input); err != nil {
		return EndpointWithSecret{}, err
	}

	plaintext, err := webhook.GenerateSecret()
	if err != nil {
		return EndpointWithSecret{}, NewWebhookSecretGenerationError(err)
	}
	encrypted, err := s.cipher.EncryptString(plaintext)
	if err != nil {
		return EndpointWithSecret{}, NewWebhookSecretGenerationError(err)
	}

	var endpoint webhook.Endpoint
	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		existing, findErr := s.endpointRepo.FindByID(txCtx, input.ProductID, input.EndpointID)
		if findErr != nil {
			return findErr
		}
		if existing == nil {
			return fault.ErrNotFound
		}
		endpoint = *existing

		now := time.Now()
		if expireErr := s.secretRepo.ExpireActiveInternal(
			txCtx, existing.ID, now.Add(webhook.SecretRotationGrace),
		); expireErr != nil {
			return expireErr
		}

		secret := webhook.Secret{
			EndpointID:      existing.ID,
			EncryptedSecret: encrypted,
			Status:          webhook.SecretStatusActive,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		secret.GenerateID()

		_, secretErr := s.secretRepo.Create(txCtx, secret)

		return secretErr
	})
	if err != nil {
		return EndpointWithSecret{}, err
	}

	logger.Info().Str("webhook_endpoint_id", endpoint.ID).
		Msg("webhook signing secret rotated")

	return EndpointWithSecret{Endpoint: endpoint, PlaintextSecret: plaintext}, nil
}

// ---------------------------------------------------------------------------
// Test events
// ---------------------------------------------------------------------------

// SendTestEvent emits a synthetic event aimed at exactly one endpoint. It goes
// through the same outbox, fan-out, signing and delivery path as a business
// event, so a green test send proves the whole chain rather than just
// reachability. The envelope carries `test: true` so a receiver can refuse to
// act on it.
//
// Fan-out runs inside the emitting transaction rather than waiting for the
// queued job. That is what lets the caller be told which delivery its send
// produced — and it costs nothing, because the delivery row and the fan-out job
// become visible at the same commit, leaving the job to find the row already
// there and skip it.
func (s *webhookEndpointService) SendTestEvent(
	ctx context.Context, input webhook.SendTestEventInput,
) (TestEventResult, error) {
	if err := validateStruct(input); err != nil {
		return TestEventResult{}, err
	}

	eventType := input.ResolvedEventType()
	if err := webhook.Validate(eventType); err != nil {
		return TestEventResult{}, NewInvalidWebhookEventTypesError(err.Error())
	}

	var result TestEventResult
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		endpoint, findErr := s.endpointRepo.FindByID(txCtx, input.ProductID, input.EndpointID)
		if findErr != nil {
			return findErr
		}
		if endpoint == nil {
			return fault.ErrNotFound
		}
		if !endpoint.IsEnabled() {
			return NewWebhookEndpointNotEnabledError(endpoint.ID)
		}

		data, dataErr := webhook.TestEventData(eventType, endpoint.ID)
		if dataErr != nil {
			return NewInvalidWebhookEventTypesError(dataErr.Error())
		}

		endpointID := endpoint.ID
		event, emitErr := s.emitter.Emit(txCtx, webhook.EmitInput{
			ProductID:        input.ProductID,
			EventType:        eventType,
			TargetEndpointID: &endpointID,
			Data:             data,
		})
		if emitErr != nil {
			return emitErr
		}

		deliveries, fanErr := s.fanout.CreateDeliveriesInTx(
			txCtx, event, []webhook.Endpoint{*endpoint},
		)
		if fanErr != nil {
			return fanErr
		}

		result = TestEventResult{Event: event, Deliveries: deliveries}

		return nil
	})
	if err != nil {
		return TestEventResult{}, err
	}

	s.logger.Info().
		Str("webhook_endpoint_id", input.EndpointID).
		Str("event_type", eventType).
		Str("event_id", result.Event.ID).
		Msg("webhook test event queued")

	return result, nil
}

// ---------------------------------------------------------------------------
// Delivery log
// ---------------------------------------------------------------------------

func (s *webhookEndpointService) ListDeliveries(
	ctx context.Context, input webhook.ListDeliveriesInput,
) ([]webhook.DeliveryWithEvent, error) {
	if err := validateStruct(input); err != nil {
		return nil, err
	}
	if input.Status != nil && !input.Status.IsValid() {
		return nil, fault.BadRequest(
			"INVALID_WEBHOOK_DELIVERY_STATUS",
			fmt.Sprintf("Delivery status %q is invalid", string(*input.Status)),
		)
	}

	endpoint, err := s.endpointRepo.FindByID(ctx, input.ProductID, input.EndpointID)
	if err != nil {
		return nil, err
	}
	if endpoint == nil {
		return nil, fault.ErrNotFound
	}

	return s.deliveryRepo.ListByEndpoint(ctx, input)
}

func (s *webhookEndpointService) GetDelivery(
	ctx context.Context, input webhook.GetDeliveryInput,
) (*DeliveryDetail, error) {
	if err := validateStruct(input); err != nil {
		return nil, err
	}

	delivery, err := s.deliveryRepo.FindByIDForEndpoint(
		ctx, input.ProductID, input.EndpointID, input.DeliveryID,
	)
	if err != nil {
		return nil, err
	}
	if delivery == nil {
		return nil, nil
	}

	event, err := s.eventRepo.FindByIDForProduct(ctx, input.ProductID, delivery.EventID)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, fault.ErrNotFound
	}

	attempts, err := s.deliveryRepo.ListAttempts(ctx, delivery.ID)
	if err != nil {
		return nil, err
	}

	return &DeliveryDetail{Delivery: *delivery, Event: *event, Attempts: attempts}, nil
}

// RetryDelivery replays a finished delivery as a brand new row pointing back at
// the original. The frozen body is reused verbatim: re-marshalling it would
// produce a different signature for what is meant to be the same event.
func (s *webhookEndpointService) RetryDelivery(
	ctx context.Context, input webhook.RetryDeliveryInput,
) (webhook.Delivery, error) {
	logger := s.logger.With().Str("operation", "RetryDelivery").Logger()

	if err := validateStruct(input); err != nil {
		return webhook.Delivery{}, err
	}

	var replay webhook.Delivery
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		original, findErr := s.deliveryRepo.FindByIDForEndpoint(
			txCtx, input.ProductID, input.EndpointID, input.DeliveryID,
		)
		if findErr != nil {
			return findErr
		}
		if original == nil {
			return fault.ErrNotFound
		}
		if original.IsReplay() {
			return NewWebhookDeliveryNotReplayableError(original.ID)
		}
		if !original.Status.IsTerminal() {
			return NewWebhookDeliveryStillPendingError(original.ID)
		}

		endpoint, endpointErr := s.endpointRepo.FindByID(
			txCtx, input.ProductID, input.EndpointID,
		)
		if endpointErr != nil {
			return endpointErr
		}
		if endpoint == nil {
			return fault.ErrNotFound
		}
		if !endpoint.IsEnabled() {
			return NewWebhookEndpointNotEnabledError(endpoint.ID)
		}

		now := time.Now()
		originalID := original.ID
		replay = webhook.Delivery{
			EventID:            original.EventID,
			EndpointID:         original.EndpointID,
			ProductID:          original.ProductID,
			Status:             webhook.DeliveryStatusPending,
			MaxAttempts:        webhook.MaxDeliveryAttempts,
			TargetURL:          endpoint.URL,
			SignedBody:         original.SignedBody,
			ReplayOfDeliveryID: &originalID,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		replay.GenerateID()

		stored, createErr := s.deliveryRepo.CreateInternal(txCtx, replay)
		if createErr != nil {
			return createErr
		}
		replay = stored

		return enqueueWebhookDelivery(
			txCtx, s.queue, transactor.CurrentTx(txCtx), stored.ID,
		)
	})
	if err != nil {
		return webhook.Delivery{}, err
	}

	logger.Info().Str("delivery_id", replay.ID).
		Str("replay_of_delivery_id", input.DeliveryID).
		Msg("webhook delivery replayed")

	return replay, nil
}

// validateSubscriptions rejects unknown types and unknown wildcard groups
// before they are persisted, so a subscription can never silently match nothing.
func validateSubscriptions(subscriptions []string) error {
	for _, subscription := range subscriptions {
		if err := webhook.ValidateSubscription(subscription); err != nil {
			return NewInvalidWebhookEventTypesError(err.Error())
		}
	}

	return nil
}
