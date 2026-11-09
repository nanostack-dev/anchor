package repository

import (
	"context"
	"database/sql"
	"log/slog"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/tenant"
	"anchor/internal/mapper"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
)

// tenantToDomain is the stateless mapper that the generated base calls.
func tenantToDomain(row model.PlatformTenants) tenant.PlatformTenant {
	return (&mapper.TenantMapper{}).ToDomain(row)
}

func _() {
	nanorepo.Implement[TenantRepository, tenantRepositoryImpl](
		table.PlatformTenants, tenantToDomain,
	)
}

func platformTenantsUpdatableColumns() postgres.ColumnList {
	return table.PlatformTenants.AllColumns.Except(
		table.PlatformTenants.CreatedAt,
		table.PlatformTenants.UpdatedAt,
	)
}

type TenantRepository interface {
	Create(ctx context.Context, tenant tenant.PlatformTenant) (tenant.PlatformTenant, error)
	Count(ctx context.Context) (int64, error)
	// FindAll is hand-written: it orders by created_at and id, and login
	// uses the first tenant.
	FindAll(ctx context.Context) ([]tenant.PlatformTenant, error)
}

type tenantRepositoryImpl struct {
	platformTenantsBase

	db           *sql.DB
	tenantMapper *mapper.TenantMapper
	logger       *slog.Logger
}

func NewTenantRepository(
	db *sql.DB, tenantMapper *mapper.TenantMapper, logger *slog.Logger,
) TenantRepository {
	return &tenantRepositoryImpl{
		platformTenantsBase: newPlatformTenantsBase(db),
		db:                  db,
		tenantMapper:        tenantMapper,
		logger:              logger.With(slog.String("component", "tenant_repository")),
	}
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

// Count is hand-written on the base: when Count is the only derived method of
// the file, nano gen repo v0.23.0-beta.3 writes a file that does not compile.
// The body gives the derived SQL, SELECT COUNT(*) FROM platform_tenants.
func (r *tenantRepositoryImpl) Count(ctx context.Context) (int64, error) {
	return r.count(ctx, nil)
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
