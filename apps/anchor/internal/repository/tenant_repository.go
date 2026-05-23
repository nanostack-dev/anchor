package repository

import (
	"context"
	"database/sql"

	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/tenant"
	"anchor/internal/mapper"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"
)

var _ TenantRepository = (*tenantRepositoryImpl)(nil)

func platformTenantsUpdatableColumns() postgres.ColumnList {
	return table.PlatformTenants.AllColumns.Except(
		table.PlatformTenants.CreatedAt,
		table.PlatformTenants.UpdatedAt,
	)
}

type TenantRepository interface {
	FindByID(ctx context.Context, id string, options *jetx.DBOptions) (
		*tenant.PlatformTenant, error,
	)
	Create(
		ctx context.Context, tenant tenant.PlatformTenant, options *jetx.DBOptions,
	) (tenant.PlatformTenant, error)
	DeleteByID(ctx context.Context, id string, options *jetx.DBOptions) error
	Count(ctx context.Context, options *jetx.DBOptions) (int64, error)
	FindAll(ctx context.Context, t *jetx.DBOptions) ([]tenant.PlatformTenant, error)
}

type tenantRepositoryImpl struct {
	db           *sql.DB
	tenantMapper *mapper.TenantMapper
	logger       zerolog.Logger
}

func NewTenantRepository(
	db *sql.DB, tenantMapper *mapper.TenantMapper, logger zerolog.Logger,
) TenantRepository { // Logger still passed in but not stored
	return &tenantRepositoryImpl{
		db:           db,
		tenantMapper: tenantMapper,
		logger: logger.With().Str(
			"component", "tenant_repository",
		).Logger(),
	}
}

func (r *tenantRepositoryImpl) FindByID(
	ctx context.Context, id string, options *jetx.DBOptions,
) (*tenant.PlatformTenant, error) {
	stmt := table.PlatformTenants.SELECT(
		table.PlatformTenants.AllColumns,
	).FROM(
		table.PlatformTenants,
	).WHERE(
		table.PlatformTenants.ID.EQ(postgres.String(id)),
	).LIMIT(1)

	return jetx.QueryOptionalMap[model.PlatformTenants, tenant.PlatformTenant](
		ctx, r.db, stmt,
		r.tenantMapper.ToDomain, options, // Pass options correctly
	)
}

func (r *tenantRepositoryImpl) Create(
	ctx context.Context, t tenant.PlatformTenant, options *jetx.DBOptions,
) (tenant.PlatformTenant, error) {
	// Name generation and timestamp setting should be handled elsewhere
	entity := r.tenantMapper.ToEntity(t)

	stmt := table.PlatformTenants.INSERT(
		platformTenantsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.PlatformTenants.AllColumns)

	return jetx.QueryMap[model.PlatformTenants, tenant.PlatformTenant](
		ctx, r.db, stmt, r.tenantMapper.ToDomain, options,
	) // Pass options correctly
}

func (r *tenantRepositoryImpl) DeleteByID(
	ctx context.Context, id string, options *jetx.DBOptions,
) error {
	stmt := table.PlatformTenants.DELETE().WHERE(table.PlatformTenants.ID.EQ(postgres.String(id)))

	return jetx.Exec(ctx, r.db, stmt, options) // Pass options correctly
}

func (r *tenantRepositoryImpl) Count(ctx context.Context, options *jetx.DBOptions) (
	int64, error,
) {
	return jetx.QueryCount(ctx, r.db, table.PlatformTenants, options)
}

func (r *tenantRepositoryImpl) FindAll(
	ctx context.Context, options *jetx.DBOptions,
) ([]tenant.PlatformTenant, error) {
	stmt := table.PlatformTenants.SELECT(table.PlatformTenants.AllColumns)

	return jetx.QueryMap[[]model.PlatformTenants, []tenant.PlatformTenant](
		ctx, r.db, stmt, r.tenantMapper.ToDomainList, options,
	)
}
