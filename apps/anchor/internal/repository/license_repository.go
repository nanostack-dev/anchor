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
	"anchor/internal/domain/license"
	"anchor/internal/mapper"
)

var _ LicenseRepository = (*licenseRepositoryImpl)(nil)

func licensesUpdatableColumns() postgres.ColumnList {
	return table.Licenses.AllColumns.Except(
		table.Licenses.CreatedAt, table.Licenses.UpdatedAt,
	)
}

type LicenseRepository interface {
	FindByOrganization(
		ctx context.Context, productID string, organizationID string,
	) (*license.License, error)
	ListByProduct(ctx context.Context, productID string) ([]license.License, error)
	// CountByPlan returns how many licenses of the product reference the plan.
	CountByPlan(ctx context.Context, productID string, planID string) (int64, error)
	Create(ctx context.Context, lic license.License) (license.License, error)
	Update(
		ctx context.Context, productID string, lic license.License,
	) (license.License, error)
}

type licenseRepositoryImpl struct {
	db            *sql.DB
	licenseMapper *mapper.LicenseMapper
	logger        zerolog.Logger
}

func NewLicenseRepository(
	db *sql.DB, licenseMapper *mapper.LicenseMapper, logger zerolog.Logger,
) LicenseRepository {
	return &licenseRepositoryImpl{
		db:            db,
		licenseMapper: licenseMapper,
		logger:        logger.With().Str("component", "license_repository").Logger(),
	}
}

func (r *licenseRepositoryImpl) FindByOrganization(
	ctx context.Context, productID string, organizationID string,
) (*license.License, error) {
	stmt := table.Licenses.SELECT(table.Licenses.AllColumns).WHERE(
		table.Licenses.ProductID.EQ(postgres.String(productID)).AND(
			table.Licenses.OrganizationID.EQ(postgres.String(organizationID)),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.licenseMapper.ToDomain)
}

func (r *licenseRepositoryImpl) ListByProduct(
	ctx context.Context, productID string,
) ([]license.License, error) {
	stmt := table.Licenses.SELECT(table.Licenses.AllColumns).WHERE(
		table.Licenses.ProductID.EQ(postgres.String(productID)),
	).ORDER_BY(table.Licenses.CreatedAt.ASC())

	return transactor.QueryMapSlice(ctx, r.db, stmt, r.licenseMapper.ToDomain)
}

func (r *licenseRepositoryImpl) CountByPlan(
	ctx context.Context, productID string, planID string,
) (int64, error) {
	stmt := table.Licenses.SELECT(postgres.COUNT(postgres.STAR)).WHERE(
		table.Licenses.ProductID.EQ(postgres.String(productID)).AND(
			table.Licenses.PlanID.EQ(postgres.String(planID)),
		),
	)

	return transactor.QueryCount(ctx, r.db, stmt)
}

func (r *licenseRepositoryImpl) Create(
	ctx context.Context, lic license.License,
) (license.License, error) {
	entity := r.licenseMapper.ToEntity(lic)

	stmt := table.Licenses.INSERT(
		licensesUpdatableColumns(),
	).MODEL(entity).RETURNING(table.Licenses.AllColumns)

	created, err := transactor.Query[model.Licenses](ctx, r.db, stmt)
	if err != nil {
		return license.License{}, err
	}

	return r.licenseMapper.ToDomain(created), nil
}

func (r *licenseRepositoryImpl) Update(
	ctx context.Context, productID string, lic license.License,
) (license.License, error) {
	lic.UpdatedAt = time.Now()
	entity := r.licenseMapper.ToEntity(lic)

	stmt := table.Licenses.UPDATE(
		licensesUpdatableColumns().Except(
			table.Licenses.ID,
			table.Licenses.ProductID,
			table.Licenses.OrganizationID,
		),
	).MODEL(entity).WHERE(
		table.Licenses.ID.EQ(postgres.String(lic.ID)).AND(
			table.Licenses.ProductID.EQ(postgres.String(productID)),
		),
	).RETURNING(table.Licenses.AllColumns)

	updated, err := transactor.Query[model.Licenses](ctx, r.db, stmt)
	if err != nil {
		return license.License{}, err
	}

	return r.licenseMapper.ToDomain(updated), nil
}
