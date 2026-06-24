package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/slicex"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	orgapikey "anchor/internal/domain/organization/apikey"
	"anchor/internal/mapper"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"
)

type OrganizationAPIKeyRepository interface {
	Create(
		ctx context.Context,
		apiKey orgapikey.OrganizationAPIKey,
	) (orgapikey.OrganizationAPIKey, error)
	Update(
		ctx context.Context,
		apiKey orgapikey.OrganizationAPIKey,
	) (orgapikey.OrganizationAPIKey, error)
	GetByID(
		ctx context.Context,
		organizationID, id string,
	) (*orgapikey.OrganizationAPIKey, error)
	GetByOrganizationIDAndName(
		ctx context.Context,
		organizationID, name string,
	) (*orgapikey.OrganizationAPIKey, error)
	GetByOrganizationIDAndHashedValue(
		ctx context.Context,
		organizationID, hashedValue string,
	) (*orgapikey.OrganizationAPIKey, error)
	GetByProductIDAndHashedValueInternal(
		ctx context.Context,
		productID, hashedValue string,
	) (*orgapikey.OrganizationAPIKey, error)
	SearchByOrganizationID(
		ctx context.Context,
		input orgapikey.SearchOrganizationAPIKeysInput,
	) (search.Result[orgapikey.OrganizationAPIKey], error)
	Delete(
		ctx context.Context,
		organizationID, id string,
	) error
	UpdateLastUsedAt(
		ctx context.Context,
		organizationID, id string,
	) error
	UpdateStatus(
		ctx context.Context,
		organizationID, id string,
		status orgapikey.Status,
	) error
	// GetByIDInternal fetches an API key by ID without tenant scoping.
	// Reserved for async queue workers and other trusted system-internal paths
	// where no authenticated tenant context is available.
	// Must never be called from tenant-facing API handlers.
	GetByIDInternal(
		ctx context.Context,
		id string,
	) (*orgapikey.OrganizationAPIKey, error)
}

var _ OrganizationAPIKeyRepository = (*organizationAPIKeyRepository)(nil)

func organizationAPIKeysUpdatableColumns() postgres.ColumnList {
	return table.OrganizationAPIKeys.AllColumns.Except(
		table.OrganizationAPIKeys.ID,
		table.OrganizationAPIKeys.OrganizationID,
		table.OrganizationAPIKeys.HashedValue,
		table.OrganizationAPIKeys.ObfuscatedValue,
		table.OrganizationAPIKeys.CreatedAt,
		table.OrganizationAPIKeys.UpdatedAt,
	)
}

type organizationAPIKeyWithPermissions struct {
	model.OrganizationAPIKeys
	Permissions []model.OrganizationAPIKeyPermissions
}

type organizationAPIKeyRepository struct {
	db     *sql.DB
	logger zerolog.Logger
	mapper *mapper.OrganizationAPIKeyMapper
}

func NewOrganizationAPIKeyRepository(
	db *sql.DB,
	mapper *mapper.OrganizationAPIKeyMapper,
	logger zerolog.Logger,
) OrganizationAPIKeyRepository {
	return &organizationAPIKeyRepository{
		db: db,
		logger: logger.With().Str(
			"component", "organization_api_key_repository",
		).Logger(),
		mapper: mapper,
	}
}

//nolint:dupl // mirrors product API key persistence flow with organization-scoped tables
func (r *organizationAPIKeyRepository) Create(
	ctx context.Context,
	apiKey orgapikey.OrganizationAPIKey,
) (orgapikey.OrganizationAPIKey, error) {
	entity := r.mapper.ToEntity(apiKey)
	stmt := table.OrganizationAPIKeys.INSERT(
		table.OrganizationAPIKeys.AllColumns.Except(
			table.OrganizationAPIKeys.CreatedAt,
			table.OrganizationAPIKeys.UpdatedAt,
		),
	).MODEL(entity).RETURNING(table.OrganizationAPIKeys.AllColumns)

	created, err := transactor.Query[model.OrganizationAPIKeys](ctx, r.db, stmt)
	if err != nil {
		r.logger.Error().Err(err).
			Str("api_key_id", apiKey.ID).
			Str("organization_id", apiKey.OrganizationID).
			Msg("Failed to create organization API key")
		return orgapikey.OrganizationAPIKey{}, err
	}

	var permissions []model.OrganizationAPIKeyPermissions
	if len(apiKey.Permissions) > 0 {
		permissions = r.mapper.PermissionsToEntity(apiKey.Permissions)
		permStmt := table.OrganizationAPIKeyPermissions.INSERT(
			table.OrganizationAPIKeyPermissions.AllColumns.Except(
				table.OrganizationAPIKeyPermissions.CreatedAt,
			),
		).MODELS(permissions)

		err = transactor.Exec(ctx, r.db, permStmt)
		if err != nil {
			r.logger.Error().Err(err).
				Str("api_key_id", apiKey.ID).
				Str("organization_id", apiKey.OrganizationID).
				Msg("Failed to create organization API key permissions")
			return orgapikey.OrganizationAPIKey{}, err
		}
	}

	return r.mapper.ToDomainWithPermissions(created, permissions), nil
}

func (r *organizationAPIKeyRepository) GetByID(
	ctx context.Context,
	organizationID, id string,
) (*orgapikey.OrganizationAPIKey, error) {
	stmt := postgres.SELECT(
		table.OrganizationAPIKeys.AllColumns,
		table.OrganizationAPIKeyPermissions.AllColumns,
	).FROM(
		table.OrganizationAPIKeys.LEFT_JOIN(
			table.OrganizationAPIKeyPermissions,
			table.OrganizationAPIKeys.ID.EQ(table.OrganizationAPIKeyPermissions.APIKeyID),
		),
	).WHERE(
		table.OrganizationAPIKeys.ID.EQ(postgres.String(id)).AND(
			table.OrganizationAPIKeys.OrganizationID.EQ(postgres.String(organizationID)),
		),
	)

	return transactor.QueryOptionalMap[
		organizationAPIKeyWithPermissions,
		orgapikey.OrganizationAPIKey,
	](
		ctx,
		r.db,
		stmt,
		func(row organizationAPIKeyWithPermissions) orgapikey.OrganizationAPIKey {
			return r.mapper.ToDomainWithPermissions(row.OrganizationAPIKeys, row.Permissions)
		},
	)
}

func (r *organizationAPIKeyRepository) GetByIDInternal(
	ctx context.Context,
	id string,
) (*orgapikey.OrganizationAPIKey, error) {
	stmt := postgres.SELECT(
		table.OrganizationAPIKeys.AllColumns,
		table.OrganizationAPIKeyPermissions.AllColumns,
	).FROM(
		table.OrganizationAPIKeys.LEFT_JOIN(
			table.OrganizationAPIKeyPermissions,
			table.OrganizationAPIKeys.ID.EQ(table.OrganizationAPIKeyPermissions.APIKeyID),
		),
	).WHERE(
		table.OrganizationAPIKeys.ID.EQ(postgres.String(id)),
	)

	return transactor.QueryOptionalMap[
		organizationAPIKeyWithPermissions,
		orgapikey.OrganizationAPIKey,
	](
		ctx,
		r.db,
		stmt,
		func(row organizationAPIKeyWithPermissions) orgapikey.OrganizationAPIKey {
			return r.mapper.ToDomainWithPermissions(row.OrganizationAPIKeys, row.Permissions)
		},
	)
}

func (r *organizationAPIKeyRepository) GetByOrganizationIDAndName(
	ctx context.Context,
	organizationID, name string,
) (*orgapikey.OrganizationAPIKey, error) {
	stmt := postgres.SELECT(
		table.OrganizationAPIKeys.AllColumns,
		table.OrganizationAPIKeyPermissions.AllColumns,
	).FROM(
		table.OrganizationAPIKeys.LEFT_JOIN(
			table.OrganizationAPIKeyPermissions,
			table.OrganizationAPIKeys.ID.EQ(table.OrganizationAPIKeyPermissions.APIKeyID),
		),
	).WHERE(
		table.OrganizationAPIKeys.OrganizationID.EQ(postgres.String(organizationID)).AND(
			table.OrganizationAPIKeys.Name.EQ(postgres.String(name)),
		),
	)

	return transactor.QueryOptionalMap[
		organizationAPIKeyWithPermissions,
		orgapikey.OrganizationAPIKey,
	](
		ctx,
		r.db,
		stmt,
		func(row organizationAPIKeyWithPermissions) orgapikey.OrganizationAPIKey {
			return r.mapper.ToDomainWithPermissions(row.OrganizationAPIKeys, row.Permissions)
		},
	)
}

func (r *organizationAPIKeyRepository) GetByOrganizationIDAndHashedValue(
	ctx context.Context,
	organizationID, hashedValue string,
) (*orgapikey.OrganizationAPIKey, error) {
	stmt := postgres.SELECT(
		table.OrganizationAPIKeys.AllColumns,
		table.OrganizationAPIKeyPermissions.AllColumns,
	).FROM(
		table.OrganizationAPIKeys.LEFT_JOIN(
			table.OrganizationAPIKeyPermissions,
			table.OrganizationAPIKeys.ID.EQ(table.OrganizationAPIKeyPermissions.APIKeyID),
		),
	).WHERE(
		table.OrganizationAPIKeys.OrganizationID.EQ(postgres.String(organizationID)).AND(
			table.OrganizationAPIKeys.HashedValue.EQ(postgres.String(hashedValue)),
		),
	)

	return transactor.QueryOptionalMap[
		organizationAPIKeyWithPermissions,
		orgapikey.OrganizationAPIKey,
	](
		ctx,
		r.db,
		stmt,
		func(row organizationAPIKeyWithPermissions) orgapikey.OrganizationAPIKey {
			return r.mapper.ToDomainWithPermissions(row.OrganizationAPIKeys, row.Permissions)
		},
	)
}

// GetByProductIDAndHashedValueInternal resolves an organization API key by its hashed
// value across all organizations within a product, deriving the organization
// from the key. It bypasses organization scope intentionally: the caller is the
// product (productApiKeyAuth) and the hash uniquely identifies a single key.
func (r *organizationAPIKeyRepository) GetByProductIDAndHashedValueInternal(
	ctx context.Context,
	productID, hashedValue string,
) (*orgapikey.OrganizationAPIKey, error) {
	stmt := postgres.SELECT(
		table.OrganizationAPIKeys.AllColumns,
		table.OrganizationAPIKeyPermissions.AllColumns,
	).FROM(
		table.OrganizationAPIKeys.
			LEFT_JOIN(
				table.OrganizationAPIKeyPermissions,
				table.OrganizationAPIKeys.ID.EQ(table.OrganizationAPIKeyPermissions.APIKeyID),
			).
			INNER_JOIN(
				table.Organizations,
				table.OrganizationAPIKeys.OrganizationID.EQ(table.Organizations.ID),
			),
	).WHERE(
		table.Organizations.ProductID.EQ(postgres.String(productID)).AND(
			table.OrganizationAPIKeys.HashedValue.EQ(postgres.String(hashedValue)),
		),
	)

	return transactor.QueryOptionalMap[
		organizationAPIKeyWithPermissions,
		orgapikey.OrganizationAPIKey,
	](
		ctx,
		r.db,
		stmt,
		func(row organizationAPIKeyWithPermissions) orgapikey.OrganizationAPIKey {
			return r.mapper.ToDomainWithPermissions(row.OrganizationAPIKeys, row.Permissions)
		},
	)
}

//nolint:dupl // mirrors product API key update flow with organization-scoped tables
func (r *organizationAPIKeyRepository) Update(
	ctx context.Context,
	apiKey orgapikey.OrganizationAPIKey,
) (orgapikey.OrganizationAPIKey, error) {
	stmt := table.OrganizationAPIKeys.UPDATE(
		organizationAPIKeysUpdatableColumns(),
	).MODEL(
		r.mapper.ToEntity(apiKey),
	).WHERE(
		table.OrganizationAPIKeys.ID.EQ(postgres.String(apiKey.ID)).AND(
			table.OrganizationAPIKeys.OrganizationID.EQ(postgres.String(apiKey.OrganizationID)),
		),
	).RETURNING(
		table.OrganizationAPIKeys.AllColumns,
	)

	updated, err := transactor.Query[model.OrganizationAPIKeys](ctx, r.db, stmt)
	if err != nil {
		return orgapikey.OrganizationAPIKey{}, err
	}

	permissions, err := r.getPermissionEntities(ctx, apiKey.OrganizationID, updated.ID)
	if err != nil {
		return orgapikey.OrganizationAPIKey{}, err
	}

	return r.mapper.ToDomainWithPermissions(updated, permissions), nil
}

func (r *organizationAPIKeyRepository) SearchByOrganizationID(
	ctx context.Context,
	input orgapikey.SearchOrganizationAPIKeysInput,
) (search.Result[orgapikey.OrganizationAPIKey], error) {
	whereStmt := table.OrganizationAPIKeys.OrganizationID.EQ(postgres.String(input.OrganizationID))
	whereStmt = r.applyFilters(whereStmt, input.Request.Filter)
	whereStmt = r.applyFullTextSearch(whereStmt, input.Request.FullTextSearch)

	query := table.OrganizationAPIKeys.SELECT(
		table.OrganizationAPIKeys.AllColumns,
	).WHERE(whereStmt)

	resultCount, err := transactor.QueryCount(
		ctx,
		r.db,
		table.OrganizationAPIKeys.SELECT(postgres.COUNT(postgres.STAR)).WHERE(whereStmt),
	)
	if err != nil {
		r.logger.Error().Err(err).Str(
			"organization_id", input.OrganizationID,
		).Msg("failed to count organization API keys")
		return search.Result[orgapikey.OrganizationAPIKey]{}, err
	}

	if len(input.Request.Sort) > 0 {
		for _, sort := range input.Request.Sort {
			switch sort.Field {
			case orgapikey.SortFieldOrganizationAPIKeyID:
				query = query.ORDER_BY(jetx.OrderBy(table.OrganizationAPIKeys.ID, sort.Direction))
			case orgapikey.SortFieldOrganizationAPIKeyCreatedAt:
				query = query.ORDER_BY(jetx.OrderBy(table.OrganizationAPIKeys.CreatedAt, sort.Direction))
			case orgapikey.SortFieldOrganizationAPIKeyUpdatedAt:
				query = query.ORDER_BY(jetx.OrderBy(table.OrganizationAPIKeys.UpdatedAt, sort.Direction))
			case orgapikey.SortFieldOrganizationAPIKeyName:
				query = query.ORDER_BY(jetx.OrderBy(table.OrganizationAPIKeys.Name, sort.Direction))
			case orgapikey.SortFieldOrganizationAPIKeyStatus:
				query = query.ORDER_BY(jetx.OrderBy(table.OrganizationAPIKeys.Status, sort.Direction))
			case orgapikey.SortFieldOrganizationAPIKeyLastUsed:
				query = query.ORDER_BY(jetx.OrderBy(table.OrganizationAPIKeys.LastUsedAt, sort.Direction))
			}
		}
	}

	query = query.LIMIT(int64(input.Request.Pagination.Limit)).OFFSET(int64(input.Request.Pagination.Offset))

	itemEntities, err := transactor.QueryMapSlice(
		ctx,
		r.db,
		query,
		func(entity model.OrganizationAPIKeys) model.OrganizationAPIKeys {
			return entity
		},
	)
	if err != nil {
		r.logger.Error().Err(err).Str(
			"organization_id", input.OrganizationID,
		).Msg("failed to search organization API keys")
		return search.Result[orgapikey.OrganizationAPIKey]{}, err
	}

	if len(itemEntities) == 0 {
		return search.Result[orgapikey.OrganizationAPIKey]{
			Items: []orgapikey.OrganizationAPIKey{},
			Count: 0,
			Total: resultCount,
		}, nil
	}

	apiKeyIDs := slicex.Map(itemEntities, func(entity model.OrganizationAPIKeys) string {
		return entity.ID
	})

	permissionEntities, err := r.getPermissionEntitiesByAPIKeyIDs(
		ctx,
		input.OrganizationID,
		apiKeyIDs,
	)
	if err != nil {
		r.logger.Error().Err(err).Str(
			"organization_id", input.OrganizationID,
		).Msg("failed to load organization API key permissions")
		return search.Result[orgapikey.OrganizationAPIKey]{}, err
	}

	permissionsByAPIKeyID := make(
		map[string][]model.OrganizationAPIKeyPermissions,
		len(itemEntities),
	)
	for _, permissionEntity := range permissionEntities {
		permissionsByAPIKeyID[permissionEntity.APIKeyID] = append(
			permissionsByAPIKeyID[permissionEntity.APIKeyID],
			permissionEntity,
		)
	}

	items := slicex.Map(itemEntities, func(entity model.OrganizationAPIKeys) orgapikey.OrganizationAPIKey {
		return r.mapper.ToDomainWithPermissions(entity, permissionsByAPIKeyID[entity.ID])
	})

	return search.Result[orgapikey.OrganizationAPIKey]{
		Items: items,
		Count: len(items),
		Total: resultCount,
	}, nil
}

func (r *organizationAPIKeyRepository) Delete(
	ctx context.Context,
	organizationID, id string,
) error {
	stmt := table.OrganizationAPIKeys.DELETE().WHERE(
		table.OrganizationAPIKeys.ID.EQ(postgres.String(id)).AND(
			table.OrganizationAPIKeys.OrganizationID.EQ(postgres.String(organizationID)),
		),
	)

	return transactor.Exec(ctx, r.db, stmt)
}

func (r *organizationAPIKeyRepository) UpdateLastUsedAt(
	ctx context.Context,
	organizationID, id string,
) error {
	now := time.Now()

	stmt := table.OrganizationAPIKeys.UPDATE(
		table.OrganizationAPIKeys.LastUsedAt,
		table.OrganizationAPIKeys.UpdatedAt,
	).SET(
		postgres.TimestampzT(now),
		postgres.TimestampzT(now),
	).WHERE(
		table.OrganizationAPIKeys.ID.EQ(postgres.String(id)).AND(
			table.OrganizationAPIKeys.OrganizationID.EQ(postgres.String(organizationID)),
		),
	)

	return transactor.Exec(ctx, r.db, stmt)
}

func (r *organizationAPIKeyRepository) UpdateStatus(
	ctx context.Context,
	organizationID, id string,
	status orgapikey.Status,
) error {
	now := time.Now()

	stmt := table.OrganizationAPIKeys.UPDATE(
		table.OrganizationAPIKeys.Status,
		table.OrganizationAPIKeys.UpdatedAt,
	).SET(
		postgres.String(string(status)),
		postgres.TimestampzT(now),
	).WHERE(
		table.OrganizationAPIKeys.ID.EQ(postgres.String(id)).AND(
			table.OrganizationAPIKeys.OrganizationID.EQ(postgres.String(organizationID)),
		).AND(
			table.OrganizationAPIKeys.Status.EQ(postgres.String(string(orgapikey.StatusActive))),
		),
	)

	return transactor.Exec(ctx, r.db, stmt)
}

func (r *organizationAPIKeyRepository) getPermissionEntities(
	ctx context.Context,
	organizationID, apiKeyID string,
) ([]model.OrganizationAPIKeyPermissions, error) {
	stmt := table.OrganizationAPIKeyPermissions.SELECT(
		table.OrganizationAPIKeyPermissions.AllColumns,
	).WHERE(
		table.OrganizationAPIKeyPermissions.APIKeyID.EQ(postgres.String(apiKeyID)).AND(
			table.OrganizationAPIKeyPermissions.OrganizationID.EQ(postgres.String(organizationID)),
		),
	)

	return transactor.QueryMapSlice(
		ctx,
		r.db,
		stmt,
		func(entity model.OrganizationAPIKeyPermissions) model.OrganizationAPIKeyPermissions {
			return entity
		},
	)
}

func (r *organizationAPIKeyRepository) getPermissionEntitiesByAPIKeyIDs(
	ctx context.Context,
	organizationID string,
	apiKeyIDs []string,
) ([]model.OrganizationAPIKeyPermissions, error) {
	if len(apiKeyIDs) == 0 {
		return []model.OrganizationAPIKeyPermissions{}, nil
	}

	apiKeyIDExpressions := jetx.ToStringExpressions(apiKeyIDs)
	stmt := table.OrganizationAPIKeyPermissions.SELECT(
		table.OrganizationAPIKeyPermissions.AllColumns,
	).WHERE(
		table.OrganizationAPIKeyPermissions.APIKeyID.IN(apiKeyIDExpressions...).AND(
			table.OrganizationAPIKeyPermissions.OrganizationID.EQ(postgres.String(organizationID)),
		),
	)

	return transactor.QueryMapSlice(
		ctx,
		r.db,
		stmt,
		func(entity model.OrganizationAPIKeyPermissions) model.OrganizationAPIKeyPermissions {
			return entity
		},
	)
}

//nolint:dupl // mirrors product API key filter logic with organization-scoped fields
func (r *organizationAPIKeyRepository) applyFilters(
	whereStmt postgres.BoolExpression,
	filter *orgapikey.SearchOrganizationAPIKeyFilter,
) postgres.BoolExpression {
	if filter == nil {
		return whereStmt
	}

	if len(filter.OrganizationAPIKeyIDs) > 0 {
		expressions := jetx.ToStringExpressions(filter.OrganizationAPIKeyIDs)
		whereStmt = whereStmt.AND(table.OrganizationAPIKeys.ID.IN(expressions...))
	}

	if len(filter.Names) > 0 {
		expressions := jetx.ToStringExpressions(filter.Names)
		whereStmt = whereStmt.AND(table.OrganizationAPIKeys.Name.IN(expressions...))
	}

	if len(filter.Status) > 0 {
		expressions := jetx.ToStringExpressions(filter.Status)
		whereStmt = whereStmt.AND(table.OrganizationAPIKeys.Status.IN(expressions...))
	}

	if filter.LastUsedBefore != nil {
		whereStmt = whereStmt.AND(table.OrganizationAPIKeys.LastUsedAt.LT(postgres.TimestampzT(*filter.LastUsedBefore)))
	}

	if filter.LastUsedAfter != nil {
		whereStmt = whereStmt.AND(table.OrganizationAPIKeys.LastUsedAt.GT(postgres.TimestampzT(*filter.LastUsedAfter)))
	}

	return whereStmt
}

func (r *organizationAPIKeyRepository) applyFullTextSearch(
	whereStmt postgres.BoolExpression,
	fullTextSearch *string,
) postgres.BoolExpression {
	if fullTextSearch == nil {
		return whereStmt
	}

	searchTerm := "%" + *fullTextSearch + "%"
	return whereStmt.AND(
		table.OrganizationAPIKeys.Name.LIKE(postgres.String(searchTerm)).OR(
			table.OrganizationAPIKeys.Description.LIKE(postgres.String(searchTerm)),
		),
	)
}
