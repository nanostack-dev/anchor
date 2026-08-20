package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/integration"
	"anchor/internal/mapper"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"
)

var _ IntegrationEventRepository = (*integrationEventRepositoryImpl)(nil)

func integrationEventsUpdatableColumns() postgres.ColumnList {
	return table.IntegrationEvents.AllColumns.Except(
		table.IntegrationEvents.CreatedAt, table.IntegrationEvents.UpdatedAt,
	)
}

type IntegrationEventRepository interface {
	// CreateInternal inserts a new integration event. Reserved for trusted
	// system-internal paths (webhook ingress) where no authenticated tenant
	// context exists. Must NOT be called from tenant-facing API handlers.
	CreateInternal(
		ctx context.Context, event integration.Event,
	) (integration.Event, error)
	// FindByIDInternal looks up an event by its globally-unique ID without
	// tenant scoping. Reserved for trusted system-internal paths (e.g. async
	// queue workers) where no authenticated tenant context exists. Must NOT
	// be called from tenant-facing API handlers.
	FindByIDInternal(
		ctx context.Context, id string,
	) (*integration.Event, error)
	// FindByExternalEventIDInternal looks up an event by instance ID and
	// external event ID without tenant scoping. Reserved for trusted
	// system-internal paths (webhook ingress deduplication) where no
	// authenticated tenant context exists. Must NOT be called from
	// tenant-facing API handlers.
	FindByExternalEventIDInternal(
		ctx context.Context, instanceID string, externalEventID string,
	) (*integration.Event, error)
	// UpdateStatusInternal updates event status by ID without tenant scoping.
	// Reserved for trusted system-internal paths (async queue workers, webhook
	// ingress) where no authenticated tenant context exists. Must NOT be
	// called from tenant-facing API handlers.
	UpdateStatusInternal(
		ctx context.Context, id string, status integration.EventStatus, errMsg *string,
	) error
}

type integrationEventRepositoryImpl struct {
	db     *sql.DB
	mapper *mapper.IntegrationEventMapper
	logger zerolog.Logger
}

func NewIntegrationEventRepository(
	db *sql.DB, m *mapper.IntegrationEventMapper, logger zerolog.Logger,
) IntegrationEventRepository {
	return &integrationEventRepositoryImpl{
		db:     db,
		mapper: m,
		logger: logger.With().Str("component", "integration_event_repository").Logger(),
	}
}

func (r *integrationEventRepositoryImpl) CreateInternal(
	ctx context.Context, event integration.Event,
) (integration.Event, error) {
	entity := r.mapper.ToEntity(event)

	stmt := table.IntegrationEvents.INSERT(
		integrationEventsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.IntegrationEvents.AllColumns)

	return transactor.QueryMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	).Value()
}

func (r *integrationEventRepositoryImpl) FindByIDInternal(
	ctx context.Context, id string,
) (*integration.Event, error) {
	stmt := table.IntegrationEvents.SELECT(
		table.IntegrationEvents.AllColumns,
	).FROM(
		table.IntegrationEvents,
	).WHERE(
		table.IntegrationEvents.ID.EQ(postgres.String(id)),
	).LIMIT(1)

	result := transactor.QueryOptionalMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
	if err := result.Err(); err != nil {
		return nil, err
	}
	return result.ToPtr(), nil
}

func (r *integrationEventRepositoryImpl) FindByExternalEventIDInternal(
	ctx context.Context, instanceID string, externalEventID string,
) (*integration.Event, error) {
	stmt := table.IntegrationEvents.SELECT(
		table.IntegrationEvents.AllColumns,
	).FROM(
		table.IntegrationEvents,
	).WHERE(
		table.IntegrationEvents.IntegrationInstanceID.EQ(postgres.String(instanceID)).AND(
			table.IntegrationEvents.ExternalEventID.EQ(postgres.String(externalEventID)),
		),
	).LIMIT(1)

	result := transactor.QueryOptionalMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
	if err := result.Err(); err != nil {
		return nil, err
	}
	return result.ToPtr(), nil
}

func (r *integrationEventRepositoryImpl) UpdateStatusInternal(
	ctx context.Context, id string, status integration.EventStatus, errMsg *string,
) error {
	now := time.Now()

	// Build a partial entity for the columns we want to update.
	entity := model.IntegrationEvents{
		Status:    string(status),
		UpdatedAt: now,
	}

	// Always update status + updated_at.
	columns := postgres.ColumnList{
		table.IntegrationEvents.Status,
		table.IntegrationEvents.UpdatedAt,
	}

	if status == integration.EventStatusProcessed || status == integration.EventStatusFailed {
		entity.ProcessedAt = &now
		columns = append(columns, table.IntegrationEvents.ProcessedAt)
	}

	if errMsg != nil {
		entity.Error = errMsg
		columns = append(columns, table.IntegrationEvents.Error)
	}

	stmt := table.IntegrationEvents.UPDATE(columns).MODEL(entity).WHERE(
		table.IntegrationEvents.ID.EQ(postgres.String(id)),
	)

	return transactor.Exec(ctx, r.db, stmt).Err()
}
