package repository

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/license"
	"anchor/internal/mapper"
)

var _ SchemaFieldRepository = (*schemaFieldRepositoryImpl)(nil)

func licenseSchemaFieldsUpdatableColumns() postgres.ColumnList {
	return table.LicenseSchemaFields.AllColumns.Except(
		table.LicenseSchemaFields.CreatedAt, table.LicenseSchemaFields.UpdatedAt,
	)
}

type schemaFieldRepositoryImpl struct {
	db     *sql.DB
	mapper *mapper.LicenseSchemaFieldMapper
	logger zerolog.Logger
}

func NewSchemaFieldRepository(
	db *sql.DB, m *mapper.LicenseSchemaFieldMapper, logger zerolog.Logger,
) SchemaFieldRepository {
	return &schemaFieldRepositoryImpl{
		db:     db,
		mapper: m,
		logger: logger.With().Str("component", "license_schema_field_repository").Logger(),
	}
}

func (r *schemaFieldRepositoryImpl) ListBySchema(
	ctx context.Context, schemaID string,
) ([]license.Field, error) {
	stmt := table.LicenseSchemaFields.SELECT(table.LicenseSchemaFields.AllColumns).
		FROM(table.LicenseSchemaFields).
		WHERE(table.LicenseSchemaFields.LicenseSchemaID.EQ(postgres.String(schemaID))).
		ORDER_BY(table.LicenseSchemaFields.Name.ASC())
	return transactor.QueryMapSlice(ctx, r.db, stmt, r.mapper.ToDomain).Value()
}

func (r *schemaFieldRepositoryImpl) ReplaceAll(
	ctx context.Context, schemaID string, fields []license.Field,
) ([]license.Field, error) {
	deleteStmt := table.LicenseSchemaFields.DELETE().WHERE(
		table.LicenseSchemaFields.LicenseSchemaID.EQ(postgres.String(schemaID)),
	)
	if err := transactor.Exec(ctx, r.db, deleteStmt).Err(); err != nil {
		return nil, err
	}

	if len(fields) == 0 {
		return []license.Field{}, nil
	}

	now := time.Now()
	entities := make([]model.LicenseSchemaFields, 0, len(fields))
	for _, field := range fields {
		field.SchemaID = schemaID
		if field.CreatedAt.IsZero() {
			field.CreatedAt = now
		}
		if field.UpdatedAt.IsZero() {
			field.UpdatedAt = field.CreatedAt
		}
		entities = append(entities, r.mapper.ToEntity(field))
	}

	insertStmt := table.LicenseSchemaFields.INSERT(licenseSchemaFieldsUpdatableColumns()).
		MODELS(entities).
		RETURNING(table.LicenseSchemaFields.AllColumns)
	written, err := transactor.QueryMapSlice(ctx, r.db, insertStmt, r.mapper.ToDomain).Value()
	if err != nil {
		return nil, err
	}

	// RETURNING echoes insertion order; reads are ordered by name, so sort here
	// too and a caller cannot tell a write apart from the read that follows it.
	slices.SortFunc(written, func(a, b license.Field) int {
		return strings.Compare(a.Name, b.Name)
	})
	return written, nil
}
