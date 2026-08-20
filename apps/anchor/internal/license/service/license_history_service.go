package service

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/license"
	licenserepo "anchor/internal/license/repository"
	intrepo "anchor/internal/repository"
)

// LicenseHistoryService reads one Organization's license history, newest first.
// An Organization that was never licensed returns an empty page. A 404 means
// no such Organization.
type LicenseHistoryService interface {
	ListChanges(
		ctx context.Context, in license.ListLicenseChangesInput,
	) (search.Result[license.OrganizationLicenseChange], error)
}

type licenseHistoryService struct {
	changes       licenserepo.OrganizationLicenseChangeRepository
	organizations intrepo.OrganizationRepository
	logger        zerolog.Logger
}

func NewLicenseHistoryService(
	changes licenserepo.OrganizationLicenseChangeRepository,
	organizations intrepo.OrganizationRepository,
	logger zerolog.Logger,
) LicenseHistoryService {
	return &licenseHistoryService{
		changes:       changes,
		organizations: organizations,
		logger:        logger.With().Str("component", "license_history_service").Logger(),
	}
}

func (s *licenseHistoryService) ListChanges(
	ctx context.Context, in license.ListLicenseChangesInput,
) (search.Result[license.OrganizationLicenseChange], error) {
	if err := validate.ValidateStruct(in); err != nil {
		return search.Result[license.OrganizationLicenseChange]{}, err
	}

	found := s.organizations.FindByID(ctx, in.ProductID, in.OrganizationID)
	if err := found.Err(); err != nil {
		return search.Result[license.OrganizationLicenseChange]{}, err
	}
	if !found.IsPresent() {
		return search.Result[license.OrganizationLicenseChange]{}, ErrLicenseOrganizationNotFound
	}

	return s.changes.ListByOrganization(ctx, in)
}
