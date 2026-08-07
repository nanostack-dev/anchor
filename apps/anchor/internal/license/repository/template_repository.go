package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/rs/zerolog"

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
) (*license.Template, error) {
	stmt := table.LicenseTemplates.SELECT(table.LicenseTemplates.AllColumns).
		FROM(table.LicenseTemplates).
		WHERE(
			licenseTemplateScope(tenantID, productID).
				AND(table.LicenseTemplates.ID.EQ(postgres.String(templateID))),
		).LIMIT(1)
	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.mapper.ToDomain).Value()
}

func (r *templateRepositoryImpl) FindByName(
	ctx context.Context, tenantID string, productID string, name string,
) (*license.Template, error) {
	stmt := table.LicenseTemplates.SELECT(table.LicenseTemplates.AllColumns).
		FROM(table.LicenseTemplates).
		WHERE(
			licenseTemplateScope(tenantID, productID).
				AND(table.LicenseTemplates.Name.EQ(postgres.String(name))).
				AND(activeOnly()),
		).LIMIT(1)
	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.mapper.ToDomain).Value()
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
		ORDER_BY(table.LicenseTemplates.Name.ASC())
	return transactor.QueryMapSlice(ctx, r.db, stmt, r.mapper.ToDomain).Value()
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
