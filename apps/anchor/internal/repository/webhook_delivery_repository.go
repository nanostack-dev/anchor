package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/webhook"
	"anchor/internal/mapper"
)

var _ WebhookDeliveryRepository = (*webhookDeliveryRepositoryImpl)(nil)

func webhookDeliveriesInsertableColumns() postgres.ColumnList {
	return table.WebhookDeliveries.AllColumns.Except(
		table.WebhookDeliveries.CreatedAt, table.WebhookDeliveries.UpdatedAt,
	)
}

func webhookDeliveryAttemptsInsertableColumns() postgres.ColumnList {
	return table.WebhookDeliveryAttempts.AllColumns.Except(
		table.WebhookDeliveryAttempts.CreatedAt,
	)
}

// DeliveryOutcome is the delivery-row state written after one attempt.
type DeliveryOutcome struct {
	Status         webhook.DeliveryStatus
	AttemptCount   int32
	LastStatusCode *int32
	LastError      *string
	CompletedAt    *time.Time
}

type WebhookDeliveryRepository interface {
	// ListByEndpoint returns the product-scoped delivery log of an endpoint,
	// newest first, joined to the event that produced each delivery.
	ListByEndpoint(
		ctx context.Context, input webhook.ListDeliveriesInput,
	) ([]webhook.DeliveryWithEvent, error)
	// FindByIDForEndpoint loads one delivery under its product and endpoint.
	FindByIDForEndpoint(
		ctx context.Context, productID string, endpointID string, deliveryID string,
	) (*webhook.Delivery, error)
	ListAttempts(ctx context.Context, deliveryID string) ([]webhook.Attempt, error)

	// CreateInternal inserts a delivery row.
	//
	// Product-scope bypass: written by the fan-out worker and by the manual
	// retry path, both of which carry a product id resolved upstream.
	CreateInternal(ctx context.Context, delivery webhook.Delivery) (webhook.Delivery, error)

	// FindOriginalByEventAndEndpointInternal looks up the non-replay delivery
	// of an (event, endpoint) pair. It is what makes fan-out idempotent: a
	// re-run after a crash finds the existing row instead of double-delivering.
	//
	// Product-scope bypass: called by the fan-out worker.
	FindOriginalByEventAndEndpointInternal(
		ctx context.Context, eventID string, endpointID string,
	) (*webhook.Delivery, error)

	// FindByIDInternal loads a delivery without a product filter.
	//
	// Product-scope bypass: the delivery worker processes jobs for every
	// product. Never call this from a tenant-facing handler.
	FindByIDInternal(ctx context.Context, deliveryID string) (*webhook.Delivery, error)

	// UpdateOutcomeInternal writes the post-attempt state of a delivery.
	//
	// Product-scope bypass: written by the delivery worker.
	UpdateOutcomeInternal(
		ctx context.Context, deliveryID string, outcome DeliveryOutcome,
	) error

	// CreateAttemptInternal appends an immutable attempt record.
	//
	// Product-scope bypass: written by the delivery worker.
	CreateAttemptInternal(ctx context.Context, attempt webhook.Attempt) (webhook.Attempt, error)
}

type webhookDeliveryRepositoryImpl struct {
	db             *sql.DB
	deliveryMapper *mapper.WebhookDeliveryMapper
	attemptMapper  *mapper.WebhookDeliveryAttemptMapper
	eventMapper    *mapper.WebhookEventMapper
	logger         zerolog.Logger
}

func NewWebhookDeliveryRepository(
	db *sql.DB,
	deliveryMapper *mapper.WebhookDeliveryMapper,
	attemptMapper *mapper.WebhookDeliveryAttemptMapper,
	eventMapper *mapper.WebhookEventMapper,
	logger zerolog.Logger,
) WebhookDeliveryRepository {
	return &webhookDeliveryRepositoryImpl{
		db:             db,
		deliveryMapper: deliveryMapper,
		attemptMapper:  attemptMapper,
		eventMapper:    eventMapper,
		logger:         logger.With().Str("component", "webhook_delivery_repository").Logger(),
	}
}

// deliveryWithEventRow is the join projection backing the delivery log.
type deliveryWithEventRow struct {
	model.WebhookDeliveries
	model.WebhookEvents
}

func (r *webhookDeliveryRepositoryImpl) ListByEndpoint(
	ctx context.Context, input webhook.ListDeliveriesInput,
) ([]webhook.DeliveryWithEvent, error) {
	condition := table.WebhookDeliveries.ProductID.EQ(postgres.String(input.ProductID)).AND(
		table.WebhookDeliveries.EndpointID.EQ(postgres.String(input.EndpointID)),
	)
	if input.Status != nil {
		condition = condition.AND(
			table.WebhookDeliveries.Status.EQ(postgres.String(string(*input.Status))),
		)
	}
	if input.EventType != nil {
		condition = condition.AND(
			table.WebhookEvents.EventType.EQ(postgres.String(*input.EventType)),
		)
	}

	stmt := postgres.SELECT(
		table.WebhookDeliveries.AllColumns,
		table.WebhookEvents.AllColumns,
	).FROM(
		table.WebhookDeliveries.INNER_JOIN(
			table.WebhookEvents,
			table.WebhookDeliveries.EventID.EQ(table.WebhookEvents.ID),
		),
	).WHERE(condition).ORDER_BY(
		table.WebhookDeliveries.CreatedAt.DESC(),
	).LIMIT(int64(input.NormalizedLimit())).OFFSET(int64(input.Offset))

	return transactor.QueryMapSlice(
		ctx, r.db, stmt, func(row deliveryWithEventRow) webhook.DeliveryWithEvent {
			return webhook.DeliveryWithEvent{
				Delivery: r.deliveryMapper.ToDomain(row.WebhookDeliveries),
				Event:    r.eventMapper.ToDomain(row.WebhookEvents),
			}
		},
	)
}

func (r *webhookDeliveryRepositoryImpl) FindByIDForEndpoint(
	ctx context.Context, productID string, endpointID string, deliveryID string,
) (*webhook.Delivery, error) {
	stmt := table.WebhookDeliveries.SELECT(table.WebhookDeliveries.AllColumns).WHERE(
		table.WebhookDeliveries.ID.EQ(postgres.String(deliveryID)).AND(
			table.WebhookDeliveries.ProductID.EQ(postgres.String(productID)),
		).AND(
			table.WebhookDeliveries.EndpointID.EQ(postgres.String(endpointID)),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.deliveryMapper.ToDomain)
}

func (r *webhookDeliveryRepositoryImpl) ListAttempts(
	ctx context.Context, deliveryID string,
) ([]webhook.Attempt, error) {
	stmt := table.WebhookDeliveryAttempts.SELECT(
		table.WebhookDeliveryAttempts.AllColumns,
	).WHERE(
		table.WebhookDeliveryAttempts.DeliveryID.EQ(postgres.String(deliveryID)),
	).ORDER_BY(table.WebhookDeliveryAttempts.AttemptNumber.ASC())

	return transactor.QueryMapSlice(ctx, r.db, stmt, r.attemptMapper.ToDomain)
}

func (r *webhookDeliveryRepositoryImpl) CreateInternal(
	ctx context.Context, delivery webhook.Delivery,
) (webhook.Delivery, error) {
	entity := r.deliveryMapper.ToEntity(delivery)

	stmt := table.WebhookDeliveries.INSERT(
		webhookDeliveriesInsertableColumns(),
	).MODEL(entity).RETURNING(table.WebhookDeliveries.AllColumns)

	created, err := transactor.Query[model.WebhookDeliveries](ctx, r.db, stmt)
	if err != nil {
		return webhook.Delivery{}, err
	}

	return r.deliveryMapper.ToDomain(created), nil
}

func (r *webhookDeliveryRepositoryImpl) FindOriginalByEventAndEndpointInternal(
	ctx context.Context, eventID string, endpointID string,
) (*webhook.Delivery, error) {
	stmt := table.WebhookDeliveries.SELECT(table.WebhookDeliveries.AllColumns).WHERE(
		table.WebhookDeliveries.EventID.EQ(postgres.String(eventID)).AND(
			table.WebhookDeliveries.EndpointID.EQ(postgres.String(endpointID)),
		).AND(
			table.WebhookDeliveries.ReplayOfDeliveryID.IS_NULL(),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.deliveryMapper.ToDomain)
}

func (r *webhookDeliveryRepositoryImpl) FindByIDInternal(
	ctx context.Context, deliveryID string,
) (*webhook.Delivery, error) {
	stmt := table.WebhookDeliveries.SELECT(table.WebhookDeliveries.AllColumns).WHERE(
		table.WebhookDeliveries.ID.EQ(postgres.String(deliveryID)),
	).LIMIT(1)

	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.deliveryMapper.ToDomain)
}

func (r *webhookDeliveryRepositoryImpl) UpdateOutcomeInternal(
	ctx context.Context, deliveryID string, outcome DeliveryOutcome,
) error {
	entity := model.WebhookDeliveries{
		Status:         string(outcome.Status),
		AttemptCount:   outcome.AttemptCount,
		LastStatusCode: outcome.LastStatusCode,
		LastError:      outcome.LastError,
		CompletedAt:    outcome.CompletedAt,
		UpdatedAt:      time.Now(),
	}

	stmt := table.WebhookDeliveries.UPDATE(
		postgres.ColumnList{
			table.WebhookDeliveries.Status,
			table.WebhookDeliveries.AttemptCount,
			table.WebhookDeliveries.LastStatusCode,
			table.WebhookDeliveries.LastError,
			table.WebhookDeliveries.CompletedAt,
			table.WebhookDeliveries.UpdatedAt,
		},
	).MODEL(entity).WHERE(
		table.WebhookDeliveries.ID.EQ(postgres.String(deliveryID)),
	)

	return transactor.Exec(ctx, r.db, stmt)
}

func (r *webhookDeliveryRepositoryImpl) CreateAttemptInternal(
	ctx context.Context, attempt webhook.Attempt,
) (webhook.Attempt, error) {
	entity := r.attemptMapper.ToEntity(attempt)

	stmt := table.WebhookDeliveryAttempts.INSERT(
		webhookDeliveryAttemptsInsertableColumns(),
	).MODEL(entity).RETURNING(table.WebhookDeliveryAttempts.AllColumns)

	created, err := transactor.Query[model.WebhookDeliveryAttempts](ctx, r.db, stmt)
	if err != nil {
		return webhook.Attempt{}, err
	}

	return r.attemptMapper.ToDomain(created), nil
}
