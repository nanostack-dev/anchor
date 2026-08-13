package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/license"
	"anchor/internal/mapper"
)

var _ UsageObservationRepository = (*usageObservationRepositoryImpl)(nil)

type usageObservationRepositoryImpl struct {
	db     *sql.DB
	mapper *mapper.UsageObservationMapper
	logger zerolog.Logger
}

func NewUsageObservationRepository(
	db *sql.DB, m *mapper.UsageObservationMapper, logger zerolog.Logger,
) UsageObservationRepository {
	return &usageObservationRepositoryImpl{
		db:     db,
		mapper: m,
		logger: logger.With().Str("component", "usage_observation_repository").Logger(),
	}
}

func (r *usageObservationRepositoryImpl) Append(
	ctx context.Context, observation license.UsageObservation,
) (license.UsageObservation, error) {
	if observation.ObservedAt.IsZero() {
		observation.ObservedAt = time.Now()
	}
	entity := r.mapper.ToEntity(observation)
	stmt := table.UsageObservations.INSERT(table.UsageObservations.AllColumns).
		MODEL(entity).
		RETURNING(table.UsageObservations.AllColumns)
	return transactor.QueryMap(ctx, r.db, stmt, r.mapper.ToDomain).Value()
}

func (r *usageObservationRepositoryImpl) LatestPerKey(
	ctx context.Context, tenantID string, productID string, organizationID string,
) ([]license.UsageObservation, error) {
	// DISTINCT ON (key), ordered by key then observed_at desc, is Postgres's
	// idiom for "the latest row per key" in one pass — no aggregation and no
	// self-join. See ADR-0005: this is the same last(value, observed_at)
	// semantics the continuous aggregates use, expressed for a raw read.
	stmt := table.UsageObservations.
		SELECT(table.UsageObservations.AllColumns).
		DISTINCT(table.UsageObservations.Key).
		FROM(table.UsageObservations).
		WHERE(
			table.UsageObservations.PlatformTenantID.EQ(postgres.String(tenantID)).
				AND(table.UsageObservations.ProductID.EQ(postgres.String(productID))).
				AND(table.UsageObservations.OrganizationID.EQ(postgres.String(organizationID))),
		).
		ORDER_BY(table.UsageObservations.Key.ASC(), table.UsageObservations.ObservedAt.DESC())
	return transactor.QueryMapSlice(ctx, r.db, stmt, r.mapper.ToDomain).Value()
}
