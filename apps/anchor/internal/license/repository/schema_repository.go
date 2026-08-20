package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/rs/zerolog"

	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/license"
	"anchor/internal/mapper"
)

var _ SchemaRepository = (*schemaRepositoryImpl)(nil)

func licenseSchemasUpdatableColumns() postgres.ColumnList {
	return table.LicenseSchemas.AllColumns.Except(
		table.LicenseSchemas.CreatedAt, table.LicenseSchemas.UpdatedAt,
	)
}

type schemaRepositoryImpl struct {
	db     *sql.DB
	mapper *mapper.LicenseSchemaMapper
	logger zerolog.Logger
}

func NewSchemaRepository(
	db *sql.DB, m *mapper.LicenseSchemaMapper, logger zerolog.Logger,
) SchemaRepository {
	return &schemaRepositoryImpl{
		db:     db,
		mapper: m,
		logger: logger.With().Str("component", "license_schema_repository").Logger(),
	}
}

func (r *schemaRepositoryImpl) FindByProduct(
	ctx context.Context, tenantID string, productID string,
) (functional.Option[license.Schema], error) {
	stmt := table.LicenseSchemas.SELECT(table.LicenseSchemas.AllColumns).
		FROM(table.LicenseSchemas).
		WHERE(
			table.LicenseSchemas.PlatformTenantID.EQ(postgres.String(tenantID)).
				AND(table.LicenseSchemas.ProductID.EQ(postgres.String(productID))),
		).LIMIT(1)
	return transactor.QueryOptionalMap(ctx, r.db, stmt, r.mapper.ToDomain)
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

func (r *schemaRepositoryImpl) DeleteByProduct(
	ctx context.Context, tenantID string, productID string,
) error {
	stmt := table.LicenseSchemas.DELETE().WHERE(
		table.LicenseSchemas.PlatformTenantID.EQ(postgres.String(tenantID)).
			AND(table.LicenseSchemas.ProductID.EQ(postgres.String(productID))),
	)
	return transactor.Exec(ctx, r.db, stmt).Err()
}
