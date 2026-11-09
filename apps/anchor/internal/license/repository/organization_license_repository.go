package repository

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/db/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/license"
	"anchor/internal/mapper"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
)

// The base returns model rows: the repository maps them with the license mapper.
func _() {
	nanorepo.Implement[OrganizationLicenseRepository, organizationLicenseRepositoryImpl](
		table.OrganizationLicenses, nanorepo.NoMapper[model.OrganizationLicenses],
	)

	// POST /v1/products/{product_id}/licensing/organization-licenses/search: the
	// Product's customer book. The Organizations are the subject, so the license
	// joins with LEFT JOIN, and its tenant is the tenant of the Product, stated in
	// the ON clause: a WHERE on it would drop every Organization that holds no
	// license. The Product joins to carry the tenant.
	nanorepo.Search[organizationLicenseRow]("OrganizationLicense", table.Organizations,
		nanorepo.Map(organizationLicenseRowToDomain),
		nanorepo.From(organizationsWithTheirLicense),
		nanorepo.Scope(table.Organizations.ProductID),
		nanorepo.Scope(table.Products.PlatformTenantID),
		nanorepo.Filter(table.Organizations.ID, nanorepo.In.API("organization_ids")),
		nanorepo.Filter(table.OrganizationLicenses.TemplateID, nanorepo.In.API("license_template_ids")),
		nanorepo.Flag("Licensed", licenseHeld),
		nanorepo.FullText(table.Organizations.Name),
		nanorepo.SortKey(table.Organizations.Name).API("organization_name"),
		nanorepo.SortKey(table.OrganizationLicenses.InstantiatedAt).API("instantiated_at").NullsLast(),
		nanorepo.DefaultSort(table.Organizations.Name, nanorepo.Asc),
		nanorepo.DefaultLimit(10),
	)
}

// organizationLicenseRow is one row of the customer-book search: an Organization
// left-joined onto the license it holds. Every license column is a pointer
// because the join keeps the Organizations that hold none.
type organizationLicenseRow struct {
	model.Organizations
	LicenseID               *string    `alias:"organization_licenses.id"`
	LicensePlatformTenantID *string    `alias:"organization_licenses.platform_tenant_id"`
	LicenseProductID        *string    `alias:"organization_licenses.product_id"`
	LicenseOrganizationID   *string    `alias:"organization_licenses.organization_id"`
	LicenseTemplateID       *string    `alias:"organization_licenses.template_id"`
	LicenseValuesJSON       *string    `alias:"organization_licenses.values_json"`
	LicenseInstantiatedAt   *time.Time `alias:"organization_licenses.instantiated_at"`
	LicenseCreatedAt        *time.Time `alias:"organization_licenses.created_at"`
	LicenseUpdatedAt        *time.Time `alias:"organization_licenses.updated_at"`
	LicenseAdjustedFields   *string    `alias:"organization_licenses.adjusted_fields"`
}

func organizationsWithTheirLicense() postgres.ReadableTable {
	return table.Organizations.
		INNER_JOIN(table.Products, table.Products.ID.EQ(table.Organizations.ProductID)).
		LEFT_JOIN(
			table.OrganizationLicenses,
			table.OrganizationLicenses.OrganizationID.EQ(table.Organizations.ID).
				AND(table.OrganizationLicenses.ProductID.EQ(table.Organizations.ProductID)).
				AND(table.OrganizationLicenses.PlatformTenantID.EQ(table.Products.PlatformTenantID)),
		)
}

// organizationLicenseRowToDomain maps a search row: an Organization with the
// license it holds, if any.
func organizationLicenseRowToDomain(row organizationLicenseRow) license.OrganizationLicenseSummary {
	summary := license.OrganizationLicenseSummary{
		OrganizationID:   row.Organizations.ID,
		OrganizationName: row.Organizations.Name,
	}
	if row.LicenseID == nil {
		return summary
	}
	held := model.OrganizationLicenses{ID: *row.LicenseID}
	if row.LicensePlatformTenantID != nil {
		held.PlatformTenantID = *row.LicensePlatformTenantID
	}
	if row.LicenseProductID != nil {
		held.ProductID = *row.LicenseProductID
	}
	if row.LicenseOrganizationID != nil {
		held.OrganizationID = *row.LicenseOrganizationID
	}
	if row.LicenseTemplateID != nil {
		held.TemplateID = *row.LicenseTemplateID
	}
	if row.LicenseValuesJSON != nil {
		held.ValuesJSON = *row.LicenseValuesJSON
	}
	if row.LicenseInstantiatedAt != nil {
		held.InstantiatedAt = *row.LicenseInstantiatedAt
	}
	if row.LicenseCreatedAt != nil {
		held.CreatedAt = *row.LicenseCreatedAt
	}
	if row.LicenseUpdatedAt != nil {
		held.UpdatedAt = *row.LicenseUpdatedAt
	}
	if row.LicenseAdjustedFields != nil {
		held.AdjustedFields = *row.LicenseAdjustedFields
	}
	license := (&mapper.OrganizationLicenseMapper{}).ToDomain(held)
	summary.License = &license
	return summary
}

// licenseHeld is true for an Organization that holds a license.
func licenseHeld() postgres.BoolExpression {
	return table.OrganizationLicenses.ID.IS_NOT_NULL()
}

func organizationLicensesUpdatableColumns() postgres.ColumnList {
	return table.OrganizationLicenses.AllColumns.Except(
		table.OrganizationLicenses.CreatedAt, table.OrganizationLicenses.UpdatedAt,
	)
}

type organizationLicenseRepositoryImpl struct {
	organizationLicensesBase

	db     *sql.DB
	mapper *mapper.OrganizationLicenseMapper
	logger *slog.Logger
}

func NewOrganizationLicenseRepository(
	db *sql.DB, m *mapper.OrganizationLicenseMapper, logger *slog.Logger,
) OrganizationLicenseRepository {
	return &organizationLicenseRepositoryImpl{
		organizationLicensesBase: newOrganizationLicensesBase(db),
		db:                       db,
		mapper:                   m,
		logger:                   logger.With(slog.String("component", "organization_license_repository")),
	}
}

// organizationLicenseScope is the tenant and product predicate every
// tenant-facing statement carries.
func organizationLicenseScope(tenantID, productID string) postgres.BoolExpression {
	return table.OrganizationLicenses.PlatformTenantID.EQ(postgres.String(tenantID)).
		AND(table.OrganizationLicenses.ProductID.EQ(postgres.String(productID)))
}

func (r *organizationLicenseRepositoryImpl) FindByOrganization(
	ctx context.Context, tenantID string, productID string, organizationID string,
) (functional.Option[license.OrganizationLicense], error) {
	stmt := table.OrganizationLicenses.SELECT(table.OrganizationLicenses.AllColumns).
		FROM(table.OrganizationLicenses).
		WHERE(
			organizationLicenseScope(tenantID, productID).
				AND(table.OrganizationLicenses.OrganizationID.EQ(postgres.String(organizationID))),
		).LIMIT(1)
	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.mapper.ToDomain)
}

func (r *organizationLicenseRepositoryImpl) FindByOrganizations(
	ctx context.Context, tenantID string, productID string, organizationIDs []string,
) (map[string]license.OrganizationLicense, error) {
	if len(organizationIDs) == 0 {
		return map[string]license.OrganizationLicense{}, nil
	}

	stmt := table.OrganizationLicenses.SELECT(table.OrganizationLicenses.AllColumns).
		FROM(table.OrganizationLicenses).
		WHERE(
			organizationLicenseScope(tenantID, productID).
				AND(table.OrganizationLicenses.OrganizationID.IN(
					jetx.ToStringExpressions(organizationIDs)...,
				)),
		)

	rows, err := transactor.QueryMapSlice(ctx, r.db, stmt, r.mapper.ToDomain).Value()
	if err != nil {
		return nil, err
	}

	return functional.Slice(rows).ToMap(func(row license.OrganizationLicense) string {
		return row.OrganizationID
	}), nil
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

func (r *organizationLicenseRepositoryImpl) ListOrganizationIDsForTemplateAfter(
	ctx context.Context,
	tenantID string,
	productID string,
	templateID string,
	afterOrganizationID string,
	limit int,
) ([]string, error) {
	where := organizationLicenseScope(tenantID, productID).
		AND(table.OrganizationLicenses.TemplateID.EQ(postgres.String(templateID)))
	if afterOrganizationID != "" {
		where = where.AND(
			table.OrganizationLicenses.OrganizationID.GT(postgres.String(afterOrganizationID)),
		)
	}

	stmt := table.OrganizationLicenses.
		SELECT(table.OrganizationLicenses.OrganizationID).
		FROM(table.OrganizationLicenses).
		WHERE(where).
		ORDER_BY(table.OrganizationLicenses.OrganizationID.ASC()).
		LIMIT(int64(limit))

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
