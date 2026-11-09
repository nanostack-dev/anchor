package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/email"
	"anchor/internal/mapper"
)

func _() {
	nanorepo.Implement[TemplateRepository, templateRepositoryImpl](table.EmailTemplates, emailTemplateToDomain)

	// POST /v1/products/{product_id}/email/templates/search and the deprecated GET list.
	nanorepo.Search[model.EmailTemplates]("EmailTemplate", table.EmailTemplates,
		nanorepo.Map(emailTemplateToDomain),
		nanorepo.Scope(table.EmailTemplates.PlatformTenantID),
		nanorepo.Scope(table.EmailTemplates.ProductID),
		nanorepo.Filter(table.EmailTemplates.ID, nanorepo.In),
		nanorepo.Filter(table.EmailTemplates.Slug, nanorepo.In),
		nanorepo.Filter(table.EmailTemplates.IsActive, nanorepo.Eq),
		nanorepo.FullText(table.EmailTemplates.Name, table.EmailTemplates.Slug),
		nanorepo.Sort(
			table.EmailTemplates.CreatedAt, table.EmailTemplates.UpdatedAt,
			table.EmailTemplates.Name, table.EmailTemplates.Slug,
		),
		nanorepo.DefaultSort(table.EmailTemplates.CreatedAt, nanorepo.Desc),
		nanorepo.DefaultLimit(defaultPageLimit),
	)
}

// emailTemplateToDomain maps the rows that the base returns. EmailTemplateMapper has no state.
func emailTemplateToDomain(row model.EmailTemplates) email.Template {
	return (&mapper.EmailTemplateMapper{}).ToDomain(row)
}

func emailTemplatesUpdatableColumns() postgres.ColumnList {
	return table.EmailTemplates.AllColumns.Except(
		table.EmailTemplates.CreatedAt, table.EmailTemplates.UpdatedAt,
	)
}

type templateRepositoryImpl struct {
	emailTemplatesBase

	mapper *mapper.EmailTemplateMapper
	logger *slog.Logger
}

func NewTemplateRepository(
	db *sql.DB, m *mapper.EmailTemplateMapper, logger *slog.Logger,
) TemplateRepository {
	return &templateRepositoryImpl{
		emailTemplatesBase: newEmailTemplatesBase(db),
		mapper:             m,
		logger:             logger.With(slog.String("component", "email_template_repository")),
	}
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
		return r.FindByPlatformTenantIDAndProductIDAndID(ctx, in.TenantID, in.ProductID, in.ID)
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
