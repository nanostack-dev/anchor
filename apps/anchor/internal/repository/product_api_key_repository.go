package repository

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"anchor/internal/mapper"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/product/apikey"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"
)

type ProductAPIKeyRepository interface {
	// Create creates a new product API key
	Create(ctx context.Context, apiKey apikey.ProductAPIKey) (
		apikey.
			ProductAPIKey, error,
	)
	// Update updates an existing product API key (limited fields only)
	Update(ctx context.Context, apiKey apikey.ProductAPIKey) (
		apikey.ProductAPIKey, error,
	)

	// ReplacePermissions replaces all permissions assigned to an API key.
	ReplacePermissions(
		ctx context.Context,
		productID, apiKeyID string,
		permissions []apikey.ProductAPIKeyPermission,
	) error

	// DeletePermissionsByName removes a permission from all API keys of a product.
	DeletePermissionsByName(
		ctx context.Context,
		productID, permissionName string,
	) error

	// GetByProductAndID retrieves a product API key by product and ID.
	GetByProductAndID(ctx context.Context, productID, id string) (functional.Option[apikey.ProductAPIKey], error)

	// GetByProductAndName retrieves a product API key by product and name.
	GetByProductAndName(
		ctx context.Context, productID, name string,
	) (functional.Option[apikey.ProductAPIKey], error)

	// DeleteByProductAndID deletes a product API key of a product.
	DeleteByProductAndID(ctx context.Context, productID, id string) error

	// SearchByProductId searches for product API keys with filtering, sorting, and pagination
	SearchByProductID(ctx context.Context, q ProductAPIKeySearch) (search.Result[apikey.ProductAPIKey], error)

	// UpdateLastUsedAt updates the last used timestamp for a product API key
	UpdateLastUsedAt(ctx context.Context, productID, id string) error

	// GetByHashedValue retrieves a product API key by its hashed value
	GetByProductIDAndHashedValue(
		ctx context.Context, productID, hashedValue string,
	) (functional.Option[apikey.ProductAPIKey], error)
}

// productAPIKeyToDomain is the stateless mapper that the generated base calls.
func productAPIKeyToDomain(row model.ProductAPIKeys) apikey.ProductAPIKey {
	return (&mapper.ProductAPIKeyMapper{}).ToDomain(row)
}

func _() {
	nanorepo.Implement[ProductAPIKeyRepository, productAPIKeyRepository](
		table.ProductAPIKeys, productAPIKeyToDomain,
	)

	// POST /v1/products/{product_id}/api-keys/search. The API also accepts
	// expires_before and expires_after, which have no column: the mapper ignores
	// them, as it always did. The permissions of the page load in one statement.
	nanorepo.Search[model.ProductAPIKeys]("ProductAPIKey", table.ProductAPIKeys,
		nanorepo.MapPage((*productAPIKeyRows).page),
		nanorepo.Scope(table.ProductAPIKeys.ProductID),
		nanorepo.Filter(table.ProductAPIKeys.ID, nanorepo.In),
		nanorepo.Filter(table.ProductAPIKeys.Name, nanorepo.In),
		nanorepo.Filter(table.ProductAPIKeys.Status, nanorepo.In),
		nanorepo.FullText(table.ProductAPIKeys.Name, table.ProductAPIKeys.Description),
		nanorepo.Sort(
			table.ProductAPIKeys.ID, table.ProductAPIKeys.CreatedAt, table.ProductAPIKeys.Name,
			table.ProductAPIKeys.Status, table.ProductAPIKeys.LastUsedAt,
		),
		nanorepo.DefaultSort(table.ProductAPIKeys.CreatedAt, nanorepo.Asc),
		nanorepo.DefaultLimit(defaultPageLimit),
	)
}

func productAPIKeysUpdatableColumns() postgres.ColumnList {
	return table.ProductAPIKeys.AllColumns.Except(
		table.ProductAPIKeys.ID,
		table.ProductAPIKeys.ProductID,
		table.ProductAPIKeys.Mutable,
		table.ProductAPIKeys.HashedValue,
		table.ProductAPIKeys.ObfuscatedValue,
		table.ProductAPIKeys.CreatedAt,
		table.ProductAPIKeys.UpdatedAt,
	)
}

type productAPIKeyWithPermissions struct {
	model.ProductAPIKeys
	Permissions []model.ProductAPIKeyPermissions
}

type productAPIKeyRepository struct {
	productAPIKeysBase

	db     *sql.DB
	logger *slog.Logger
	mapper *mapper.ProductAPIKeyMapper
	rows   *productAPIKeyRows
}

// productAPIKeyRows maps a page of API key rows with their permissions. It is a
// field of the implementation because the generated search calls a method of a
// field.
type productAPIKeyRows struct {
	db     *sql.DB
	mapper *mapper.ProductAPIKeyMapper
}

// page reads the permissions of the paged keys in one statement. The page is
// taken on keys alone, so a LIMIT never cuts a key's permissions.
func (m *productAPIKeyRows) page(ctx context.Context, rows []model.ProductAPIKeys) ([]apikey.ProductAPIKey, error) {
	if len(rows) == 0 {
		return []apikey.ProductAPIKey{}, nil
	}
	ids := functional.Slice(rows).Map(func(entity model.ProductAPIKeys) string { return entity.ID })
	// Every row of the page is of the product that scoped the search.
	permissions, err := transactor.QueryMapSlice(
		ctx, m.db,
		table.ProductAPIKeyPermissions.SELECT(table.ProductAPIKeyPermissions.AllColumns).WHERE(
			table.ProductAPIKeyPermissions.APIKeyID.IN(jetx.ToStringExpressions(ids)...).AND(
				table.ProductAPIKeyPermissions.ProductID.EQ(postgres.String(rows[0].ProductID)),
			),
		),
		func(entity model.ProductAPIKeyPermissions) model.ProductAPIKeyPermissions { return entity },
	).Value()
	if err != nil {
		return nil, err
	}
	byKey := functional.Slice(permissions).GroupBy(
		func(p model.ProductAPIKeyPermissions) string { return p.APIKeyID },
	)
	return functional.Slice(rows).Map(func(entity model.ProductAPIKeys) apikey.ProductAPIKey {
		return m.mapper.ToDomainWithPermissions(entity, byKey[entity.ID])
	}), nil
}

func NewProductAPIKeyRepository(
	db *sql.DB,
	mapper *mapper.ProductAPIKeyMapper,
	logger *slog.Logger,
) ProductAPIKeyRepository {
	return &productAPIKeyRepository{
		productAPIKeysBase: newProductAPIKeysBase(db),
		db:                 db,
		logger:             logger.With(slog.String("component", "product_api_key_repository")),
		mapper:             mapper,
		rows:               &productAPIKeyRows{db: db, mapper: mapper},
	}
}

const productAPIKeyNameUniqueConstraint = "product_api_keys_product_id_name_key"

// ErrProductAPIKeyNameExists is returned by Create and Update when the product
// already has a key with that name: the unique constraint settles the race the
// service pre-check cannot.
var ErrProductAPIKeyNameExists = errors.New("product API key name already exists in product")

//nolint:dupl // mirrored by organization API key repository with equivalent flow
func (r *productAPIKeyRepository) Create(
	ctx context.Context, apiKey apikey.ProductAPIKey,
) (apikey.ProductAPIKey, error) {
	entity := r.mapper.ToEntity(apiKey)
	stmt := table.ProductAPIKeys.INSERT(
		table.ProductAPIKeys.AllColumns.Except(
			table.ProductAPIKeys.CreatedAt,
			table.ProductAPIKeys.UpdatedAt,
		),
	).MODEL(entity).RETURNING(table.ProductAPIKeys.AllColumns)

	created, err := transactor.Query[model.ProductAPIKeys](ctx, r.db, stmt).
		OnUnique(ErrProductAPIKeyNameExists, productAPIKeyNameUniqueConstraint).
		Value()
	if errors.Is(err, ErrProductAPIKeyNameExists) {
		return apikey.ProductAPIKey{}, err
	}
	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to create product API key",
			log.Err(err),
			slog.String("api_key_id", apiKey.ID),
			slog.String("product_id", apiKey.ProductID),
		)
		return apikey.ProductAPIKey{}, err
	}
	var permissions []model.ProductAPIKeyPermissions
	if len(apiKey.Permissions) > 0 {
		permissions = r.mapper.PermissionsToEntity(apiKey.Permissions)
		permStmt := table.ProductAPIKeyPermissions.INSERT(
			table.ProductAPIKeyPermissions.AllColumns.Except(
				table.ProductAPIKeyPermissions.CreatedAt,
			),
		).MODELS(permissions)

		err = transactor.Exec(ctx, r.db, permStmt).Err()
		if err != nil {
			r.logger.ErrorContext(ctx, "Failed to create product API key permissions",
				log.Err(err),
				slog.String("api_key_id", apiKey.ID),
				slog.String("product_id", apiKey.ProductID),
			)
			return apikey.ProductAPIKey{}, err
		}
	}
	return r.mapper.ToDomainWithPermissions(created, permissions), nil
}

//nolint:dupl // mirrored by organization API key repository with equivalent flow
func (r *productAPIKeyRepository) Update(
	ctx context.Context, apiKey apikey.ProductAPIKey,
) (apikey.ProductAPIKey, error) {
	stmt := table.ProductAPIKeys.UPDATE(
		productAPIKeysUpdatableColumns(),
	).MODEL(
		r.mapper.ToEntity(apiKey),
	).WHERE(
		table.ProductAPIKeys.ID.EQ(postgres.String(apiKey.ID)).
			AND(table.ProductAPIKeys.ProductID.EQ(postgres.String(apiKey.ProductID))),
	).RETURNING(
		table.ProductAPIKeys.AllColumns,
	)
	updated, err := transactor.Query[model.ProductAPIKeys](
		ctx, r.db, stmt,
	).OnUnique(ErrProductAPIKeyNameExists, productAPIKeyNameUniqueConstraint).Value()
	if err != nil {
		return apikey.ProductAPIKey{}, err
	}

	permissions, err := r.getPermissionEntities(ctx, apiKey.ProductID, updated.ID)
	if err != nil {
		return apikey.ProductAPIKey{}, err
	}

	return r.mapper.ToDomainWithPermissions(updated, permissions), nil
}

func (r *productAPIKeyRepository) ReplacePermissions(
	ctx context.Context,
	productID, apiKeyID string,
	permissions []apikey.ProductAPIKeyPermission,
) error {
	deleteStmt := table.ProductAPIKeyPermissions.DELETE().WHERE(
		table.ProductAPIKeyPermissions.ProductID.EQ(postgres.String(productID)).AND(
			table.ProductAPIKeyPermissions.APIKeyID.EQ(postgres.String(apiKeyID)),
		),
	)

	if err := transactor.Exec(ctx, r.db, deleteStmt).Err(); err != nil {
		return err
	}

	if len(permissions) == 0 {
		return nil
	}

	entities := r.mapper.PermissionsToEntity(permissions)
	insertStmt := table.ProductAPIKeyPermissions.INSERT(
		table.ProductAPIKeyPermissions.AllColumns.Except(
			table.ProductAPIKeyPermissions.CreatedAt,
		),
	).MODELS(entities)

	return transactor.Exec(ctx, r.db, insertStmt).Err()
}

func (r *productAPIKeyRepository) DeletePermissionsByName(
	ctx context.Context,
	productID, permissionName string,
) error {
	stmt := table.ProductAPIKeyPermissions.DELETE().WHERE(
		table.ProductAPIKeyPermissions.ProductID.EQ(postgres.String(productID)).AND(
			table.ProductAPIKeyPermissions.PermissionName.EQ(postgres.String(permissionName)),
		),
	)

	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *productAPIKeyRepository) UpdateLastUsedAt(
	ctx context.Context, productID, id string,
) error {
	now := time.Now()

	stmt := table.ProductAPIKeys.UPDATE(
		table.ProductAPIKeys.LastUsedAt,
		table.ProductAPIKeys.UpdatedAt,
	).SET(
		postgres.TimestampzT(now),
		postgres.TimestampzT(now),
	).WHERE(
		table.ProductAPIKeys.ID.EQ(postgres.String(id)).AND(
			table.ProductAPIKeys.ProductID.EQ(
				postgres.String(productID),
			),
		),
	)

	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *productAPIKeyRepository) GetByProductIDAndHashedValue(
	ctx context.Context, productID, hashedValue string,
) (functional.Option[apikey.ProductAPIKey], error) {
	return r.findOne(ctx, table.ProductAPIKeys.ProductID.EQ(postgres.String(productID)).AND(
		table.ProductAPIKeys.HashedValue.EQ(postgres.String(hashedValue)),
	))
}
func (r *productAPIKeyRepository) getPermissionEntities(
	ctx context.Context, productID, apiKeyID string,
) ([]model.ProductAPIKeyPermissions, error) {
	stmt := table.ProductAPIKeyPermissions.SELECT(
		table.ProductAPIKeyPermissions.AllColumns,
	).WHERE(
		table.ProductAPIKeyPermissions.APIKeyID.EQ(postgres.String(apiKeyID)).AND(
			table.ProductAPIKeyPermissions.ProductID.EQ(postgres.String(productID)),
		),
	)

	return transactor.QueryMapSlice(
		ctx, r.db, stmt,
		func(entity model.ProductAPIKeyPermissions) model.ProductAPIKeyPermissions {
			return entity
		},
	).Value()
}

// findOne runs the shared row-and-permissions query. Each caller passes its
// own scope in where; this helper adds none.
func (r *productAPIKeyRepository) findOne(
	ctx context.Context, where postgres.BoolExpression,
) (functional.Option[apikey.ProductAPIKey], error) {
	stmt := postgres.SELECT(
		table.ProductAPIKeys.AllColumns,
		table.ProductAPIKeyPermissions.AllColumns,
	).FROM(
		table.ProductAPIKeys.
			LEFT_JOIN(
				table.ProductAPIKeyPermissions,
				table.ProductAPIKeys.ID.EQ(table.ProductAPIKeyPermissions.APIKeyID),
			),
	).WHERE(where)

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt, func(row productAPIKeyWithPermissions) apikey.ProductAPIKey {
			return r.mapper.ToDomainWithPermissions(row.ProductAPIKeys, row.Permissions)
		},
	)
}
