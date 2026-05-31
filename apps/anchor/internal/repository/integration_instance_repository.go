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

var _ IntegrationInstanceRepository = (*integrationInstanceRepositoryImpl)(nil)

func integrationInstancesUpdatableColumns() postgres.ColumnList {
	return table.IntegrationInstances.AllColumns.Except(
		table.IntegrationInstances.CreatedAt, table.IntegrationInstances.UpdatedAt,
	)
}

type IntegrationInstanceRepository interface {
	FindByID(
		ctx context.Context, tenantID string, id string,
	) (*integration.Instance, error)
	// FindByIDInternal looks up an instance by its globally-unique ID without
	// tenant scoping. Reserved for trusted system-internal paths (e.g. async
	// queue workers, webhook ingress) where no authenticated tenant context
	// exists. Must NOT be used from tenant-facing API handlers.
	FindByIDInternal(
		ctx context.Context, id string,
	) (*integration.Instance, error)
	FindByProductAndProvider(
		ctx context.Context, tenantID string, productID string, providerType string,
	) (*integration.Instance, error)
	// FindByProductAndProviderInternal looks up an instance by product_id and
	// provider_type without tenant scoping. Reserved for trusted system-internal
	// paths (e.g. webhook ingress) where no authenticated tenant context exists.
	// Must NOT be used from tenant-facing API handlers.
	FindByProductAndProviderInternal(
		ctx context.Context, productID string, providerType string,
	) (*integration.Instance, error)
	ListByProduct(
		ctx context.Context, tenantID string, productID string,
	) ([]integration.Instance, error)
	// ListByProviderInternal lists instances by provider_type without tenant scoping.
	// Reserved for trusted system-internal paths such as workers and schedulers.
	ListByProviderInternal(
		ctx context.Context, providerType string,
	) ([]integration.Instance, error)
	Create(
		ctx context.Context, instance integration.Instance,
	) (integration.Instance, error)
	Update(
		ctx context.Context, tenantID string, instance integration.Instance,
	) (integration.Instance, error)
	DeleteByID(
		ctx context.Context, tenantID string, id string,
	) error
}

type integrationInstanceRepositoryImpl struct {
	db     *sql.DB
	mapper *mapper.IntegrationInstanceMapper
	logger zerolog.Logger
}

func NewIntegrationInstanceRepository(
	db *sql.DB, m *mapper.IntegrationInstanceMapper, logger zerolog.Logger,
) IntegrationInstanceRepository {
	return &integrationInstanceRepositoryImpl{
		db:     db,
		mapper: m,
		logger: logger.With().Str("component", "integration_instance_repository").Logger(),
	}
}

func (r *integrationInstanceRepositoryImpl) FindByID(
	ctx context.Context, tenantID string, id string,
) (*integration.Instance, error) {
	stmt := table.IntegrationInstances.SELECT(
		table.IntegrationInstances.AllColumns,
	).FROM(
		table.IntegrationInstances,
	).WHERE(
		table.IntegrationInstances.ID.EQ(postgres.String(id)).AND(
			table.IntegrationInstances.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap[model.IntegrationInstances, integration.Instance](
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
}

func (r *integrationInstanceRepositoryImpl) FindByIDInternal(
	ctx context.Context, id string,
) (*integration.Instance, error) {
	stmt := table.IntegrationInstances.SELECT(
		table.IntegrationInstances.AllColumns,
	).FROM(
		table.IntegrationInstances,
	).WHERE(
		table.IntegrationInstances.ID.EQ(postgres.String(id)),
	).LIMIT(1)

	return transactor.QueryOptionalMap[model.IntegrationInstances, integration.Instance](
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
}

func (r *integrationInstanceRepositoryImpl) FindByProductAndProvider(
	ctx context.Context, tenantID string, productID string, providerType string,
) (*integration.Instance, error) {
	stmt := table.IntegrationInstances.SELECT(
		table.IntegrationInstances.AllColumns,
	).FROM(
		table.IntegrationInstances,
	).WHERE(
		table.IntegrationInstances.PlatformTenantID.EQ(postgres.String(tenantID)).AND(
			table.IntegrationInstances.ProductID.EQ(postgres.String(productID)),
		).AND(
			table.IntegrationInstances.ProviderType.EQ(postgres.String(providerType)),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap[model.IntegrationInstances, integration.Instance](
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
}

func (r *integrationInstanceRepositoryImpl) FindByProductAndProviderInternal(
	ctx context.Context, productID string, providerType string,
) (*integration.Instance, error) {
	stmt := table.IntegrationInstances.SELECT(
		table.IntegrationInstances.AllColumns,
	).FROM(
		table.IntegrationInstances,
	).WHERE(
		table.IntegrationInstances.ProductID.EQ(postgres.String(productID)).AND(
			table.IntegrationInstances.ProviderType.EQ(postgres.String(providerType)),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap[model.IntegrationInstances, integration.Instance](
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
}

func (r *integrationInstanceRepositoryImpl) ListByProduct(
	ctx context.Context, tenantID string, productID string,
) ([]integration.Instance, error) {
	stmt := table.IntegrationInstances.SELECT(
		table.IntegrationInstances.AllColumns,
	).FROM(
		table.IntegrationInstances,
	).WHERE(
		table.IntegrationInstances.PlatformTenantID.EQ(postgres.String(tenantID)).AND(
			table.IntegrationInstances.ProductID.EQ(postgres.String(productID)),
		),
	).ORDER_BY(
		table.IntegrationInstances.CreatedAt.DESC(),
	)

	return transactor.QueryMapSlice(ctx, r.db, stmt, r.mapper.ToDomain)
}

func (r *integrationInstanceRepositoryImpl) ListByProviderInternal(
	ctx context.Context,
	providerType string,
) ([]integration.Instance, error) {
	stmt := table.IntegrationInstances.SELECT(
		table.IntegrationInstances.AllColumns,
	).FROM(
		table.IntegrationInstances,
	).WHERE(
		table.IntegrationInstances.ProviderType.EQ(postgres.String(providerType)),
	).ORDER_BY(
		table.IntegrationInstances.CreatedAt.DESC(),
	)

	return transactor.QueryMapSlice(ctx, r.db, stmt, r.mapper.ToDomain)
}

func (r *integrationInstanceRepositoryImpl) Create(
	ctx context.Context, instance integration.Instance,
) (integration.Instance, error) {
	entity := r.mapper.ToEntity(instance)

	stmt := table.IntegrationInstances.INSERT(
		integrationInstancesUpdatableColumns(),
	).MODEL(entity).RETURNING(table.IntegrationInstances.AllColumns)

	return transactor.QueryMap[model.IntegrationInstances, integration.Instance](
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
}

func (r *integrationInstanceRepositoryImpl) Update(
	ctx context.Context, tenantID string, instance integration.Instance,
) (integration.Instance, error) {
	instance.UpdatedAt = time.Now()
	entity := r.mapper.ToEntity(instance)

	stmt := table.IntegrationInstances.UPDATE(
		integrationInstancesUpdatableColumns().Except(
			table.IntegrationInstances.ID,
			table.IntegrationInstances.PlatformTenantID,
			table.IntegrationInstances.ProductID,
			table.IntegrationInstances.ProviderType,
		),
	).MODEL(entity).WHERE(
		table.IntegrationInstances.ID.EQ(postgres.String(instance.ID)).AND(
			table.IntegrationInstances.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	).RETURNING(table.IntegrationInstances.AllColumns)

	return transactor.QueryMap[model.IntegrationInstances, integration.Instance](
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
}

func (r *integrationInstanceRepositoryImpl) DeleteByID(
	ctx context.Context, tenantID string, id string,
) error {
	stmt := table.IntegrationInstances.DELETE().WHERE(
		table.IntegrationInstances.ID.EQ(postgres.String(id)).AND(
			table.IntegrationInstances.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	)

	return transactor.Exec(ctx, r.db, stmt)
}
