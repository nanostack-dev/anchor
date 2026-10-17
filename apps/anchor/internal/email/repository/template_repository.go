package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"github.com/go-jet/jet/v2/postgres"
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
	ctx context.Context, tenantID string, productID string, id string,
) (functional.Option[email.Template], error) {
	stmt := table.EmailTemplates.SELECT(table.EmailTemplates.AllColumns).
		FROM(table.EmailTemplates).
		WHERE(
			table.EmailTemplates.ID.EQ(postgres.String(id)).
				AND(table.EmailTemplates.PlatformTenantID.EQ(postgres.String(tenantID))).
				AND(table.EmailTemplates.ProductID.EQ(postgres.String(productID))),
		).LIMIT(1)
	return transactor.QueryOptionalMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
}

func (r *templateRepositoryImpl) FindBySlug(
	ctx context.Context, tenantID string, productID string, slug string,
) (functional.Option[email.Template], error) {
	stmt := table.EmailTemplates.SELECT(table.EmailTemplates.AllColumns).
		FROM(table.EmailTemplates).
		WHERE(
			table.EmailTemplates.PlatformTenantID.EQ(postgres.String(tenantID)).
				AND(table.EmailTemplates.ProductID.EQ(postgres.String(productID))).
				AND(table.EmailTemplates.Slug.EQ(postgres.String(slug))),
		).LIMIT(1)
	return transactor.QueryOptionalMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
}

func (r *templateRepositoryImpl) FindBySlugInternal(
	ctx context.Context, productID string, slug string,
) (functional.Option[email.Template], error) {
	stmt := table.EmailTemplates.SELECT(table.EmailTemplates.AllColumns).
		FROM(table.EmailTemplates).
		WHERE(
			table.EmailTemplates.ProductID.EQ(postgres.String(productID)).
				AND(table.EmailTemplates.Slug.EQ(postgres.String(slug))),
		).LIMIT(1)
	return transactor.QueryOptionalMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
}

func (r *templateRepositoryImpl) Search(
	ctx context.Context, in email.SearchTemplatesInput,
) (search.Result[email.Template], error) {
	where := table.EmailTemplates.PlatformTenantID.EQ(postgres.String(in.TenantID)).
		AND(table.EmailTemplates.ProductID.EQ(postgres.String(in.ProductID)))

	if filter := in.Request.Filter; filter != nil {
		if ids := jetx.BuildIDFilter(table.EmailTemplates.ID, filter.IDs); ids != nil {
			where = where.AND(ids)
		}
		if slugs := jetx.BuildStringArrayFilter(table.EmailTemplates.Slug, filter.Slugs); slugs != nil {
			where = where.AND(slugs)
		}
		if filter.IsActive != nil {
			where = where.AND(table.EmailTemplates.IsActive.EQ(postgres.Bool(*filter.IsActive)))
		}
	}
	if term := in.Request.FullTextSearch; term != nil && *term != "" {
		if fullText := jetx.BuildSubstringFilter(
			[]postgres.ColumnString{table.EmailTemplates.Name, table.EmailTemplates.Slug}, *term,
		); fullText != nil {
			where = where.AND(fullText)
		}
	}

	order, err := transactor.StrictKeys(
		in.Request.Sort,
		map[email.SortFieldTemplate]transactor.Key[model.EmailTemplates]{
			email.SortFieldTemplateCreatedAt: transactor.TimeKey(
				table.EmailTemplates.CreatedAt, func(row model.EmailTemplates) time.Time { return row.CreatedAt },
			),
			email.SortFieldTemplateUpdatedAt: transactor.TimeKey(
				table.EmailTemplates.UpdatedAt, func(row model.EmailTemplates) time.Time { return row.UpdatedAt },
			),
			email.SortFieldTemplateName: transactor.StringKey(
				table.EmailTemplates.Name, func(row model.EmailTemplates) string { return row.Name },
			),
			email.SortFieldTemplateSlug: transactor.StringKey(
				table.EmailTemplates.Slug, func(row model.EmailTemplates) string { return row.Slug },
			),
		},
		transactor.StringKey(table.EmailTemplates.ID, func(row model.EmailTemplates) string { return row.ID }),
	)
	if err != nil {
		return search.Result[email.Template]{}, err
	}

	return transactor.Page(r.db, r.mapper.ToDomain, table.EmailTemplates.AllColumns).
		From(table.EmailTemplates).
		Where(where).
		Keyset(order).
		Run(ctx, in.Request.Pagination).
		Value()
}

const emailTemplateSlugUniqueConstraint = "email_templates_product_id_slug_key"

// ErrTemplateSlugExists is returned by Create when the product already has a
// template with that slug: the unique constraint settles the race the service
// pre-check cannot.
var ErrTemplateSlugExists = errors.New("email template slug already exists in product")

func (r *templateRepositoryImpl) Create(
	ctx context.Context, t email.Template,
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
	return transactor.QueryMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	).OnUnique(ErrTemplateSlugExists, emailTemplateSlugUniqueConstraint).Value()
}

func (r *templateRepositoryImpl) Update(
	ctx context.Context, in email.UpdateTemplateInput,
) (functional.Option[email.Template], error) {
	var columns postgres.ColumnList
	var entity model.EmailTemplates
	if in.Name != nil {
		columns = append(columns, table.EmailTemplates.Name)
		entity.Name = *in.Name
	}
	if in.Description != nil {
		columns = append(columns, table.EmailTemplates.Description)
		entity.Description = *in.Description
	}
	if in.IsActive != nil {
		columns = append(columns, table.EmailTemplates.IsActive)
		entity.IsActive = *in.IsActive
	}
	if len(columns) == 0 {
		return r.FindByID(ctx, in.TenantID, in.ProductID, in.ID)
	}
	stmt := table.EmailTemplates.UPDATE(columns).MODEL(entity).WHERE(
		table.EmailTemplates.ID.EQ(postgres.String(in.ID)).
			AND(table.EmailTemplates.PlatformTenantID.EQ(postgres.String(in.TenantID))).
			AND(table.EmailTemplates.ProductID.EQ(postgres.String(in.ProductID))),
	).RETURNING(table.EmailTemplates.AllColumns)
	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.mapper.ToDomain)
}

func (r *templateRepositoryImpl) SaveExamples(
	ctx context.Context,
	tenantID string,
	productID string,
	templateID string,
	examples []email.TemplateExample,
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
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *templateRepositoryImpl) SetVersionPointers(
	ctx context.Context,
	tenantID string,
	templateID string,
	draftVersionID *string,
	publishedVersionID *string,
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
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *templateRepositoryImpl) DeleteByID(
	ctx context.Context, tenantID string, productID string, id string,
) error {
	stmt := table.EmailTemplates.DELETE().WHERE(
		table.EmailTemplates.ID.EQ(postgres.String(id)).
			AND(table.EmailTemplates.PlatformTenantID.EQ(postgres.String(tenantID))).
			AND(table.EmailTemplates.ProductID.EQ(postgres.String(productID))),
	)
	return transactor.Exec(ctx, r.db, stmt).Err()
}
