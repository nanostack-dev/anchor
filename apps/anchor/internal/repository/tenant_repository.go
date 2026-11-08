package repository

import (
	"context"
	"database/sql"
	"log/slog"

	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/tenant"
	"anchor/internal/mapper"

	"github.com/nanostack-dev/nanostack-framework/db/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

var _ TenantRepository = (*tenantRepositoryImpl)(nil)

func platformTenantsUpdatableColumns() postgres.ColumnList {
	return table.PlatformTenants.AllColumns.Except(
		table.PlatformTenants.CreatedAt,
		table.PlatformTenants.UpdatedAt,
	)
}

type TenantRepository interface {
	FindByID(ctx context.Context, id string) (functional.Option[tenant.PlatformTenant], error)
	Create(ctx context.Context, tenant tenant.PlatformTenant) (tenant.PlatformTenant, error)
	DeleteByID(ctx context.Context, id string) error
	Count(ctx context.Context) (int64, error)
	FindAll(ctx context.Context) ([]tenant.PlatformTenant, error)
}

type tenantRepositoryImpl struct {
	db           *sql.DB
	tenantMapper *mapper.TenantMapper
	logger       *slog.Logger
}

func NewTenantRepository(
	db *sql.DB, tenantMapper *mapper.TenantMapper, logger *slog.Logger,
) TenantRepository {
	return &tenantRepositoryImpl{
		db:           db,
		tenantMapper: tenantMapper,
		logger:       logger.With(slog.String("component", "tenant_repository")),
	}
}

func (r *tenantRepositoryImpl) FindByID(
	ctx context.Context, id string,
) (functional.Option[tenant.PlatformTenant], error) {
	stmt := table.PlatformTenants.SELECT(
		table.PlatformTenants.AllColumns,
	).FROM(
		table.PlatformTenants,
	).WHERE(
		table.PlatformTenants.ID.EQ(postgres.String(id)),
	).LIMIT(1)

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt, r.tenantMapper.ToDomain,
	)
}

func (r *tenantRepositoryImpl) Create(
	ctx context.Context, t tenant.PlatformTenant,
) (tenant.PlatformTenant, error) {
	entity := r.tenantMapper.ToEntity(t)

	stmt := table.PlatformTenants.INSERT(
		platformTenantsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.PlatformTenants.AllColumns)

	return transactor.QueryMap(
		ctx, r.db, stmt, r.tenantMapper.ToDomain,
	).Value()
}

func (r *tenantRepositoryImpl) DeleteByID(
	ctx context.Context, id string,
) error {
	stmt := table.PlatformTenants.DELETE().WHERE(table.PlatformTenants.ID.EQ(postgres.String(id)))

	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *tenantRepositoryImpl) Count(ctx context.Context) (int64, error) {
	return transactor.QueryCount(ctx, r.db, table.PlatformTenants.SELECT(postgres.COUNT(postgres.STAR))).Value()
}

func (r *tenantRepositoryImpl) FindAll(
	ctx context.Context,
) ([]tenant.PlatformTenant, error) {
	stmt := table.PlatformTenants.SELECT(table.PlatformTenants.AllColumns).
		ORDER_BY(table.PlatformTenants.CreatedAt.ASC(), table.PlatformTenants.ID.ASC())

	return transactor.QueryMap(
		ctx, r.db, stmt, r.tenantMapper.ToDomainList,
	).Value()
}
