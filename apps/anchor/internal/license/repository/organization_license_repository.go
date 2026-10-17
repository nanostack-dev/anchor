package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
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

// organizationLicenseScope is the tenant and product predicate every
// tenant-facing statement carries.
func organizationLicenseScope(tenantID, productID string) postgres.BoolExpression {
	return table.OrganizationLicenses.PlatformTenantID.EQ(postgres.String(tenantID)).
		AND(table.OrganizationLicenses.ProductID.EQ(postgres.String(productID)))
}

// ListUninitializedAdjustmentsInternal is only used by the startup backfill;
// tenant-facing services must never use this cross-tenant query.
func (r *organizationLicenseRepositoryImpl) ListUninitializedAdjustmentsInternal(
	ctx context.Context, limit int,
) ([]license.OrganizationLicense, error) {
	stmt := table.OrganizationLicenses.SELECT(table.OrganizationLicenses.AllColumns).
		FROM(table.OrganizationLicenses).
		WHERE(table.OrganizationLicenses.AdjustedFields.EQ(postgres.RawString("'null'::jsonb"))).
		ORDER_BY(table.OrganizationLicenses.ID.ASC()).LIMIT(int64(limit))
	return transactor.QueryMapSlice(ctx, r.db, stmt, r.mapper.ToDomain).Value()
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

// organizationLicenseJoin left-joins each Organization onto the license it
// holds. Left, not inner: an Organization that was never licensed is a result
// with no license, and dropping it would hide exactly the customers an operator
// is looking for half the time. organization_licenses.organization_id is
// unique, so the join cannot multiply rows.
func organizationLicenseJoin(tenantID string) postgres.ReadableTable {
	return table.Organizations.LEFT_JOIN(
		table.OrganizationLicenses,
		table.OrganizationLicenses.OrganizationID.EQ(table.Organizations.ID).
			AND(table.OrganizationLicenses.ProductID.EQ(table.Organizations.ProductID)).
			// In the join rather than the WHERE: organizations carries no tenant
			// column of its own, so this is the only place the scope can be
			// stated, and moving it to the WHERE would drop every Organization
			// holding no license.
			AND(table.OrganizationLicenses.PlatformTenantID.EQ(
				postgres.String(tenantID),
			)),
	)
}

// escapeLikeTerm makes an operator's search term literal. Without it a
// customer named "100% Renewable" is unfindable by typing its name, because
// the percent sign matches anything.
func escapeLikeTerm(term string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(term)
}

// licenseHeldFilter narrows to Organizations that hold a license, or to those
// that hold none. The left join is what makes both expressible from one query.
func licenseHeldFilter(licensed *bool) postgres.BoolExpression {
	switch {
	case licensed == nil:
		return nil
	case *licensed:
		return table.OrganizationLicenses.ID.IS_NOT_NULL()
	default:
		return table.OrganizationLicenses.ID.IS_NULL()
	}
}

func (r *organizationLicenseRepositoryImpl) Search(
	ctx context.Context, in license.SearchOrganizationLicensesInput,
) (search.Result[license.OrganizationLicenseSummary], error) {
	where := table.Organizations.ProductID.EQ(postgres.String(in.ProductID))

	if filter := in.Request.Filter; filter != nil {
		if ids := jetx.BuildIDFilter(
			table.Organizations.ID, filter.OrganizationIDs,
		); ids != nil {
			where = where.AND(ids)
		}
		if templates := jetx.BuildIDFilter(
			table.OrganizationLicenses.TemplateID, filter.LicenseTemplateIDs,
		); templates != nil {
			where = where.AND(templates)
		}
		if held := licenseHeldFilter(filter.Licensed); held != nil {
			where = where.AND(held)
		}
	}

	// ILIKE rather than jetx.BuildSubstringFilter, which builds a
	// case-sensitive LIKE. An operator typing a customer's name into a search
	// box does not capitalise it the way the record does, and "northwind"
	// finding nothing reads as a broken box rather than as a precise one. The
	// shared builder is what every other search in Anchor uses; fixing it there
	// would change all of them at once and belongs in the framework.
	if term := in.Request.FullTextSearch; term != nil && *term != "" {
		lowered := postgres.String("%" + escapeLikeTerm(strings.ToLower(*term)) + "%")
		where = where.AND(postgres.LOWER(table.Organizations.Name).LIKE(lowered))
	}

	order, err := organizationLicenseOrder(in.Request.Sort)
	if err != nil {
		return search.Result[license.OrganizationLicenseSummary]{}, err
	}

	return transactor.Page(
		r.db,
		r.mapper.SummaryToDomain,
		table.Organizations.AllColumns,
		table.OrganizationLicenses.AllColumns,
	).
		From(organizationLicenseJoin(in.TenantID)).
		Where(where).
		Keyset(order).
		Run(ctx, in.Request.Pagination).
		Value()
}

// organizationLicenseOrder is the keyset ordering of the customer-book search.
//
// instantiated_at comes from the outer side of the join, so it is NULL for an
// Organization holding no license: a nullable key, although the column is NOT
// NULL in its own table. NullsLast keeps the organizations holding no license
// after every licensed one in both directions, and the cursor follows that
// placement. Organizations hold one row each, so the organization id is the
// tiebreak.
func organizationLicenseOrder(
	sorts []search.Sort[license.SortFieldOrganizationLicense],
) (transactor.Order[mapper.OrganizationLicenseSummaryRow], error) {
	return transactor.StrictKeys(
		sorts,
		map[license.SortFieldOrganizationLicense]transactor.Key[mapper.OrganizationLicenseSummaryRow]{
			license.SortFieldOrganizationLicenseOrganizationName: transactor.StringKey(
				table.Organizations.Name,
				func(row mapper.OrganizationLicenseSummaryRow) string { return row.Organizations.Name },
			),
			license.SortFieldOrganizationLicenseInstantiatedAt: transactor.NullableTimeKey(
				table.OrganizationLicenses.InstantiatedAt,
				func(row mapper.OrganizationLicenseSummaryRow) *time.Time {
					if row.License == nil {
						return nil
					}
					return &row.License.InstantiatedAt
				},
			).NullsLast(),
		},
		transactor.StringKey(
			table.Organizations.ID,
			func(row mapper.OrganizationLicenseSummaryRow) string { return row.Organizations.ID },
		),
	)
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
