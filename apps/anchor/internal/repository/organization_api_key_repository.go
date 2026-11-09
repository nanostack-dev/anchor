package repository

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	orgapikey "anchor/internal/domain/organization/apikey"
	"anchor/internal/mapper"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"
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
	) (functional.Option[orgapikey.OrganizationAPIKey], error)
	GetByOrganizationIDAndName(
		ctx context.Context,
		organizationID, name string,
	) (functional.Option[orgapikey.OrganizationAPIKey], error)
	GetByOrganizationIDAndHashedValue(
		ctx context.Context,
		organizationID, hashedValue string,
	) (functional.Option[orgapikey.OrganizationAPIKey], error)
	GetByProductIDAndHashedValueInternal(
		ctx context.Context,
		productID, hashedValue string,
	) (functional.Option[orgapikey.OrganizationAPIKey], error)
	SearchByOrganizationID(
		ctx context.Context, q OrganizationAPIKeySearch,
	) (search.Result[orgapikey.OrganizationAPIKey], error)
	DeleteByOrganizationIDAndID(
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
	) (functional.Option[orgapikey.OrganizationAPIKey], error)
}

// organizationAPIKeyToDomain is the stateless mapper that the generated base calls.
func organizationAPIKeyToDomain(row model.OrganizationAPIKeys) orgapikey.OrganizationAPIKey {
	return (&mapper.OrganizationAPIKeyMapper{}).ToDomain(row)
}

func _() {
	nanorepo.Implement[OrganizationAPIKeyRepository, organizationAPIKeyRepository](
		table.OrganizationAPIKeys, organizationAPIKeyToDomain,
	)

	// POST /v1/products/{product_id}/organizations/{organization_id}/api-keys/search.
	// last_used_after and last_used_before are exclusive bounds. The permissions of
	// the page load in one statement.
	nanorepo.Search[model.OrganizationAPIKeys]("OrganizationAPIKey", table.OrganizationAPIKeys,
		nanorepo.MapPage((*organizationAPIKeyRows).page),
		nanorepo.Scope(table.OrganizationAPIKeys.OrganizationID),
		nanorepo.Filter(table.OrganizationAPIKeys.ID, nanorepo.In),
		nanorepo.Filter(table.OrganizationAPIKeys.Name, nanorepo.In),
		nanorepo.Filter(table.OrganizationAPIKeys.Status, nanorepo.In),
		nanorepo.Filter(table.OrganizationAPIKeys.LastUsedAt,
			nanorepo.Range("last_used_after", "last_used_before").Exclusive()),
		nanorepo.FullText(table.OrganizationAPIKeys.Name, table.OrganizationAPIKeys.Description),
		nanorepo.Sort(
			table.OrganizationAPIKeys.ID, table.OrganizationAPIKeys.CreatedAt, table.OrganizationAPIKeys.Name,
			table.OrganizationAPIKeys.Status, table.OrganizationAPIKeys.LastUsedAt,
		),
		nanorepo.DefaultSort(table.OrganizationAPIKeys.CreatedAt, nanorepo.Asc),
		nanorepo.DefaultLimit(defaultPageLimit),
	)
}

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
	organizationAPIKeysBase

	db     *sql.DB
	logger *slog.Logger
	mapper *mapper.OrganizationAPIKeyMapper
	rows   *organizationAPIKeyRows
}

// organizationAPIKeyRows maps a page of API key rows with their permissions. It
// is a field of the implementation because the generated search calls a method
// of a field.
type organizationAPIKeyRows struct {
	db     *sql.DB
	mapper *mapper.OrganizationAPIKeyMapper
}

// page reads the permissions of the paged keys in one statement. The page is
// taken on keys alone, so a LIMIT never cuts a key's permissions.
func (m *organizationAPIKeyRows) page(
	ctx context.Context, rows []model.OrganizationAPIKeys,
) ([]orgapikey.OrganizationAPIKey, error) {
	if len(rows) == 0 {
		return []orgapikey.OrganizationAPIKey{}, nil
	}
	ids := functional.Slice(rows).Map(func(entity model.OrganizationAPIKeys) string { return entity.ID })
	// Every row of the page is of the organization that scoped the search.
	permissions, err := transactor.QueryMapSlice(
		ctx, m.db,
		table.OrganizationAPIKeyPermissions.SELECT(table.OrganizationAPIKeyPermissions.AllColumns).WHERE(
			table.OrganizationAPIKeyPermissions.APIKeyID.IN(jetx.ToStringExpressions(ids)...).AND(
				table.OrganizationAPIKeyPermissions.OrganizationID.EQ(postgres.String(rows[0].OrganizationID)),
			),
		),
		func(entity model.OrganizationAPIKeyPermissions) model.OrganizationAPIKeyPermissions { return entity },
	).Value()
	if err != nil {
		return nil, err
	}
	byKey := functional.Slice(permissions).GroupBy(
		func(p model.OrganizationAPIKeyPermissions) string { return p.APIKeyID },
	)
	return functional.Slice(rows).Map(func(entity model.OrganizationAPIKeys) orgapikey.OrganizationAPIKey {
		return m.mapper.ToDomainWithPermissions(entity, byKey[entity.ID])
	}), nil
}

func NewOrganizationAPIKeyRepository(
	db *sql.DB,
	mapper *mapper.OrganizationAPIKeyMapper,
	logger *slog.Logger,
) OrganizationAPIKeyRepository {
	return &organizationAPIKeyRepository{
		organizationAPIKeysBase: newOrganizationAPIKeysBase(db),
		db:                      db,
		logger:                  logger.With(slog.String("component", "organization_api_key_repository")),
		mapper:                  mapper,
		rows:                    &organizationAPIKeyRows{db: db, mapper: mapper},
	}
}

const organizationAPIKeyNameUniqueConstraint = "organization_api_keys_organization_id_name_key"

// ErrOrganizationAPIKeyNameExists is returned by Create and Update when the
// organization already has a key with that name. The service pre-check is not
// race-free: a concurrent writer's row stays invisible until it commits, so the
// unique constraint is the only race-free arbiter.
var ErrOrganizationAPIKeyNameExists = errors.New("organization API key name already exists in organization")

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

	created, err := transactor.Query[model.OrganizationAPIKeys](ctx, r.db, stmt).
		OnUnique(ErrOrganizationAPIKeyNameExists, organizationAPIKeyNameUniqueConstraint).
		Value()
	if errors.Is(err, ErrOrganizationAPIKeyNameExists) {
		return orgapikey.OrganizationAPIKey{}, err
	}
	if err != nil {
		r.logger.ErrorContext(ctx, "Failed to create organization API key",
			log.Err(err),
			slog.String("api_key_id", apiKey.ID),
			slog.String("organization_id", apiKey.OrganizationID),
		)
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

		err = transactor.Exec(ctx, r.db, permStmt).Err()
		if err != nil {
			r.logger.ErrorContext(ctx, "Failed to create organization API key permissions",
				log.Err(err),
				slog.String("api_key_id", apiKey.ID),
				slog.String("organization_id", apiKey.OrganizationID),
			)
			return orgapikey.OrganizationAPIKey{}, err
		}
	}

	return r.mapper.ToDomainWithPermissions(created, permissions), nil
}

func (r *organizationAPIKeyRepository) GetByID(
	ctx context.Context,
	organizationID, id string,
) (functional.Option[orgapikey.OrganizationAPIKey], error) {
	return r.findOne(ctx, table.OrganizationAPIKeys.ID.EQ(postgres.String(id)).AND(
		table.OrganizationAPIKeys.OrganizationID.EQ(postgres.String(organizationID)),
	))
}

func (r *organizationAPIKeyRepository) GetByIDInternal(
	ctx context.Context,
	id string,
) (functional.Option[orgapikey.OrganizationAPIKey], error) {
	return r.findOne(ctx, table.OrganizationAPIKeys.ID.EQ(postgres.String(id)))
}

func (r *organizationAPIKeyRepository) GetByOrganizationIDAndName(
	ctx context.Context,
	organizationID, name string,
) (functional.Option[orgapikey.OrganizationAPIKey], error) {
	return r.findOne(ctx, table.OrganizationAPIKeys.OrganizationID.EQ(postgres.String(organizationID)).AND(
		table.OrganizationAPIKeys.Name.EQ(postgres.String(name)),
	))
}

func (r *organizationAPIKeyRepository) GetByOrganizationIDAndHashedValue(
	ctx context.Context,
	organizationID, hashedValue string,
) (functional.Option[orgapikey.OrganizationAPIKey], error) {
	return r.findOne(ctx, table.OrganizationAPIKeys.OrganizationID.EQ(postgres.String(organizationID)).AND(
		table.OrganizationAPIKeys.HashedValue.EQ(postgres.String(hashedValue)),
	))
}

// GetByProductIDAndHashedValueInternal resolves an organization API key by its hashed
// value across all organizations within a product, deriving the organization
// from the key. It bypasses organization scope intentionally: the caller is the
// product (productApiKeyAuth) and the hash uniquely identifies a single key.
func (r *organizationAPIKeyRepository) GetByProductIDAndHashedValueInternal(
	ctx context.Context,
	productID, hashedValue string,
) (functional.Option[orgapikey.OrganizationAPIKey], error) {
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

	return transactor.QueryOptionalMap(
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

	updated, err := transactor.Query[model.OrganizationAPIKeys](ctx, r.db, stmt).
		OnUnique(ErrOrganizationAPIKeyNameExists, organizationAPIKeyNameUniqueConstraint).
		Value()
	if err != nil {
		return orgapikey.OrganizationAPIKey{}, err
	}

	permissions, err := r.getPermissionEntities(ctx, apiKey.OrganizationID, updated.ID)
	if err != nil {
		return orgapikey.OrganizationAPIKey{}, err
	}

	return r.mapper.ToDomainWithPermissions(updated, permissions), nil
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

	return transactor.Exec(ctx, r.db, stmt).Err()
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

	return transactor.Exec(ctx, r.db, stmt).Err()
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
	).Value()
}

// findOne runs the shared row-and-permissions query. Each caller passes its
// own scope in where; this helper adds none.
func (r *organizationAPIKeyRepository) findOne(
	ctx context.Context, where postgres.BoolExpression,
) (functional.Option[orgapikey.OrganizationAPIKey], error) {
	stmt := postgres.SELECT(
		table.OrganizationAPIKeys.AllColumns,
		table.OrganizationAPIKeyPermissions.AllColumns,
	).FROM(
		table.OrganizationAPIKeys.LEFT_JOIN(
			table.OrganizationAPIKeyPermissions,
			table.OrganizationAPIKeys.ID.EQ(table.OrganizationAPIKeyPermissions.APIKeyID),
		),
	).WHERE(where)

	return transactor.QueryOptionalMap(
		ctx,
		r.db,
		stmt,
		func(row organizationAPIKeyWithPermissions) orgapikey.OrganizationAPIKey {
			return r.mapper.ToDomainWithPermissions(row.OrganizationAPIKeys, row.Permissions)
		},
	)
}
