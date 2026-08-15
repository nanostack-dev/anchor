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

// LicenseHistoryService reads what OrganizationLicenseService recorded: every
// change ever made to one Organization's license, newest first and paginated.
//
// It reads and never writes. Recording belongs to the write that caused the
// change, so an entry cannot be produced without the license move it describes
// — the same reason UsageService owns the append that UsageSeriesService only
// reads back.
//
// An Organization that has never been licensed reads as an empty history
// rather than as a 404: nothing has happened to it yet, which is a fact rather
// than an absence. A 404 means no such Organization.
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

	organization, err := s.organizations.FindByID(ctx, in.ProductID, in.OrganizationID)
	if err != nil {
		return search.Result[license.OrganizationLicenseChange]{}, err
	}
	if organization == nil {
		return search.Result[license.OrganizationLicenseChange]{}, ErrLicenseOrganizationNotFound
	}

	return s.changes.ListByOrganization(ctx, in)
}
