package repository

import (
	"context"
	"database/sql"
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

var _ TemplateRepository = (*templateRepositoryImpl)(nil)

func licenseTemplatesUpdatableColumns() postgres.ColumnList {
	return table.LicenseTemplates.AllColumns.Except(
		table.LicenseTemplates.CreatedAt, table.LicenseTemplates.UpdatedAt,
	)
}

// activeOnly narrows to the tiers still on sale.
func activeOnly() postgres.BoolExpression {
	return table.LicenseTemplates.Status.EQ(postgres.String(string(license.TemplateActive)))
}

type templateRepositoryImpl struct {
	db     *sql.DB
	mapper *mapper.LicenseTemplateMapper
	logger zerolog.Logger
}

func NewTemplateRepository(
	db *sql.DB, m *mapper.LicenseTemplateMapper, logger zerolog.Logger,
) TemplateRepository {
	return &templateRepositoryImpl{
		db:     db,
		mapper: m,
		logger: logger.With().Str("component", "license_template_repository").Logger(),
	}
}

// licenseTemplateScope is the tenant and product predicate every statement in
// this file carries. Written once so a new query cannot be added with half the
// scope.
func licenseTemplateScope(tenantID, productID string) postgres.BoolExpression {
	return table.LicenseTemplates.PlatformTenantID.EQ(postgres.String(tenantID)).
		AND(table.LicenseTemplates.ProductID.EQ(postgres.String(productID)))
}

func (r *templateRepositoryImpl) FindByID(
	ctx context.Context, tenantID string, productID string, templateID string,
) (functional.Option[license.Template], error) {
	stmt := table.LicenseTemplates.SELECT(table.LicenseTemplates.AllColumns).
		FROM(table.LicenseTemplates).
		WHERE(
			licenseTemplateScope(tenantID, productID).
				AND(table.LicenseTemplates.ID.EQ(postgres.String(templateID))),
		).LIMIT(1)
	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.mapper.ToDomain)
}

func (r *templateRepositoryImpl) FindByName(
	ctx context.Context, tenantID string, productID string, name string,
) (functional.Option[license.Template], error) {
	stmt := table.LicenseTemplates.SELECT(table.LicenseTemplates.AllColumns).
		FROM(table.LicenseTemplates).
		WHERE(
			licenseTemplateScope(tenantID, productID).
				AND(table.LicenseTemplates.Name.EQ(postgres.String(name))).
				AND(activeOnly()),
		).LIMIT(1)
	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.mapper.ToDomain)
}

func (r *templateRepositoryImpl) ListByProduct(
	ctx context.Context, tenantID string, productID string, status *license.TemplateStatus,
) ([]license.Template, error) {
	where := licenseTemplateScope(tenantID, productID)
	if status != nil {
		where = where.AND(
			table.LicenseTemplates.Status.EQ(postgres.String(string(*status))),
		)
	}
	stmt := table.LicenseTemplates.SELECT(table.LicenseTemplates.AllColumns).
		FROM(table.LicenseTemplates).
		WHERE(where).
		ORDER_BY(table.LicenseTemplates.Name.ASC(), table.LicenseTemplates.ID.ASC())
	return transactor.QueryMapSlice(ctx, r.db, stmt, r.mapper.ToDomain).Value()
}

func (r *templateRepositoryImpl) Search(
	ctx context.Context, in license.SearchTemplatesInput,
) (search.Result[license.Template], error) {
	where := licenseTemplateScope(in.TenantID, in.ProductID)
	if filter := in.Request.Filter; filter != nil && len(filter.Statuses) > 0 {
		statuses := functional.Slice(filter.Statuses).Map(func(status license.TemplateStatus) string {
			return string(status)
		})
		where = where.AND(table.LicenseTemplates.Status.IN(jetx.ToStringExpressions(statuses)...))
	}
	if term := in.Request.FullTextSearch; term != nil && *term != "" {
		if fullText := jetx.BuildSubstringFilter(
			[]postgres.ColumnString{table.LicenseTemplates.Name}, *term,
		); fullText != nil {
			where = where.AND(fullText)
		}
	}

	order := transactor.Keys(
		in.Request.Sort,
		map[license.SortFieldTemplate]transactor.Key[model.LicenseTemplates]{
			license.SortFieldTemplateName: transactor.StringKey(
				table.LicenseTemplates.Name, func(row model.LicenseTemplates) string { return row.Name },
			),
			license.SortFieldTemplateStatus: transactor.StringKey(
				table.LicenseTemplates.Status, func(row model.LicenseTemplates) string { return row.Status },
			),
			license.SortFieldTemplateCreatedAt: transactor.TimeKey(
				table.LicenseTemplates.CreatedAt, func(row model.LicenseTemplates) time.Time { return row.CreatedAt },
			),
			license.SortFieldTemplateUpdatedAt: transactor.TimeKey(
				table.LicenseTemplates.UpdatedAt, func(row model.LicenseTemplates) time.Time { return row.UpdatedAt },
			),
		},
		transactor.StringKey(table.LicenseTemplates.ID, func(row model.LicenseTemplates) string { return row.ID }),
	)

	return transactor.Page(r.db, r.mapper.ToDomain, table.LicenseTemplates.AllColumns).
		From(table.LicenseTemplates).
		Where(where).
		Keyset(order).
		Run(ctx, in.Request.Pagination).
		Value()
}

func (r *templateRepositoryImpl) Create(
	ctx context.Context, template license.Template,
) (license.Template, error) {
	if template.CreatedAt.IsZero() {
		template.CreatedAt = time.Now()
	}
	if template.UpdatedAt.IsZero() {
		template.UpdatedAt = template.CreatedAt
	}
	entity := r.mapper.ToEntity(template)
	stmt := table.LicenseTemplates.INSERT(licenseTemplatesUpdatableColumns()).
		MODEL(entity).
		RETURNING(table.LicenseTemplates.AllColumns)
	return transactor.QueryMap(ctx, r.db, stmt, r.mapper.ToDomain).Value()
}

func (r *templateRepositoryImpl) Update(
	ctx context.Context, tenantID string, template license.Template,
) (license.Template, error) {
	template.UpdatedAt = time.Now()
	entity := r.mapper.ToEntity(template)
	stmt := table.LicenseTemplates.UPDATE(
		licenseTemplatesUpdatableColumns().Except(
			table.LicenseTemplates.ID,
			table.LicenseTemplates.PlatformTenantID,
			table.LicenseTemplates.ProductID,
			table.LicenseTemplates.Status,
		),
	).MODEL(entity).WHERE(
		licenseTemplateScope(tenantID, template.ProductID).
			AND(table.LicenseTemplates.ID.EQ(postgres.String(template.ID))),
	).RETURNING(table.LicenseTemplates.AllColumns)
	return transactor.QueryMap(ctx, r.db, stmt, r.mapper.ToDomain).Value()
}

func (r *templateRepositoryImpl) Archive(
	ctx context.Context, tenantID string, productID string, templateID string,
) (license.Template, error) {
	stmt := table.LicenseTemplates.UPDATE(
		table.LicenseTemplates.Status,
	).SET(
		postgres.String(string(license.TemplateArchived)),
	).WHERE(
		licenseTemplateScope(tenantID, productID).
			AND(table.LicenseTemplates.ID.EQ(postgres.String(templateID))),
	).RETURNING(table.LicenseTemplates.AllColumns)
	return transactor.QueryMap(ctx, r.db, stmt, r.mapper.ToDomain).Value()
}

// Delete removes the row outright. The service checks
// CountLicensesForTemplate before calling this; fk_organization_licenses_template
// (migration 000028) is the guarantee, and a race that slips past the check
// fails here instead, surfaced through pgerr.IsForeignKeyViolation.
func (r *templateRepositoryImpl) Delete(
	ctx context.Context, tenantID string, productID string, templateID string,
) error {
	stmt := table.LicenseTemplates.DELETE().WHERE(
		licenseTemplateScope(tenantID, productID).
			AND(table.LicenseTemplates.ID.EQ(postgres.String(templateID))),
	)
	return transactor.Exec(ctx, r.db, stmt).Err()
}
