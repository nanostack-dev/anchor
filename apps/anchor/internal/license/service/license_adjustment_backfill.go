package service

import (
	"context"
	"fmt"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"go.uber.org/fx"

	"anchor/internal/domain/license"
	licenserepo "anchor/internal/license/repository"
)

const adjustmentBackfillBatchSize = 100

// LicenseAdjustmentBackfill initializes legacy adjustment provenance before
// this replica starts accepting requests or processing template sync jobs.
// Row transactions make interrupted startups resumable and replicas safe.
type LicenseAdjustmentBackfill struct {
	licenses  licenserepo.OrganizationLicenseRepository
	templates licenserepo.TemplateRepository
	changes   licenserepo.OrganizationLicenseChangeRepository
	tx        transactor.Transactor
}

func NewLicenseAdjustmentBackfill(
	licenses licenserepo.OrganizationLicenseRepository,
	templates licenserepo.TemplateRepository,
	changes licenserepo.OrganizationLicenseChangeRepository,
	tx transactor.Transactor,
) *LicenseAdjustmentBackfill {
	return &LicenseAdjustmentBackfill{licenses: licenses, templates: templates, changes: changes, tx: tx}
}

func RegisterLicenseAdjustmentBackfill(lifecycle fx.Lifecycle, backfill *LicenseAdjustmentBackfill) {
	lifecycle.Append(fx.Hook{OnStart: backfill.Run})
}

func (b *LicenseAdjustmentBackfill) Run(ctx context.Context) error {
	for {
		rows, err := b.licenses.ListUninitializedAdjustmentsInternal(ctx, adjustmentBackfillBatchSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		// Each iteration commits one row, so a restart retains completed work.
		for _, row := range rows {
			if initErr := b.initialize(ctx, row); initErr != nil {
				return fmt.Errorf("backfill license %s adjustments: %w", row.ID, initErr)
			}
		}
	}
}

func (b *LicenseAdjustmentBackfill) initialize(ctx context.Context, row license.OrganizationLicense) error {
	return b.tx.InTx(ctx, func(txCtx context.Context) error {
		found, err := b.licenses.FindByOrganizationForUpdate(
			txCtx,
			row.PlatformTenantID,
			row.ProductID,
			row.OrganizationID,
		)
		if err != nil || found.IsAbsent() {
			return err
		}
		current := found.Value()
		if current.AdjustedFields != nil {
			return nil
		}
		template, err := b.templates.FindByID(txCtx, current.PlatformTenantID, current.ProductID, current.TemplateID)
		if err != nil {
			return err
		}
		if template.IsAbsent() {
			return ErrLicenseTemplateNotFound
		}
		// Old differences might be bespoke even without complete history. Keep
		// held values conservatively; missing fields must still follow the tier.
		differences := functional.Slice(license.DiffValues(current.Values, template.Value().Values)).
			Filter(func(diff license.FieldDifference) bool { return diff.Kind != license.DifferenceOnlyInTemplate })
		current.RecordAdjustedFields(
			functional.Slice(differences).Map(func(diff license.FieldDifference) string { return diff.Field }),
		)
		if historyErr := b.restoreHistory(txCtx, &current); historyErr != nil {
			return historyErr
		}
		_, err = b.licenses.Update(txCtx, current.PlatformTenantID, current)
		return err
	})
}

func (b *LicenseAdjustmentBackfill) restoreHistory(ctx context.Context, current *license.OrganizationLicense) error {
	for offset := int32(0); ; offset += adjustmentBackfillBatchSize {
		page, err := b.changes.ListByOrganization(ctx, license.ListLicenseChangesInput{
			TenantID: current.PlatformTenantID, ProductID: current.ProductID, OrganizationID: current.OrganizationID,
			Pagination: search.Pagination{Limit: adjustmentBackfillBatchSize, Offset: offset},
		})
		if err != nil {
			return err
		}
		// Newest first; stop at the current copy's provenance boundary. Equal
		// values remain pinned when history proves they were explicitly adjusted.
		for _, change := range page.Items {
			if change.ChangedAt.Before(current.InstantiatedAt) {
				return nil
			}
			if change.LicenseID == current.ID && change.Type == license.ChangeAdjusted && change.Field != nil {
				current.RecordAdjustedFields([]string{*change.Field})
			}
		}
		if len(page.Items) < adjustmentBackfillBatchSize {
			return nil
		}
	}
}
