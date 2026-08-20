package service

import (
	"context"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/license"
	licenserepo "anchor/internal/license/repository"
	intrepo "anchor/internal/repository"
)

// UsageSeriesService reads what UsageService has recorded, shaped into a
// bounded, paginated series. Like UsageService it holds no evaluator:
// interpreting a series against a limit is a different question than
// returning what was reported, and answering it is not this service's job —
// see docs/adr/0005-timescaledb-for-usage-history.md.
//
// This is deliberately a separate read from OrganizationLicenseService.
// GetLicense: a large paginated series and a small hot cacheable read want
// different shapes, so this one is not cached, and reading it never touches
// the license state read's cache.
type UsageSeriesService interface {
	GetSeries(
		ctx context.Context, in license.GetUsageSeriesInput,
	) (search.Result[license.UsageSeriesPoint], error)
}

type usageSeriesService struct {
	series        licenserepo.UsageSeriesRepository
	organizations intrepo.OrganizationRepository
	logger        zerolog.Logger
}

func NewUsageSeriesService(
	series licenserepo.UsageSeriesRepository,
	organizations intrepo.OrganizationRepository,
	logger zerolog.Logger,
) UsageSeriesService {
	return &usageSeriesService{
		series:        series,
		organizations: organizations,
		logger:        logger.With().Str("component", "usage_series_service").Logger(),
	}
}

func (s *usageSeriesService) GetSeries(
	ctx context.Context, in license.GetUsageSeriesInput,
) (search.Result[license.UsageSeriesPoint], error) {
	// One reading of the clock for the whole request, so a defaulted To is an
	// exact value in a test, same reasoning as ReportUsage's own now().
	request := in.WithDefaults(time.Now())

	if err := validate.ValidateStruct(request); err != nil {
		return search.Result[license.UsageSeriesPoint]{}, err
	}

	found := s.organizations.FindByID(ctx, request.ProductID, request.OrganizationID)
	if err := found.Err(); err != nil {
		return search.Result[license.UsageSeriesPoint]{}, err
	}
	if !found.IsPresent() {
		return search.Result[license.UsageSeriesPoint]{}, ErrLicenseOrganizationNotFound
	}

	return s.series.Read(ctx, request)
}
