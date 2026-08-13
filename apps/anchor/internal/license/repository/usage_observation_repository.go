package repository

import (
	"context"
	"database/sql"
	"time"

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
