package repository

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/license"
	"anchor/internal/mapper"
)

func _() {
	nanorepo.Implement[SchemaRepository, schemaRepositoryImpl](
		table.LicenseSchemas, licenseSchemaToDomain,
	)
}

// licenseSchemaToDomain is the mapper of the generated base. The base calls it
// by name, so it cannot be a method value of the injected mapper.
func licenseSchemaToDomain(row model.LicenseSchemas) license.Schema {
	return (&mapper.LicenseSchemaMapper{}).ToDomain(row)
}

func licenseSchemasUpdatableColumns() postgres.ColumnList {
	return table.LicenseSchemas.AllColumns.Except(
		table.LicenseSchemas.CreatedAt, table.LicenseSchemas.UpdatedAt,
	)
}

type schemaRepositoryImpl struct {
	licenseSchemasBase

	db     *sql.DB
	mapper *mapper.LicenseSchemaMapper
	logger *slog.Logger
}

func NewSchemaRepository(
	db *sql.DB, m *mapper.LicenseSchemaMapper, logger *slog.Logger,
) SchemaRepository {
	return &schemaRepositoryImpl{
		licenseSchemasBase: newLicenseSchemasBase(db),
		db:                 db,
		mapper:             m,
		logger:             logger.With(slog.String("component", "license_schema_repository")),
	}
}

func (r *schemaRepositoryImpl) Create(
	ctx context.Context, schema license.Schema,
) (license.Schema, error) {
	if schema.CreatedAt.IsZero() {
		schema.CreatedAt = time.Now()
	}
	if schema.UpdatedAt.IsZero() {
		schema.UpdatedAt = schema.CreatedAt
	}
	entity := r.mapper.ToEntity(schema)
	stmt := table.LicenseSchemas.INSERT(licenseSchemasUpdatableColumns()).
		MODEL(entity).
		RETURNING(table.LicenseSchemas.AllColumns)
	return transactor.QueryMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	).Value()
}

func (r *schemaRepositoryImpl) Update(
	ctx context.Context, tenantID string, schema license.Schema,
) (license.Schema, error) {
	schema.UpdatedAt = time.Now()
	entity := r.mapper.ToEntity(schema)
	stmt := table.LicenseSchemas.UPDATE(
		licenseSchemasUpdatableColumns().Except(
			table.LicenseSchemas.ID,
			table.LicenseSchemas.PlatformTenantID,
			table.LicenseSchemas.ProductID,
		),
	).MODEL(entity).WHERE(
		table.LicenseSchemas.ID.EQ(postgres.String(schema.ID)).
			AND(table.LicenseSchemas.PlatformTenantID.EQ(postgres.String(tenantID))),
	).RETURNING(table.LicenseSchemas.AllColumns)
	return transactor.QueryMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	).Value()
}
