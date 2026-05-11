package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/shared/toolkit"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/email"
	"anchor/internal/mapper"
)

var _ TemplateRepository = (*templateRepositoryImpl)(nil)

func emailTemplatesUpdatableColumns() postgres.ColumnList {
	return table.EmailTemplates.AllColumns.Except(
		table.EmailTemplates.CreatedAt, table.EmailTemplates.UpdatedAt,
	)
}

type templateRepositoryImpl struct {
	db     *sql.DB
	mapper *mapper.EmailTemplateMapper
	logger zerolog.Logger
}

func NewTemplateRepository(
	db *sql.DB, m *mapper.EmailTemplateMapper, logger zerolog.Logger,
) TemplateRepository {
	return &templateRepositoryImpl{
		db:     db,
		mapper: m,
		logger: logger.With().Str("component", "email_template_repository").Logger(),
	}
}

func (r *templateRepositoryImpl) FindByID(
	ctx context.Context, tenantID string, productID string, id string, options *toolkit.DBOptions,
) (*email.Template, error) {
	stmt := table.EmailTemplates.SELECT(table.EmailTemplates.AllColumns).
		FROM(table.EmailTemplates).
		WHERE(
			table.EmailTemplates.ID.EQ(postgres.String(id)).
				AND(table.EmailTemplates.PlatformTenantID.EQ(postgres.String(tenantID))).
				AND(table.EmailTemplates.ProductID.EQ(postgres.String(productID))),
		).LIMIT(1)
	return toolkit.QueryOptionalMap[model.EmailTemplates, email.Template](
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *templateRepositoryImpl) FindBySlug(
	ctx context.Context, tenantID string, productID string, slug string, options *toolkit.DBOptions,
) (*email.Template, error) {
	stmt := table.EmailTemplates.SELECT(table.EmailTemplates.AllColumns).
		FROM(table.EmailTemplates).
		WHERE(
			table.EmailTemplates.PlatformTenantID.EQ(postgres.String(tenantID)).
				AND(table.EmailTemplates.ProductID.EQ(postgres.String(productID))).
				AND(table.EmailTemplates.Slug.EQ(postgres.String(slug))),
		).LIMIT(1)
	return toolkit.QueryOptionalMap[model.EmailTemplates, email.Template](
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *templateRepositoryImpl) FindBySlugInternal(
	ctx context.Context, productID string, slug string, options *toolkit.DBOptions,
) (*email.Template, error) {
	stmt := table.EmailTemplates.SELECT(table.EmailTemplates.AllColumns).
		FROM(table.EmailTemplates).
		WHERE(
			table.EmailTemplates.ProductID.EQ(postgres.String(productID)).
				AND(table.EmailTemplates.Slug.EQ(postgres.String(slug))),
		).LIMIT(1)
	return toolkit.QueryOptionalMap[model.EmailTemplates, email.Template](
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *templateRepositoryImpl) List(
	ctx context.Context, tenantID string, productID string, limit int64, offset int64, options *toolkit.DBOptions,
) ([]email.Template, error) {
	if limit <= 0 {
		limit = 50
	}
	stmt := table.EmailTemplates.SELECT(table.EmailTemplates.AllColumns).
		FROM(table.EmailTemplates).
		WHERE(
			table.EmailTemplates.PlatformTenantID.EQ(postgres.String(tenantID)).
				AND(table.EmailTemplates.ProductID.EQ(postgres.String(productID))),
		).
		ORDER_BY(table.EmailTemplates.CreatedAt.DESC()).
		LIMIT(limit).OFFSET(offset)
	return toolkit.QueryMapSlice(ctx, r.db, stmt, r.mapper.ToDomain, options)
}

func (r *templateRepositoryImpl) Create(
	ctx context.Context, t email.Template, options *toolkit.DBOptions,
) (email.Template, error) {
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = t.CreatedAt
	}
	entity := r.mapper.ToEntity(t)
	stmt := table.EmailTemplates.INSERT(emailTemplatesUpdatableColumns()).
		MODEL(entity).
		RETURNING(table.EmailTemplates.AllColumns)
	return toolkit.QueryMap[model.EmailTemplates, email.Template](
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *templateRepositoryImpl) Update(
	ctx context.Context, tenantID string, t email.Template, options *toolkit.DBOptions,
) (email.Template, error) {
	t.UpdatedAt = time.Now()
	entity := r.mapper.ToEntity(t)
	stmt := table.EmailTemplates.UPDATE(
		emailTemplatesUpdatableColumns().Except(
			table.EmailTemplates.ID,
			table.EmailTemplates.PlatformTenantID,
			table.EmailTemplates.ProductID,
		),
	).MODEL(entity).WHERE(
		table.EmailTemplates.ID.EQ(postgres.String(t.ID)).
			AND(table.EmailTemplates.PlatformTenantID.EQ(postgres.String(tenantID))),
	).RETURNING(table.EmailTemplates.AllColumns)
	return toolkit.QueryMap[model.EmailTemplates, email.Template](
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *templateRepositoryImpl) SaveExamples(
	ctx context.Context,
	tenantID string,
	productID string,
	templateID string,
	examples []email.TemplateExample,
	options *toolkit.DBOptions,
) error {
	examplesJSON := "[]"
	if len(examples) > 0 {
		if b, err := json.Marshal(examples); err == nil {
			examplesJSON = string(b)
		}
	}
	now := time.Now()
	entity := model.EmailTemplates{
		ExampleData: examplesJSON,
		UpdatedAt:   now,
	}
	stmt := table.EmailTemplates.UPDATE(
		table.EmailTemplates.ExampleData,
		table.EmailTemplates.UpdatedAt,
	).MODEL(entity).WHERE(
		table.EmailTemplates.ID.EQ(postgres.String(templateID)).
			AND(table.EmailTemplates.PlatformTenantID.EQ(postgres.String(tenantID))).
			AND(table.EmailTemplates.ProductID.EQ(postgres.String(productID))),
	)
	return toolkit.Exec(ctx, r.db, stmt, options)
}

func (r *templateRepositoryImpl) SetVersionPointers(
	ctx context.Context,
	tenantID string,
	templateID string,
	draftVersionID *string,
	publishedVersionID *string,
	options *toolkit.DBOptions,
) error {
	now := time.Now()
	entity := model.EmailTemplates{
		DraftVersionID:     draftVersionID,
		PublishedVersionID: publishedVersionID,
		UpdatedAt:          now,
	}
	stmt := table.EmailTemplates.UPDATE(
		table.EmailTemplates.DraftVersionID,
		table.EmailTemplates.PublishedVersionID,
		table.EmailTemplates.UpdatedAt,
	).MODEL(entity).WHERE(
		table.EmailTemplates.ID.EQ(postgres.String(templateID)).
			AND(table.EmailTemplates.PlatformTenantID.EQ(postgres.String(tenantID))),
	)
	return toolkit.Exec(ctx, r.db, stmt, options)
}

func (r *templateRepositoryImpl) DeleteByID(
	ctx context.Context, tenantID string, productID string, id string, options *toolkit.DBOptions,
) error {
	stmt := table.EmailTemplates.DELETE().WHERE(
		table.EmailTemplates.ID.EQ(postgres.String(id)).
			AND(table.EmailTemplates.PlatformTenantID.EQ(postgres.String(tenantID))).
			AND(table.EmailTemplates.ProductID.EQ(postgres.String(productID))),
	)
	return toolkit.Exec(ctx, r.db, stmt, options)
}
