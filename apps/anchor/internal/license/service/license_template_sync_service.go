package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nanostack-dev/nanostack-framework/modules/cache"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/pgkit/queue"
	"github.com/rs/zerolog"

	"anchor/internal/domain/license"
	licenserepo "anchor/internal/license/repository"
)

// See docs/adr/0017-license-follows-its-template.md.
const (
	licenseTemplateSyncQueueName   = "license_template_sync"
	licenseTemplateSyncBatchSize   = 100
	licenseTemplateSyncMaxAttempts = 6
)

type licenseTemplateSyncPayload struct {
	TenantID            string `json:"tenant_id"`
	ProductID           string `json:"product_id"`
	TemplateID          string `json:"template_id"`
	AfterOrganizationID string `json:"after_organization_id,omitempty"`
}

type LicenseTemplateSyncEnqueuer interface {
	EnqueueTemplateSync(ctx context.Context, tenantID, productID, templateID string) error
}

type licenseTemplateSyncEnqueuer struct {
	queue *queue.Client
}

func NewLicenseTemplateSyncEnqueuer(queueClient *queue.Client) LicenseTemplateSyncEnqueuer {
	return &licenseTemplateSyncEnqueuer{queue: queueClient}
}

func (e *licenseTemplateSyncEnqueuer) EnqueueTemplateSync(
	ctx context.Context, tenantID, productID, templateID string,
) error {
	return enqueueTemplateSync(ctx, e.queue, licenseTemplateSyncPayload{
		TenantID:   tenantID,
		ProductID:  productID,
		TemplateID: templateID,
	})
}

func enqueueTemplateSync(
	ctx context.Context, queueClient *queue.Client, payload licenseTemplateSyncPayload,
) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	params := queue.EnqueueParams{
		QueueName:   licenseTemplateSyncQueueName,
		Payload:     encoded,
		MaxAttempts: licenseTemplateSyncMaxAttempts,
	}
	if tx := transactor.CurrentTx(ctx); tx != nil {
		_, err = queueClient.EnqueueTx(ctx, tx, params)
		return err
	}
	_, err = queueClient.Enqueue(ctx, params)
	return err
}

type LicenseTemplateSyncService interface {
	ProcessQueueJob(ctx context.Context, job queue.Job) error
}

type licenseTemplateSyncService struct {
	templateRepo licenserepo.TemplateRepository
	licenseRepo  licenserepo.OrganizationLicenseRepository
	changes      licenserepo.OrganizationLicenseChangeRepository
	schemas      LicenseSchemaService
	transactor   transactor.Transactor
	queue        *queue.Client
	licenses     *organizationLicenseCache
	logger       zerolog.Logger
}

func NewLicenseTemplateSyncService(
	templateRepo licenserepo.TemplateRepository,
	licenseRepo licenserepo.OrganizationLicenseRepository,
	changes licenserepo.OrganizationLicenseChangeRepository,
	schemas LicenseSchemaService,
	tx transactor.Transactor,
	queueClient *queue.Client,
	cacheStore cache.Store,
	logger zerolog.Logger,
) LicenseTemplateSyncService {
	return &licenseTemplateSyncService{
		templateRepo: templateRepo,
		licenseRepo:  licenseRepo,
		changes:      changes,
		schemas:      schemas,
		transactor:   tx,
		queue:        queueClient,
		licenses:     newOrganizationLicenseCache(cacheStore, logger),
		logger:       logger.With().Str("component", "license_template_sync_service").Logger(),
	}
}

func (s *licenseTemplateSyncService) ProcessQueueJob(
	ctx context.Context, job queue.Job,
) error {
	var payload licenseTemplateSyncPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return queue.NonRetryable(fmt.Errorf("invalid license template sync payload: %w", err))
	}
	if payload.TenantID == "" || payload.ProductID == "" || payload.TemplateID == "" {
		return queue.NonRetryable(errors.New(
			"license template sync payload missing tenant_id, product_id or template_id",
		))
	}

	found, err := s.templateRepo.FindByID(
		ctx, payload.TenantID, payload.ProductID, payload.TemplateID,
	)
	if err != nil {
		return err
	}
	if found.IsAbsent() {
		return nil
	}

	organizationIDs, err := s.licenseRepo.ListOrganizationIDsForTemplateAfter(
		ctx,
		payload.TenantID,
		payload.ProductID,
		payload.TemplateID,
		payload.AfterOrganizationID,
		licenseTemplateSyncBatchSize,
	)
	if err != nil {
		return err
	}

	synced, refused := 0, 0
	for _, organizationID := range organizationIDs {
		outcome, syncErr := s.syncOne(ctx, payload, organizationID)
		if syncErr != nil {
			return syncErr
		}
		switch outcome {
		case syncOutcomeSynced:
			synced++
		case syncOutcomeRefused:
			refused++
		case syncOutcomeUnchanged:
		}
	}

	s.logger.Info().
		Str("product_id", payload.ProductID).
		Str("license_template_id", payload.TemplateID).
		Int("considered", len(organizationIDs)).
		Int("synced", synced).
		Int("refused", refused).
		Msg("license template sync batch finished")

	if len(organizationIDs) < licenseTemplateSyncBatchSize {
		return nil
	}
	payload.AfterOrganizationID = organizationIDs[len(organizationIDs)-1]
	return enqueueTemplateSync(ctx, s.queue, payload)
}

type syncOutcome int

const (
	syncOutcomeUnchanged syncOutcome = iota
	syncOutcomeSynced
	syncOutcomeRefused
)

// Re-reads the template under the row lock: a stale copy could otherwise
// overwrite a newer concurrent job's writes for good.
func (s *licenseTemplateSyncService) syncOne(
	ctx context.Context,
	payload licenseTemplateSyncPayload,
	organizationID string,
) (syncOutcome, error) {
	changedAt := time.Now()
	outcome := syncOutcomeUnchanged

	if txErr := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		foundExisting, findErr := s.licenseRepo.FindByOrganizationForUpdate(
			txCtx, payload.TenantID, payload.ProductID, organizationID,
		)
		if findErr != nil {
			return findErr
		}
		if foundExisting.IsAbsent() {
			return nil
		}
		existing := foundExisting.Value()
		if existing.TemplateID != payload.TemplateID {
			return nil
		}

		foundTemplate, templateErr := s.templateRepo.FindByID(
			txCtx, payload.TenantID, payload.ProductID, payload.TemplateID,
		)
		if templateErr != nil {
			return templateErr
		}
		if foundTemplate.IsAbsent() {
			return nil
		}
		template := foundTemplate.Value()

		previous := existing.Values
		existing.Values = existing.SyncedValues(template.Values)
		if len(license.DiffValues(previous, existing.Values)) == 0 {
			return nil
		}

		if validateErr := s.schemas.ValidateValues(
			txCtx, payload.TenantID, payload.ProductID, existing.Values,
		); validateErr != nil {
			outcome = syncOutcomeRefused
			s.logger.Warn().
				Str("product_id", payload.ProductID).
				Str("organization_id", organizationID).
				Str("license_template_id", template.ID).
				Err(validateErr).
				Msg("license refused a template sync: merged values no longer satisfy the schema")
			return nil
		}

		updated, updateErr := s.licenseRepo.Update(txCtx, payload.TenantID, existing)
		if updateErr != nil {
			return updateErr
		}
		outcome = syncOutcomeSynced
		return s.changes.Append(txCtx, []license.OrganizationLicenseChange{
			license.NewTemplateSyncChange(updated, previous, changedAt),
		})
	}); txErr != nil {
		return outcome, txErr
	}

	if outcome == syncOutcomeSynced {
		s.licenses.evict(ctx, payload.ProductID, organizationID)
	}
	return outcome, nil
}
