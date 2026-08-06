package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"anchor/internal/domain/product/role"

	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/mapper"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"
)

var _ ProductRoleRepository = (*productRoleRepositoryImpl)(nil)

type productRoleWithPermission struct {
	model.ProductRoles
	Permissions []model.ProductRoleResourcePermissions
}

type ProductRoleRepository interface {
	FindByProductIDAndRoleID(
		ctx context.Context, productID, id string,
	) (
		*role.ProductRole, error,
	)
	GetByProductIDAndName(
		ctx context.Context, productID, name string,
	) (
		*role.ProductRole, error,
	)
	Create(
		ctx context.Context, productRole role.ProductRole,
	) (
		role.ProductRole, error,
	)
	Update(
		ctx context.Context, entity role.ProductRole,
	) (role.ProductRole, error)
	DeleteByProductIDAndRoleID(
		ctx context.Context, productID string, id string,
	) error
	CountMembershipAssignments(
		ctx context.Context, roleID string,
	) (int, error)
	SearchByProductID(
		ctx context.Context, productID string,
		input search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole],
	) (search.Result[role.ProductRole], error)
}

type productRoleRepositoryImpl struct {
	db                *sql.DB
	productRoleMapper *mapper.ProductRoleMapper
	logger            zerolog.Logger
}

func NewProductRoleRepository(
	db *sql.DB,
	productRoleMapper *mapper.ProductRoleMapper,
	logger zerolog.Logger,
) ProductRoleRepository {
	return &productRoleRepositoryImpl{
		db:                db,
		productRoleMapper: productRoleMapper,
		logger:            logger,
	}
}

func productRolesUpdatableColumns() postgres.ColumnList {
	return table.ProductRoles.AllColumns.Except(
		table.ProductRoles.CreatedAt, table.ProductRoles.UpdatedAt,
	)
}

func (r *productRoleRepositoryImpl) FindByProductIDAndRoleID(
	ctx context.Context, productID, id string,
) (*role.ProductRole, error) {
	stmt := postgres.SELECT(
		table.ProductRoles.AllColumns,
		table.ProductRoleResourcePermissions.AllColumns,
	).FROM(
		table.ProductRoles.
			LEFT_JOIN(
				table.ProductRoleResourcePermissions,
				table.ProductRoles.ID.EQ(table.ProductRoleResourcePermissions.ProductRoleID),
			),
	).WHERE(
		table.ProductRoles.ID.EQ(postgres.String(id)).AND(
			table.ProductRoles.ProductID.EQ(postgres.String(productID)),
		),
	)

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt, func(permission productRoleWithPermission) role.ProductRole {
			return r.productRoleMapper.ToDomain(permission.ProductRoles, permission.Permissions)
		},
	).Value()
}

// GetByProductIDAndName looks up a role by its exact name within a product.
// Used for uniqueness checks — exact match, not the substring match the search
// filter applies.
func (r *productRoleRepositoryImpl) GetByProductIDAndName(
	ctx context.Context, productID, name string,
) (*role.ProductRole, error) {
	stmt := postgres.SELECT(
		table.ProductRoles.AllColumns,
		table.ProductRoleResourcePermissions.AllColumns,
	).FROM(
		table.ProductRoles.
			LEFT_JOIN(
				table.ProductRoleResourcePermissions,
				table.ProductRoles.ID.EQ(table.ProductRoleResourcePermissions.ProductRoleID),
			),
	).WHERE(
		postgres.LOWER(table.ProductRoles.Name).EQ(postgres.LOWER(postgres.String(name))).AND(
			table.ProductRoles.ProductID.EQ(postgres.String(productID)),
		),
	)

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt, func(permission productRoleWithPermission) role.ProductRole {
			return r.productRoleMapper.ToDomain(permission.ProductRoles, permission.Permissions)
		},
	).Value()
}

func (r *productRoleRepositoryImpl) Create(
	ctx context.Context, productRole role.ProductRole,
) (role.ProductRole, error) {
	entity := r.productRoleMapper.ToEntity(productRole)
	stmt := table.ProductRoles.INSERT(
		productRolesUpdatableColumns(),
	).MODEL(entity).RETURNING(table.ProductRoles.AllColumns)
	created, err := transactor.Query[model.ProductRoles](ctx, r.db, stmt).Value()
	if err != nil {
		return role.ProductRole{}, err
	}
	permissions := r.productRoleMapper.PermissionsToEntities(productRole.Permissions)
	if len(permissions) > 0 {
		permStmt := table.ProductRoleResourcePermissions.INSERT(
			table.ProductRoleResourcePermissions.AllColumns.Except(
				table.ProductRoleResourcePermissions.CreatedAt,
			),
		).MODELS(permissions)
		if err = transactor.Exec(ctx, r.db, permStmt).Err(); err != nil {
			r.logger.Error().Err(err).
				Str("product_role_id", productRole.ID).
				Str("product_id", productRole.ProductID).
				Msg("Failed to create product role permissions")
			return role.ProductRole{}, err
		}
	}
	return r.productRoleMapper.ToDomain(created, permissions), nil
}

func (r *productRoleRepositoryImpl) Update(
	ctx context.Context,
	domainRole role.ProductRole,
) (role.ProductRole, error) {
	domainRole.UpdatedAt = time.Now()
	entityToUpdate := r.productRoleMapper.ToEntity(domainRole)
	updateStmt := table.ProductRoles.UPDATE(
		productRolesUpdatableColumns(),
	).MODEL(
		entityToUpdate,
	).WHERE(
		table.ProductRoles.ID.EQ(postgres.String(domainRole.ID)).
			AND(table.ProductRoles.ProductID.EQ(postgres.String(domainRole.ProductID))),
	).
		RETURNING(table.ProductRoles.AllColumns)
	updated, err := transactor.Query[model.ProductRoles](ctx, r.db, updateStmt).Value()
	if err != nil {
		return role.ProductRole{}, err
	}
	// Fetch current permissions from DB
	permSelect := table.ProductRoleResourcePermissions.SELECT(
		table.ProductRoleResourcePermissions.ProductRoleID,
		table.ProductRoleResourcePermissions.ProductID,
		table.ProductRoleResourcePermissions.PermissionName,
	).WHERE(
		table.ProductRoleResourcePermissions.ProductRoleID.EQ(postgres.String(domainRole.ID)).AND(
			table.ProductRoleResourcePermissions.ProductID.EQ(postgres.String(domainRole.ProductID)),
		),
	)
	currentPerms, err := transactor.Query[[]model.ProductRoleResourcePermissions](ctx, r.db, permSelect).Value()
	if err != nil {
		return role.ProductRole{}, err
	}
	newPerms := r.productRoleMapper.PermissionsToEntities(domainRole.Permissions)
	toAdd, toRemove := r.diffRolePermissions(currentPerms, newPerms)
	if len(toRemove) > 0 {
		r.logger.Info().Str(
			"product_role_id", domainRole.ID,
		).Str(
			"product_id", domainRole.ProductID,
		).
			Msgf("Removing permissions: %v", toRemove)
		removeStmt := table.ProductRoleResourcePermissions.DELETE().WHERE(
			table.ProductRoleResourcePermissions.ProductRoleID.EQ(postgres.String(domainRole.ID)).
				AND(table.ProductRoleResourcePermissions.ProductID.EQ(postgres.String(domainRole.ProductID))).
				AND(
					table.ProductRoleResourcePermissions.PermissionName.IN(
						jetx.ToStringExpressionSliceMap(
							toRemove,
							func(perm model.ProductRoleResourcePermissions) string {
								return perm.PermissionName
							},
						)...,
					),
				),
		)
		if err = transactor.Exec(ctx, r.db, removeStmt).Err(); err != nil {
			return role.ProductRole{}, err
		}
	}
	if len(toAdd) > 0 {
		addStmt := table.ProductRoleResourcePermissions.INSERT(
			table.ProductRoleResourcePermissions.ID,
			table.ProductRoleResourcePermissions.ProductRoleID,
			table.ProductRoleResourcePermissions.ProductID,
			table.ProductRoleResourcePermissions.PermissionName,
		).MODELS(toAdd)
		if err = transactor.Exec(ctx, r.db, addStmt).Err(); err != nil {
			return role.ProductRole{}, err
		}
	}
	return r.productRoleMapper.ToDomain(updated, newPerms), nil
}

func (r *productRoleRepositoryImpl) DeleteByProductIDAndRoleID(
	ctx context.Context, productID string, id string,
) error {
	stmt := table.ProductRoles.DELETE().WHERE(
		table.ProductRoles.ID.EQ(postgres.String(id)).
			AND(table.ProductRoles.ProductID.EQ(postgres.String(productID))),
	)
	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *productRoleRepositoryImpl) CountMembershipAssignments(
	ctx context.Context, roleID string,
) (int, error) {
	orgCount, err := transactor.QueryCount(
		ctx, r.db,
		table.OrganizationMemberships.SELECT(postgres.COUNT(postgres.STAR)).WHERE(
			table.OrganizationMemberships.ProductRoleID.EQ(postgres.String(roleID)),
		),
	).Value()
	if err != nil {
		return 0, err
	}

	workspaceCount, err := transactor.QueryCount(
		ctx, r.db,
		table.WorkspaceMemberships.SELECT(postgres.COUNT(postgres.STAR)).WHERE(
			table.WorkspaceMemberships.ProductRoleID.EQ(postgres.String(roleID)),
		),
	).Value()
	if err != nil {
		return 0, err
	}

	return int(orgCount + workspaceCount), nil
}

func (r *productRoleRepositoryImpl) diffRolePermissions(
	current []model.ProductRoleResourcePermissions, newPerms []model.ProductRoleResourcePermissions,
) ([]model.ProductRoleResourcePermissions, []model.ProductRoleResourcePermissions) {
	var toAdd, toRemove []model.ProductRoleResourcePermissions
	currentMap := make(map[string]model.ProductRoleResourcePermissions)
	for _, perm := range current {
		key := perm.PermissionName
		currentMap[key] = perm
	}

	for _, perm := range newPerms {
		key := perm.PermissionName
		if _, exists := currentMap[key]; !exists {
			toAdd = append(toAdd, perm)
		} else {
			delete(currentMap, key)
		}
	}

	for _, perm := range currentMap {
		toRemove = append(toRemove, perm)
	}

	return toAdd, toRemove
}

func (r *productRoleRepositoryImpl) SearchByProductID(
	ctx context.Context, productID string,
	input search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole],
) (search.Result[role.ProductRole], error) {
	whereStmt := table.ProductRoles.ProductID.EQ(postgres.String(productID))

	if input.Filter != nil {
		if len(input.Filter.ProductRoleIDs) > 0 {
			expressions := jetx.ToStringExpressions(input.Filter.ProductRoleIDs)
			whereStmt = whereStmt.AND(table.ProductRoles.ID.IN(expressions...))
		}
		if len(input.Filter.Names) > 0 {
			nameConditions := make([]postgres.BoolExpression, len(input.Filter.Names))
			for i, name := range input.Filter.Names {
				nameConditions[i] = table.ProductRoles.Name.LIKE(postgres.String("%" + name + "%"))
			}
			whereStmt = whereStmt.AND(postgres.OR(nameConditions...))
		}
	}

	// Page over roles, not over the role⋈permissions join. Applying LIMIT/OFFSET
	// to joined rows would truncate a role's permissions to the page size (e.g. a
	// 19-permission role under LIMIT 10 would return only 10 permissions), so the
	// page is taken on roles alone and permissions are fetched separately for the
	// paged roles with no limit/offset. See product_api_key_repository.go's
	// SearchByProductID for the same pattern.
	pageResult, err := transactor.Page(
		r.db,
		func(entity model.ProductRoles) model.ProductRoles { return entity },
		table.ProductRoles.AllColumns,
	).
		From(table.ProductRoles).
		Where(whereStmt).
		OrderBy(transactor.SortColumns(
			input.Sort,
			map[role.SortFieldProductRole]postgres.Column{
				role.SortFieldProductRoleCreatedAt: table.ProductRoles.CreatedAt,
				role.SortFieldProductRoleUpdatedAt: table.ProductRoles.UpdatedAt,
				role.SortFieldProductRoleName:      table.ProductRoles.Name,
			},
		)...).
		Run(ctx, input.Pagination).
		Value()
	if err != nil {
		r.logger.Error().Err(err).Str(
			"productID", productID,
		).Msg("failed to search product roles")
		return search.Result[role.ProductRole]{}, err
	}

	if len(pageResult.Items) == 0 {
		return search.Result[role.ProductRole]{
			Items: []role.ProductRole{},
			Total: pageResult.Total,
			Count: 0,
		}, nil
	}

	pagedIDs := make([]string, len(pageResult.Items))
	for i, entity := range pageResult.Items {
		pagedIDs[i] = entity.ID
	}

	permissionEntities, err := transactor.Query[[]model.ProductRoleResourcePermissions](
		ctx, r.db,
		table.ProductRoleResourcePermissions.SELECT(table.ProductRoleResourcePermissions.AllColumns).WHERE(
			table.ProductRoleResourcePermissions.ProductRoleID.IN(jetx.ToStringExpressions(pagedIDs)...),
		),
	).Value()
	if err != nil {
		r.logger.Error().Err(err).Str(
			"productID", productID,
		).Msg("failed to load product role permissions")
		return search.Result[role.ProductRole]{}, err
	}

	permissionsByRoleID := make(map[string][]model.ProductRoleResourcePermissions, len(pageResult.Items))
	for _, permission := range permissionEntities {
		permissionsByRoleID[permission.ProductRoleID] = append(
			permissionsByRoleID[permission.ProductRoleID], permission,
		)
	}

	items := make([]role.ProductRole, len(pageResult.Items))
	for i, entity := range pageResult.Items {
		items[i] = r.productRoleMapper.ToDomain(entity, permissionsByRoleID[entity.ID])
	}

	return search.Result[role.ProductRole]{
		Items: items,
		Total: pageResult.Total,
		Count: len(items),
	}, nil
}
