package repository

import (
	"context"
	"database/sql"
	"time"

	"anchor/internal/mapper"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/slicex"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/product/apikey"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"
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

	// GetByID retrieves a product API key by Name
	GetByID(ctx context.Context, productID, id string) (
		*apikey.ProductAPIKey, error,
	)

	// GetByProductIDAndName retrieves a product API key by product Name and name
	GetByProductIDAndName(
		ctx context.Context, productID, name string,
	) (*apikey.ProductAPIKey, error)

	// Delete deletes a product API key by Name
	Delete(ctx context.Context, productID, id string) error

	// SearchByProductId searches for product API keys with filtering, sorting, and pagination
	SearchByProductID(
		ctx context.Context, input apikey.SearchProductAPIKeysInput,
	) (search.Result[apikey.ProductAPIKey], error)

	// UpdateLastUsedAt updates the last used timestamp for a product API key
	UpdateLastUsedAt(ctx context.Context, productID, id string) error

	// GetByHashedValue retrieves a product API key by its hashed value
	GetByProductIDAndHashedValue(
		ctx context.Context, productID, hashedValue string,
	) (*apikey.ProductAPIKey, error)
}

var _ ProductAPIKeyRepository = (*productAPIKeyRepository)(nil)

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
	db     *sql.DB
	logger zerolog.Logger
	mapper *mapper.ProductAPIKeyMapper
}

func NewProductAPIKeyRepository(
	db *sql.DB,
	mapper *mapper.ProductAPIKeyMapper,
	logger zerolog.Logger,
) ProductAPIKeyRepository {
	return &productAPIKeyRepository{
		db: db,
		logger: logger.With().Str(
			"component", "product_api_key_repository",
		).Logger(),
		mapper: mapper,
	}
}

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

	created, err := transactor.Query[model.ProductAPIKeys](ctx, r.db, stmt).Value()
	if err != nil {
		r.logger.Error().Err(err).
			Str("api_key_id", apiKey.ID).
			Str("product_id", apiKey.ProductID).
			Msg("Failed to create product API key")
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
			r.logger.Error().Err(err).
				Str("api_key_id", apiKey.ID).
				Str("product_id", apiKey.ProductID).
				Msg("Failed to create product API key permissions")
			return apikey.ProductAPIKey{}, err
		}
	}
	return r.mapper.ToDomainWithPermissions(created, permissions), nil
}

func (r *productAPIKeyRepository) GetByID(
	ctx context.Context, productID, id string,
) (*apikey.ProductAPIKey, error) {
	return r.findOne(ctx, table.ProductAPIKeys.ID.EQ(postgres.String(id)).AND(
		table.ProductAPIKeys.ProductID.EQ(postgres.String(productID)),
	))
}

func (r *productAPIKeyRepository) GetByProductIDAndName(
	ctx context.Context, productID, name string,
) (*apikey.ProductAPIKey, error) {
	return r.findOne(ctx, table.ProductAPIKeys.ProductID.EQ(postgres.String(productID)).
		AND(table.ProductAPIKeys.Name.EQ(postgres.String(name))))
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
	).Value()
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

func (r *productAPIKeyRepository) Delete(
	ctx context.Context, productID, id string,
) error {
	stmt := table.ProductAPIKeys.DELETE().WHERE(
		table.ProductAPIKeys.ID.EQ(postgres.String(id)).AND(
			table.ProductAPIKeys.ProductID.EQ(postgres.String(productID)),
		),
	)

	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *productAPIKeyRepository) SearchByProductID(
	ctx context.Context, input apikey.SearchProductAPIKeysInput,
) (search.Result[apikey.ProductAPIKey], error) {
	whereStmt := table.ProductAPIKeys.ProductID.EQ(postgres.String(input.ProductID))

	whereStmt = r.applyFilters(whereStmt, input.Request.Filter)
	whereStmt = r.applyFullTextSearch(whereStmt, input.Request.FullTextSearch)

	pageResult, err := transactor.Page(
		r.db,
		func(entity model.ProductAPIKeys) model.ProductAPIKeys {
			return entity
		},
		table.ProductAPIKeys.AllColumns,
	).
		From(table.ProductAPIKeys).
		Where(whereStmt).
		OrderBy(transactor.SortColumns(
			input.Request.Sort,
			map[apikey.SortFieldProductAPIKey]postgres.Column{
				apikey.SortFieldProductAPIKeyCreatedAt: table.ProductAPIKeys.CreatedAt,
				apikey.SortFieldProductAPIKeyUpdatedAt: table.ProductAPIKeys.UpdatedAt,
				apikey.SortFieldProductAPIKeyName:      table.ProductAPIKeys.Name,
				apikey.SortFieldProductAPIKeyStatus:    table.ProductAPIKeys.Status,
				apikey.SortFieldProductAPIKeyLastUsed:  table.ProductAPIKeys.LastUsedAt,
			},
		)...).
		Run(ctx, input.Request.Pagination).
		Value()
	if err != nil {
		r.logger.Error().Err(err).Str(
			"productID", input.ProductID,
		).Msg("failed to search product API keys")
		return search.Result[apikey.ProductAPIKey]{}, err
	}

	itemEntities := pageResult.Items
	resultCount := pageResult.Total

	if len(itemEntities) == 0 {
		return search.Result[apikey.ProductAPIKey]{
			Items: []apikey.ProductAPIKey{},
			Count: 0,
			Total: resultCount,
		}, nil
	}

	apiKeyIDs := slicex.Map(itemEntities, func(entity model.ProductAPIKeys) string {
		return entity.ID
	})

	permissionEntities, err := r.getPermissionEntitiesByAPIKeyIDs(
		ctx, input.ProductID, apiKeyIDs,
	)
	if err != nil {
		r.logger.Error().Err(err).Str(
			"productID", input.ProductID,
		).Msg("failed to load product API key permissions")
		return search.Result[apikey.ProductAPIKey]{}, err
	}

	permissionsByAPIKeyID := make(map[string][]model.ProductAPIKeyPermissions, len(itemEntities))
	for _, permissionEntity := range permissionEntities {
		permissionsByAPIKeyID[permissionEntity.APIKeyID] = append(
			permissionsByAPIKeyID[permissionEntity.APIKeyID],
			permissionEntity,
		)
	}

	items := slicex.Map(itemEntities, func(entity model.ProductAPIKeys) apikey.ProductAPIKey {
		return r.mapper.ToDomainWithPermissions(entity, permissionsByAPIKeyID[entity.ID])
	})

	return search.Result[apikey.ProductAPIKey]{
		Items: items,
		Count: len(items),
		Total: resultCount,
	}, nil
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
) (*apikey.ProductAPIKey, error) {
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

func (r *productAPIKeyRepository) getPermissionEntitiesByAPIKeyIDs(
	ctx context.Context, productID string, apiKeyIDs []string,
) ([]model.ProductAPIKeyPermissions, error) {
	if len(apiKeyIDs) == 0 {
		return []model.ProductAPIKeyPermissions{}, nil
	}

	apiKeyIDExpressions := jetx.ToStringExpressions(apiKeyIDs)
	stmt := table.ProductAPIKeyPermissions.SELECT(
		table.ProductAPIKeyPermissions.AllColumns,
	).WHERE(
		table.ProductAPIKeyPermissions.APIKeyID.IN(apiKeyIDExpressions...).AND(
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

//nolint:dupl // mirrored by organization API key repository with equivalent filter semantics
func (r *productAPIKeyRepository) applyFilters(
	whereStmt postgres.BoolExpression,
	filter *apikey.SearchProductAPIKeyFilter,
) postgres.BoolExpression {
	if filter == nil {
		return whereStmt
	}

	if len(filter.ProductAPIKeyIDs) > 0 {
		expressions := jetx.ToStringExpressions(filter.ProductAPIKeyIDs)
		whereStmt = whereStmt.AND(table.ProductAPIKeys.ID.IN(expressions...))
	}

	if len(filter.Names) > 0 {
		expressions := jetx.ToStringExpressions(filter.Names)
		whereStmt = whereStmt.AND(table.ProductAPIKeys.Name.IN(expressions...))
	}

	if len(filter.Status) > 0 {
		expressions := jetx.ToStringExpressions(filter.Status)
		whereStmt = whereStmt.AND(table.ProductAPIKeys.Status.IN(expressions...))
	}

	if filter.LastUsedBefore != nil {
		whereStmt = whereStmt.AND(table.ProductAPIKeys.LastUsedAt.LT(postgres.TimestampzT(*filter.LastUsedBefore)))
	}

	if filter.LastUsedAfter != nil {
		whereStmt = whereStmt.AND(table.ProductAPIKeys.LastUsedAt.GT(postgres.TimestampzT(*filter.LastUsedAfter)))
	}

	return whereStmt
}

func (r *productAPIKeyRepository) applyFullTextSearch(
	whereStmt postgres.BoolExpression,
	fullTextSearch *string,
) postgres.BoolExpression {
	if fullTextSearch == nil {
		return whereStmt
	}

	searchTerm := "%" + *fullTextSearch + "%"
	return whereStmt.AND(
		table.ProductAPIKeys.Name.LIKE(postgres.String(searchTerm)).
			OR(table.ProductAPIKeys.Description.LIKE(postgres.String(searchTerm))),
	)
}

// findOne runs the shared row-and-permissions query. Each caller passes its
// own scope in where; this helper adds none.
func (r *productAPIKeyRepository) findOne(
	ctx context.Context, where postgres.BoolExpression,
) (*apikey.ProductAPIKey, error) {
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

	result := transactor.QueryOptionalMap(
		ctx, r.db, stmt, func(row productAPIKeyWithPermissions) apikey.ProductAPIKey {
			return r.mapper.ToDomainWithPermissions(row.ProductAPIKeys, row.Permissions)
		},
	)
	if err := result.Err(); err != nil {
		return nil, err
	}
	return result.ToPtr(), nil
}
