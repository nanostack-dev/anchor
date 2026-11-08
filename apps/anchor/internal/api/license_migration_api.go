package api

import (
	"context"
	"log/slog"

	"anchor/internal/applog"

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
		OnDifference:    ptr.DerefOr(body.OnDifference, license.CarryForwardDifferences),
	})
	if err != nil {
		applog.Error(ctx, s.logger, err, "failed to migrate organization licenses",
			slog.String("product_id", request.ProductId),
			slog.String("license_template_id", body.TemplateId),
		)
		return nil, err
	}

	return MigrateOrganizationLicenses200JSONResponse(
		mapLicenseMigrationToResponse(migration),
	), nil
}
