package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/email"
	"anchor/internal/mapper"
)

var _ TemplateVersionRepository = (*templateVersionRepositoryImpl)(nil)

func emailTemplateVersionsUpdatableColumns() postgres.ColumnList {
	return table.EmailTemplateVersions.AllColumns.Except(
		table.EmailTemplateVersions.CreatedAt, table.EmailTemplateVersions.UpdatedAt,
	)
}

type templateVersionRepositoryImpl struct {
	db     *sql.DB
	mapper *mapper.EmailTemplateVersionMapper
	logger zerolog.Logger
}

func NewTemplateVersionRepository(
	db *sql.DB, m *mapper.EmailTemplateVersionMapper, logger zerolog.Logger,
) TemplateVersionRepository {
	return &templateVersionRepositoryImpl{
		db:     db,
		mapper: m,
		logger: logger.With().Str("component", "email_template_version_repository").Logger(),
	}
}

func (r *templateVersionRepositoryImpl) FindByID(
	ctx context.Context, id string,
) (*email.TemplateVersion, error) {
	stmt := table.EmailTemplateVersions.SELECT(table.EmailTemplateVersions.AllColumns).
		FROM(table.EmailTemplateVersions).
		WHERE(table.EmailTemplateVersions.ID.EQ(postgres.String(id))).
		LIMIT(1)
	result := transactor.QueryOptionalMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
	if err := result.Err(); err != nil {
		return nil, err
	}
	if !result.IsPresent() {
		return nil, nil
	}
	value := result.Value()
	return &value, nil
}

func (r *templateVersionRepositoryImpl) FindCurrentDraft(
	ctx context.Context, templateID string,
) (*email.TemplateVersion, error) {
	return r.findByStatus(ctx, templateID, email.TemplateVersionStatusDraft)
}

func (r *templateVersionRepositoryImpl) FindCurrentPublished(
	ctx context.Context, templateID string,
) (*email.TemplateVersion, error) {
	return r.findByStatus(ctx, templateID, email.TemplateVersionStatusPublished)
}

func (r *templateVersionRepositoryImpl) findByStatus(
	ctx context.Context, templateID string, status email.TemplateVersionStatus,
) (*email.TemplateVersion, error) {
	stmt := table.EmailTemplateVersions.SELECT(table.EmailTemplateVersions.AllColumns).
		FROM(table.EmailTemplateVersions).
		WHERE(
			table.EmailTemplateVersions.TemplateID.EQ(postgres.String(templateID)).
				AND(table.EmailTemplateVersions.Status.EQ(postgres.String(string(status)))),
		).LIMIT(1)
	result := transactor.QueryOptionalMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
	if err := result.Err(); err != nil {
		return nil, err
	}
	if !result.IsPresent() {
		return nil, nil
	}
	value := result.Value()
	return &value, nil
}

func (r *templateVersionRepositoryImpl) List(
	ctx context.Context, templateID string, limit int64, offset int64,
) ([]email.TemplateVersion, error) {
	if limit <= 0 {
		limit = 50
	}
	stmt := table.EmailTemplateVersions.SELECT(table.EmailTemplateVersions.AllColumns).
		FROM(table.EmailTemplateVersions).
		WHERE(table.EmailTemplateVersions.TemplateID.EQ(postgres.String(templateID))).
		ORDER_BY(table.EmailTemplateVersions.VersionNumber.DESC()).
		LIMIT(limit).OFFSET(offset)
	return transactor.QueryMapSlice(ctx, r.db, stmt, r.mapper.ToDomain).Value()
}

func (r *templateVersionRepositoryImpl) Create(
	ctx context.Context, v email.TemplateVersion,
) (email.TemplateVersion, error) {
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	if v.UpdatedAt.IsZero() {
		v.UpdatedAt = v.CreatedAt
	}
	entity := r.mapper.ToEntity(v)
	stmt := table.EmailTemplateVersions.INSERT(emailTemplateVersionsUpdatableColumns()).
		MODEL(entity).
		RETURNING(table.EmailTemplateVersions.AllColumns)
	return transactor.QueryMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	).Value()
}

func (r *templateVersionRepositoryImpl) Update(
	ctx context.Context, v email.TemplateVersion,
) (email.TemplateVersion, error) {
	v.UpdatedAt = time.Now()
	entity := r.mapper.ToEntity(v)
	stmt := table.EmailTemplateVersions.UPDATE(
		emailTemplateVersionsUpdatableColumns().Except(
			table.EmailTemplateVersions.ID,
			table.EmailTemplateVersions.TemplateID,
			table.EmailTemplateVersions.VersionNumber,
		),
	).MODEL(entity).WHERE(
		table.EmailTemplateVersions.ID.EQ(postgres.String(v.ID)),
	).RETURNING(table.EmailTemplateVersions.AllColumns)
	return transactor.QueryMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	).Value()
}

func (r *templateVersionRepositoryImpl) UpdateStatus(
	ctx context.Context,
	id string,
	status email.TemplateVersionStatus,
	publishedAt *time.Time,
) error {
	now := time.Now()
	entity := model.EmailTemplateVersions{
		Status:      string(status),
		UpdatedAt:   now,
		PublishedAt: publishedAt,
	}
	cols := postgres.ColumnList{
		table.EmailTemplateVersions.Status,
		table.EmailTemplateVersions.UpdatedAt,
		table.EmailTemplateVersions.PublishedAt,
	}
	stmt := table.EmailTemplateVersions.UPDATE(cols).MODEL(entity).WHERE(
		table.EmailTemplateVersions.ID.EQ(postgres.String(id)),
	)
	return transactor.Exec(ctx, r.db, stmt).Err()
}

// NextVersionNumber issues monotonically increasing version numbers per
// template. We pick MAX+1 from the existing versions; the unique index
// (template_id, version_number) catches concurrent inserts.
func (r *templateVersionRepositoryImpl) NextVersionNumber(
	ctx context.Context, templateID string,
) (int32, error) {
	type versionRow struct {
		Max *int32
	}
	stmt := postgres.SELECT(
		postgres.MAXi(table.EmailTemplateVersions.VersionNumber).AS("version_row.max"),
	).FROM(table.EmailTemplateVersions).WHERE(
		table.EmailTemplateVersions.TemplateID.EQ(postgres.String(templateID)),
	)
	rows, err := transactor.QueryMapSlice(ctx, r.db, stmt, func(row versionRow) int32 {
		if row.Max == nil {
			return 0
		}
		return *row.Max
	}).Value()
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 1, nil
	}
	return rows[0] + 1, nil
}
