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

var _ WebhookEndpointRepository = (*webhookEndpointRepositoryImpl)(nil)

func webhookEndpointsUpdatableColumns() postgres.ColumnList {
	return table.WebhookEndpoints.AllColumns.Except(
		table.WebhookEndpoints.CreatedAt, table.WebhookEndpoints.UpdatedAt,
	)
}

type WebhookEndpointRepository interface {
	ListByProduct(ctx context.Context, productID string) ([]webhook.Endpoint, error)
	FindByID(
		ctx context.Context, productID string, endpointID string,
	) (*webhook.Endpoint, error)
	Create(ctx context.Context, endpoint webhook.Endpoint) (webhook.Endpoint, error)
	Update(
		ctx context.Context, productID string, endpoint webhook.Endpoint,
	) (webhook.Endpoint, error)
	DeleteByID(ctx context.Context, productID string, endpointID string) error

	// FindByIDInternal loads an endpoint without a product filter.
	//
	// Product-scope bypass: the delivery and fan-out workers process jobs for
	// every product on the instance and have no tenant context of their own.
	// Never call this from a tenant-facing handler; use FindByID there.
	FindByIDInternal(ctx context.Context, endpointID string) (*webhook.Endpoint, error)

	// ListEnabledByProductInternal loads a product's ENABLED endpoints without
	// requiring a caller-supplied product scope.
	//
	// Product-scope bypass: called by the fan-out worker with the product id
	// read off the event row. Never call this from a tenant-facing handler.
	ListEnabledByProductInternal(
		ctx context.Context, productID string,
	) ([]webhook.Endpoint, error)

	// UpdateHealthInternal writes the failure/success counters and status of an
	// endpoint after a delivery attempt.
	//
	// Product-scope bypass: written by the delivery worker, which is not
	// tenant-scoped. Never call this from a tenant-facing handler.
	UpdateHealthInternal(
		ctx context.Context,
		endpointID string,
		counters webhook.FailureCounters,
		status webhook.EndpointStatus,
		disabledReason string,
	) error
}

type webhookEndpointRepositoryImpl struct {
	db             *sql.DB
	endpointMapper *mapper.WebhookEndpointMapper
	logger         zerolog.Logger
}

func NewWebhookEndpointRepository(
	db *sql.DB, endpointMapper *mapper.WebhookEndpointMapper, logger zerolog.Logger,
) WebhookEndpointRepository {
	return &webhookEndpointRepositoryImpl{
		db:             db,
		endpointMapper: endpointMapper,
		logger:         logger.With().Str("component", "webhook_endpoint_repository").Logger(),
	}
}

func (r *webhookEndpointRepositoryImpl) ListByProduct(
	ctx context.Context, productID string,
) ([]webhook.Endpoint, error) {
	stmt := table.WebhookEndpoints.SELECT(table.WebhookEndpoints.AllColumns).WHERE(
		table.WebhookEndpoints.ProductID.EQ(postgres.String(productID)),
	).ORDER_BY(table.WebhookEndpoints.CreatedAt.ASC())

	return transactor.QueryMapSlice(ctx, r.db, stmt, r.endpointMapper.ToDomain)
}

func (r *webhookEndpointRepositoryImpl) FindByID(
	ctx context.Context, productID string, endpointID string,
) (*webhook.Endpoint, error) {
	stmt := table.WebhookEndpoints.SELECT(table.WebhookEndpoints.AllColumns).WHERE(
		table.WebhookEndpoints.ID.EQ(postgres.String(endpointID)).AND(
			table.WebhookEndpoints.ProductID.EQ(postgres.String(productID)),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.endpointMapper.ToDomain)
}

func (r *webhookEndpointRepositoryImpl) FindByIDInternal(
	ctx context.Context, endpointID string,
) (*webhook.Endpoint, error) {
	stmt := table.WebhookEndpoints.SELECT(table.WebhookEndpoints.AllColumns).WHERE(
		table.WebhookEndpoints.ID.EQ(postgres.String(endpointID)),
	).LIMIT(1)

	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.endpointMapper.ToDomain)
}

func (r *webhookEndpointRepositoryImpl) ListEnabledByProductInternal(
	ctx context.Context, productID string,
) ([]webhook.Endpoint, error) {
	stmt := table.WebhookEndpoints.SELECT(table.WebhookEndpoints.AllColumns).WHERE(
		table.WebhookEndpoints.ProductID.EQ(postgres.String(productID)).AND(
			table.WebhookEndpoints.Status.EQ(
				postgres.String(string(webhook.EndpointStatusEnabled)),
			),
		),
	).ORDER_BY(table.WebhookEndpoints.CreatedAt.ASC())

	return transactor.QueryMapSlice(ctx, r.db, stmt, r.endpointMapper.ToDomain)
}

func (r *webhookEndpointRepositoryImpl) Create(
	ctx context.Context, endpoint webhook.Endpoint,
) (webhook.Endpoint, error) {
	entity := r.endpointMapper.ToEntity(endpoint)

	stmt := table.WebhookEndpoints.INSERT(
		webhookEndpointsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.WebhookEndpoints.AllColumns)

	created, err := transactor.Query[model.WebhookEndpoints](ctx, r.db, stmt)
	if err != nil {
		return webhook.Endpoint{}, err
	}

	return r.endpointMapper.ToDomain(created), nil
}

func (r *webhookEndpointRepositoryImpl) Update(
	ctx context.Context, productID string, endpoint webhook.Endpoint,
) (webhook.Endpoint, error) {
	endpoint.UpdatedAt = time.Now()

	// Identity columns stay out of the SET list so a product-scoped update can
	// never move an endpoint to another product.
	columns := webhookEndpointsUpdatableColumns().Except(
		table.WebhookEndpoints.ID, table.WebhookEndpoints.ProductID,
	)
	scope := table.WebhookEndpoints.ID.EQ(postgres.String(endpoint.ID)).AND(
		table.WebhookEndpoints.ProductID.EQ(postgres.String(productID)),
	)

	stmt := table.WebhookEndpoints.
		UPDATE(columns).
		MODEL(r.endpointMapper.ToEntity(endpoint)).
		WHERE(scope).
		RETURNING(table.WebhookEndpoints.AllColumns)

	updated, err := transactor.Query[model.WebhookEndpoints](ctx, r.db, stmt)
	if err != nil {
		return webhook.Endpoint{}, err
	}

	return r.endpointMapper.ToDomain(updated), nil
}

func (r *webhookEndpointRepositoryImpl) UpdateHealthInternal(
	ctx context.Context,
	endpointID string,
	counters webhook.FailureCounters,
	status webhook.EndpointStatus,
	disabledReason string,
) error {
	var reason *string
	if disabledReason != "" {
		reason = &disabledReason
	}

	entity := model.WebhookEndpoints{
		ConsecutiveFailureCount: counters.ConsecutiveFailureCount,
		FirstFailureAt:          counters.FirstFailureAt,
		LastFailureAt:           counters.LastFailureAt,
		LastSuccessAt:           counters.LastSuccessAt,
		Status:                  string(status),
		DisabledReason:          reason,
		UpdatedAt:               time.Now(),
	}

	stmt := table.WebhookEndpoints.UPDATE(
		postgres.ColumnList{
			table.WebhookEndpoints.ConsecutiveFailureCount,
			table.WebhookEndpoints.FirstFailureAt,
			table.WebhookEndpoints.LastFailureAt,
			table.WebhookEndpoints.LastSuccessAt,
			table.WebhookEndpoints.Status,
			table.WebhookEndpoints.DisabledReason,
			table.WebhookEndpoints.UpdatedAt,
		},
	).MODEL(entity).WHERE(
		table.WebhookEndpoints.ID.EQ(postgres.String(endpointID)),
	)

	return transactor.Exec(ctx, r.db, stmt)
}

func (r *webhookEndpointRepositoryImpl) DeleteByID(
	ctx context.Context, productID string, endpointID string,
) error {
	stmt := table.WebhookEndpoints.DELETE().WHERE(
		table.WebhookEndpoints.ID.EQ(postgres.String(endpointID)).AND(
			table.WebhookEndpoints.ProductID.EQ(postgres.String(productID)),
		),
	)

	return transactor.Exec(ctx, r.db, stmt)
}
