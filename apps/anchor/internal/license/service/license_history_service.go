package service

import (
	"context"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/license"
	"anchor/internal/domain/organization"
	licenserepo "anchor/internal/license/repository"
	intrepo "anchor/internal/repository"
)

// LicenseHistoryService searches one Organization's license history. An
// Organization that was never licensed returns an empty page. A 404 means no
// such Organization.
type LicenseHistoryService interface {
	SearchChanges(
		ctx context.Context, in license.SearchLicenseChangesInput,
	) (search.Result[license.OrganizationLicenseChange], error)
}

type licenseHistoryService struct {
	changes       licenserepo.OrganizationLicenseChangeRepository
	organizations intrepo.OrganizationRepository
	logger        *slog.Logger
}

func NewLicenseHistoryService(
	changes licenserepo.OrganizationLicenseChangeRepository,
	organizations intrepo.OrganizationRepository,
	logger *slog.Logger,
) LicenseHistoryService {
	return &licenseHistoryService{
		changes:       changes,
		organizations: organizations,
		logger:        logger.With(slog.String("component", "license_history_service")),
	}
}

func (s *licenseHistoryService) SearchChanges(
	ctx context.Context, in license.SearchLicenseChangesInput,
) (search.Result[license.OrganizationLicenseChange], error) {
	if err := in.Validate(); err != nil {
		return search.Result[license.OrganizationLicenseChange]{}, err
	}

	found, err := s.organizations.FindByProductIDAndID(ctx, in.ProductID, in.OrganizationID)
	if err != nil {
		return search.Result[license.OrganizationLicenseChange]{}, err
	}
	if found.IsAbsent() {
		return search.Result[license.OrganizationLicenseChange]{}, organization.ErrOrganizationNotFound
	}

	return s.changes.Search(ctx, in)
}
