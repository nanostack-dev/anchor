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

// A template value update is propagated onto every license instantiated from
// that template, except on each license's adjusted fields. See
// docs/adr/0017-license-follows-its-template.md.
//
// The propagation is a durable pgqueue job, not an inline loop: a product can
// hold far more licenses than fit one request transaction, and a template
// edit must survive a restart between the write and the last license synced.
// One job covers one template and one page of its organizations; a job that
// finds more re-enqueues a continuation carrying a cursor.
const (
	licenseTemplateSyncQueueName = "license_template_sync"
	// licenseTemplateSyncBatchSize bounds one job execution. Each
	// organization is written in its own transaction, so the bound is about
	// keeping one execution inside the worker's visibility timeout, not about
	// atomicity.
	licenseTemplateSyncBatchSize   = 100
	licenseTemplateSyncMaxAttempts = 6
)

type licenseTemplateSyncPayload struct {
	TenantID  string `json:"tenant_id"`
	ProductID string `json:"product_id"`
	// TemplateID names the template whose values are propagated. The values
	// themselves are read at processing time, not carried here, so two rapid
	// edits coalesce onto the final state and a re-run is a no-op.
	TemplateID string `json:"template_id"`
	// AfterOrganizationID is the continuation cursor: only organizations with
	// a greater identifier are considered. Empty on the first job of a run.
	AfterOrganizationID string `json:"after_organization_id,omitempty"`
}

// LicenseTemplateSyncEnqueuer schedules the propagation of one template's
// values onto the licenses naming it. It joins the caller's transaction, so
// the template write and the job land together or not at all.
//
// Split from LicenseTemplateSyncService so the template and schema services
// can enqueue without depending on the processor, which itself depends on the
// schema service for validation.
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

// LicenseTemplateSyncService processes one propagation job: it re-reads the
// template and writes its values onto a page of the licenses naming it.
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
		// Only a template no license names can be deleted, so a vanished
		// template means there is nothing left to propagate onto.
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

// syncOne writes one Organization's license to follow the template, in its
// own transaction, keeping the values of every adjusted field the template
// still declares.
//
// A license already holding the resolved values is left untouched — no write
// and no history entry — which is what makes a re-delivered or re-run job a
// no-op, matching what license.OutcomeUnchanged means for a migration.
//
// The merged set is validated exactly as an adjustment is. A license whose
// adjusted value no longer satisfies a tightened schema is refused whole and
// reported, never partially applied and never silently stripped of its
// adjustment; it keeps the values it holds until an operator resolves it.
//
// The template is read again inside this transaction, not carried from the
// job's first read. Two jobs for the same template can run concurrently on
// two replicas after rapid edits; a job holding the older values could
// otherwise overwrite the newer job's writes with no later job to repair
// them. Read fresh under the row lock, the stale window shrinks to a commit
// racing this transaction — and that commit enqueued its own job, which runs
// after this lock releases and repairs it.
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
			// Migrated onto another template between the listing and this
			// transaction; it follows that template now, not this one.
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
