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

var _ OrganizationLicenseRepository = (*organizationLicenseRepositoryImpl)(nil)

func organizationLicensesUpdatableColumns() postgres.ColumnList {
	return table.OrganizationLicenses.AllColumns.Except(
		table.OrganizationLicenses.CreatedAt, table.OrganizationLicenses.UpdatedAt,
	)
}

type organizationLicenseRepositoryImpl struct {
	db     *sql.DB
	mapper *mapper.OrganizationLicenseMapper
	logger zerolog.Logger
}

func NewOrganizationLicenseRepository(
	db *sql.DB, m *mapper.OrganizationLicenseMapper, logger zerolog.Logger,
) OrganizationLicenseRepository {
	return &organizationLicenseRepositoryImpl{
		db:     db,
		mapper: m,
		logger: logger.With().Str("component", "organization_license_repository").Logger(),
	}
}

// organizationLicenseScope is the tenant and product predicate every statement
// in this file carries. Written once so a new query cannot be added with half
// the scope.
func organizationLicenseScope(tenantID, productID string) postgres.BoolExpression {
	return table.OrganizationLicenses.PlatformTenantID.EQ(postgres.String(tenantID)).
		AND(table.OrganizationLicenses.ProductID.EQ(postgres.String(productID)))
}

func (r *organizationLicenseRepositoryImpl) FindByOrganization(
	ctx context.Context, tenantID string, productID string, organizationID string,
) (*license.OrganizationLicense, error) {
	return r.findByOrganization(ctx, tenantID, productID, organizationID, false)
}

func (r *organizationLicenseRepositoryImpl) FindByOrganizationForUpdate(
	ctx context.Context, tenantID string, productID string, organizationID string,
) (*license.OrganizationLicense, error) {
	return r.findByOrganization(ctx, tenantID, productID, organizationID, true)
}

func (r *organizationLicenseRepositoryImpl) findByOrganization(
	ctx context.Context, tenantID string, productID string, organizationID string, forUpdate bool,
) (*license.OrganizationLicense, error) {
	stmt := table.OrganizationLicenses.SELECT(table.OrganizationLicenses.AllColumns).
		FROM(table.OrganizationLicenses).
		WHERE(
			organizationLicenseScope(tenantID, productID).
				AND(table.OrganizationLicenses.OrganizationID.EQ(postgres.String(organizationID))),
		).LIMIT(1)
	if forUpdate {
		stmt = stmt.FOR(postgres.UPDATE())
	}
	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.mapper.ToDomain).Value()
}

func (r *organizationLicenseRepositoryImpl) Create(
	ctx context.Context, organizationLicense license.OrganizationLicense,
) (license.OrganizationLicense, error) {
	if organizationLicense.CreatedAt.IsZero() {
		organizationLicense.CreatedAt = time.Now()
	}
	if organizationLicense.UpdatedAt.IsZero() {
		organizationLicense.UpdatedAt = organizationLicense.CreatedAt
	}
	if organizationLicense.InstantiatedAt.IsZero() {
		organizationLicense.InstantiatedAt = organizationLicense.CreatedAt
	}
	entity := r.mapper.ToEntity(organizationLicense)
	stmt := table.OrganizationLicenses.INSERT(organizationLicensesUpdatableColumns()).
		MODEL(entity).
		RETURNING(table.OrganizationLicenses.AllColumns)
	return transactor.QueryMap(ctx, r.db, stmt, r.mapper.ToDomain).Value()
}

// Update rewrites the values an Organization holds, and only those. Identity,
// Organization and provenance stay on the row: this path cannot change which
// template a customer was sold.
func (r *organizationLicenseRepositoryImpl) Update(
	ctx context.Context, tenantID string, organizationLicense license.OrganizationLicense,
) (license.OrganizationLicense, error) {
	organizationLicense.UpdatedAt = time.Now()
	entity := r.mapper.ToEntity(organizationLicense)
	stmt := table.OrganizationLicenses.UPDATE(
		organizationLicensesUpdatableColumns().Except(
			table.OrganizationLicenses.ID,
			table.OrganizationLicenses.PlatformTenantID,
			table.OrganizationLicenses.ProductID,
			table.OrganizationLicenses.OrganizationID,
			table.OrganizationLicenses.TemplateID,
			table.OrganizationLicenses.InstantiatedAt,
		),
	).MODEL(entity).WHERE(
		organizationLicenseScope(tenantID, organizationLicense.ProductID).
			AND(table.OrganizationLicenses.ID.EQ(postgres.String(organizationLicense.ID))),
	).RETURNING(table.OrganizationLicenses.AllColumns)
	return transactor.QueryMap(ctx, r.db, stmt, r.mapper.ToDomain).Value()
}

// Restamp rewrites the values an Organization holds together with the
// provenance of the copy, which Update deliberately cannot touch. It is the
// only path that changes which template a customer is on, so the guarantee
// Update documents holds as a property of the code rather than of its callers.
// See docs/adr/0014-organization-licenses-are-migrated-in-bulk.md.
func (r *organizationLicenseRepositoryImpl) Restamp(
	ctx context.Context, tenantID string, organizationLicense license.OrganizationLicense,
) (license.OrganizationLicense, error) {
	organizationLicense.UpdatedAt = time.Now()
	entity := r.mapper.ToEntity(organizationLicense)
	stmt := table.OrganizationLicenses.UPDATE(
		organizationLicensesUpdatableColumns().Except(
			table.OrganizationLicenses.ID,
			table.OrganizationLicenses.PlatformTenantID,
			table.OrganizationLicenses.ProductID,
			table.OrganizationLicenses.OrganizationID,
		),
	).MODEL(entity).WHERE(
		organizationLicenseScope(tenantID, organizationLicense.ProductID).
			AND(table.OrganizationLicenses.ID.EQ(postgres.String(organizationLicense.ID))),
	).RETURNING(table.OrganizationLicenses.AllColumns)
	return transactor.QueryMap(ctx, r.db, stmt, r.mapper.ToDomain).Value()
}

// ListOrganizationIDsForTemplate returns every Organization in the Product
// whose license names the given template, ordered by identifier so a run's
// results are stable and a caller batching its own calls sees a fixed order.
func (r *organizationLicenseRepositoryImpl) ListOrganizationIDsForTemplate(
	ctx context.Context, tenantID string, productID string, templateID string,
) ([]string, error) {
	stmt := table.OrganizationLicenses.
		SELECT(table.OrganizationLicenses.OrganizationID).
		FROM(table.OrganizationLicenses).
		WHERE(
			organizationLicenseScope(tenantID, productID).
				AND(table.OrganizationLicenses.TemplateID.EQ(postgres.String(templateID))),
		).
		ORDER_BY(table.OrganizationLicenses.OrganizationID.ASC())

	return transactor.QueryMapSlice(
		ctx, r.db, stmt,
		func(entity model.OrganizationLicenses) string { return entity.OrganizationID },
	).Value()
}

// CountLicensesForTemplate reports how many Organization licenses still name
// the given template. A template delete calls this before the write, so the
// caller gets a clean 400 instead of the foreign key's own error.
func (r *organizationLicenseRepositoryImpl) CountLicensesForTemplate(
	ctx context.Context, tenantID string, productID string, templateID string,
) (int, error) {
	count, err := transactor.QueryCount(
		ctx, r.db,
		table.OrganizationLicenses.SELECT(postgres.COUNT(postgres.STAR)).
			FROM(table.OrganizationLicenses).
			WHERE(
				organizationLicenseScope(tenantID, productID).
					AND(table.OrganizationLicenses.TemplateID.EQ(postgres.String(templateID))),
			),
	).Value()
	if err != nil {
		return 0, err
	}
	return int(count), nil
}
