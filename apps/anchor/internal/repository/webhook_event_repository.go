package repository

import (
	"context"
	"database/sql"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/webhook"
	"anchor/internal/mapper"
)

var _ WebhookEventRepository = (*webhookEventRepositoryImpl)(nil)

func webhookEventsInsertableColumns() postgres.ColumnList {
	return table.WebhookEvents.AllColumns.Except(table.WebhookEvents.CreatedAt)
}

type WebhookEventRepository interface {
	// CreateInternal writes an outbox row. It is Internal because the emitter
	// is a cross-feature seam invoked from inside other services' transactions
	// with a product id they have already scoped; there is no tenant-facing
	// handler that writes an event directly.
	CreateInternal(ctx context.Context, event webhook.Event) (webhook.Event, error)

	// FindByIDInternal loads an event without a product filter.
	//
	// Product-scope bypass: the fan-out worker processes events for every
	// product on the instance. Never call this from a tenant-facing handler.
	FindByIDInternal(ctx context.Context, eventID string) (*webhook.Event, error)

	// FindByIDForProduct loads an event under a product scope, for the
	// tenant-facing delivery log.
	FindByIDForProduct(
		ctx context.Context, productID string, eventID string,
	) (*webhook.Event, error)
}

type webhookEventRepositoryImpl struct {
	db          *sql.DB
	eventMapper *mapper.WebhookEventMapper
	logger      zerolog.Logger
}

func NewWebhookEventRepository(
	db *sql.DB, eventMapper *mapper.WebhookEventMapper, logger zerolog.Logger,
) WebhookEventRepository {
	return &webhookEventRepositoryImpl{
		db:          db,
		eventMapper: eventMapper,
		logger:      logger.With().Str("component", "webhook_event_repository").Logger(),
	}
}

func (r *webhookEventRepositoryImpl) CreateInternal(
	ctx context.Context, event webhook.Event,
) (webhook.Event, error) {
	entity := r.eventMapper.ToEntity(event)

	stmt := table.WebhookEvents.INSERT(
		webhookEventsInsertableColumns(),
	).MODEL(entity).RETURNING(table.WebhookEvents.AllColumns)

	created, err := transactor.Query[model.WebhookEvents](ctx, r.db, stmt)
	if err != nil {
		return webhook.Event{}, err
	}

	return r.eventMapper.ToDomain(created), nil
}

func (r *webhookEventRepositoryImpl) FindByIDInternal(
	ctx context.Context, eventID string,
) (*webhook.Event, error) {
	stmt := table.WebhookEvents.SELECT(table.WebhookEvents.AllColumns).WHERE(
		table.WebhookEvents.ID.EQ(postgres.String(eventID)),
	).LIMIT(1)

	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.eventMapper.ToDomain)
}

func (r *webhookEventRepositoryImpl) FindByIDForProduct(
	ctx context.Context, productID string, eventID string,
) (*webhook.Event, error) {
	stmt := table.WebhookEvents.SELECT(table.WebhookEvents.AllColumns).WHERE(
		table.WebhookEvents.ID.EQ(postgres.String(eventID)).AND(
			table.WebhookEvents.ProductID.EQ(postgres.String(productID)),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.eventMapper.ToDomain)
}
