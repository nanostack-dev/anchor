package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/ptr"

	"anchor/internal/domain/license"
	"anchor/internal/security"
)

func (s *AnchorAPI) MigrateOrganizationLicenses(
	ctx context.Context, request MigrateOrganizationLicensesRequestObject,
) (MigrateOrganizationLicensesResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	body := request.Body
	migration, err := s.LicenseMigrationService.Migrate(ctx, license.MigrateLicensesInput{
		TenantID:        tenantID,
		ProductID:       request.ProductId,
		TemplateID:      body.TemplateId,
		OrganizationIDs: ptr.DerefOr(body.OrganizationIds, nil),
		FromTemplateID:  ptr.DerefOr(body.FromTemplateId, ""),
		OnDifference:    ptr.DerefOr(body.OnDifference, license.DifferenceSkip),
		DryRun:          ptr.DerefOr(body.DryRun, false),
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("license_template_id", body.TemplateId).
			Msg("failed to migrate organization licenses")
		return nil, err
	}

	return MigrateOrganizationLicenses200JSONResponse(
		mapLicenseMigrationToResponse(migration),
	), nil
}
