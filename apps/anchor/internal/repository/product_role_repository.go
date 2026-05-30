package repository

import (
	"context"
	"database/sql"
	"time"

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
		ctx context.Context, productID, id string, options *jetx.DBOptions,
	) (
		*role.ProductRole, error,
	)
	Create(
		ctx context.Context, productRole role.ProductRole,
		options *jetx.DBOptions,
	) (
		role.ProductRole, error,
	)
	Update(
		ctx context.Context, entity role.ProductRole,
		options *jetx.DBOptions,
	) (role.ProductRole, error)
	DeleteByProductIDAndRoleID(
		ctx context.Context, productID string, id string, options *jetx.DBOptions,
	) error
	CountMembershipAssignments(
		ctx context.Context, roleID string, options *jetx.DBOptions,
	) (int, error)
	SearchByProductID(
		ctx context.Context, productID string,
		input search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole],
		options *jetx.DBOptions,
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
	ctx context.Context, productID, id string, options *jetx.DBOptions,
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

	return jetx.QueryOptionalMap(
		ctx, r.db, stmt, func(permission productRoleWithPermission) role.ProductRole {
			return r.productRoleMapper.ToDomain(permission.ProductRoles, permission.Permissions)
		}, options,
	)
}

func (r *productRoleRepositoryImpl) Create(
	ctx context.Context, productRole role.ProductRole, options *jetx.DBOptions,
) (role.ProductRole, error) {
	return jetx.WithTxReturn(
		jetx.Executor(ctx, r.db, options), func(tx *sql.Tx) (role.ProductRole, error) {
			entity := r.productRoleMapper.ToEntity(productRole)
			stmt := table.ProductRoles.INSERT(
				productRolesUpdatableColumns(),
			).MODEL(entity).RETURNING(table.ProductRoles.AllColumns)
			var created model.ProductRoles
			err := stmt.QueryContext(ctx, tx, &created)
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
				if err = jetx.Exec(ctx, tx, permStmt, &jetx.DBOptions{Tx: tx}); err != nil {
					r.logger.Error().Err(err).
						Str("product_role_id", productRole.ID).
						Str("product_id", productRole.ProductID).
						Msg("Failed to create product role permissions")
					return role.ProductRole{}, err
				}
			}
			return r.productRoleMapper.ToDomain(created, permissions), nil
		},
	)
}

func (r *productRoleRepositoryImpl) Update(
	ctx context.Context,
	domainRole role.ProductRole,
	options *jetx.DBOptions,
) (role.ProductRole, error) {
	return jetx.WithTxReturn(
		jetx.Executor(ctx, r.db, options), func(tx *sql.Tx) (role.ProductRole, error) {
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
			var updated model.ProductRoles
			err := updateStmt.QueryContext(ctx, tx, &updated)
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
			var currentPerms []model.ProductRoleResourcePermissions
			err = permSelect.QueryContext(ctx, tx, &currentPerms)
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
				if err = jetx.Exec(ctx, tx, removeStmt, &jetx.DBOptions{Tx: tx}); err != nil {
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
				if err = jetx.Exec(ctx, tx, addStmt, &jetx.DBOptions{Tx: tx}); err != nil {
					return role.ProductRole{}, err
				}
			}
			return r.productRoleMapper.ToDomain(updated, newPerms), nil
		},
	)
}

func (r *productRoleRepositoryImpl) DeleteByProductIDAndRoleID(
	ctx context.Context, productID string, id string, options *jetx.DBOptions,
) error {
	stmt := table.ProductRoles.DELETE().WHERE(
		table.ProductRoles.ID.EQ(postgres.String(id)).
			AND(table.ProductRoles.ProductID.EQ(postgres.String(productID))),
	)
	return jetx.Exec(ctx, r.db, stmt, options)
}

func (r *productRoleRepositoryImpl) CountMembershipAssignments(
	ctx context.Context, roleID string, options *jetx.DBOptions,
) (int, error) {
	orgCount, err := jetx.QueryCountWithBoolExpression(
		ctx,
		r.db,
		table.OrganizationMemberships,
		table.OrganizationMemberships.ProductRoleID.EQ(postgres.String(roleID)),
		options,
	)
	if err != nil {
		return 0, err
	}

	workspaceCount, err := jetx.QueryCountWithBoolExpression(
		ctx,
		r.db,
		table.WorkspaceMemberships,
		table.WorkspaceMemberships.ProductRoleID.EQ(postgres.String(roleID)),
		options,
	)
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
	options *jetx.DBOptions,
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

	query := table.ProductRoles.SELECT(
		table.ProductRoles.AllColumns,
		table.ProductRoleResourcePermissions.AllColumns,
	).FROM(
		table.ProductRoles.
			LEFT_JOIN(
				table.ProductRoleResourcePermissions,
				table.ProductRoles.ID.EQ(table.ProductRoleResourcePermissions.ProductRoleID),
			),
	).WHERE(whereStmt)

	resultCount, err := jetx.QueryCountWithBoolExpression(
		ctx, r.db, table.ProductRoles, whereStmt, options,
	)
	if err != nil {
		r.logger.Error().Err(err).Str(
			"productID", productID,
		).Msg("failed to count product roles")
		return search.Result[role.ProductRole]{}, err
	}

	if input.Sort != nil {
		if len(input.Sort) > 0 {
			for _, sort := range input.Sort {
				switch sort.Field {
				case role.SortFieldProductRoleCreatedAt:
					fieldToOrderBy := table.ProductRoles.CreatedAt
					query = query.ORDER_BY(
						jetx.OrderBy(fieldToOrderBy, sort.Direction),
					)
				case role.SortFieldProductRoleUpdatedAt:
					fieldToOrderBy := table.ProductRoles.UpdatedAt
					query = query.ORDER_BY(
						jetx.OrderBy(fieldToOrderBy, sort.Direction),
					)
				case role.SortFieldProductRoleName:
					fieldToOrderBy := table.ProductRoles.Name
					query = query.ORDER_BY(
						jetx.OrderBy(fieldToOrderBy, sort.Direction),
					)
				}
			}
		}
	}

	query = query.LIMIT(int64(input.Pagination.Limit)).OFFSET(int64(input.Pagination.Offset))

	slice, err := jetx.QueryMapSlice(
		ctx, r.db, query, func(permission productRoleWithPermission) role.ProductRole {
			return r.productRoleMapper.ToDomain(permission.ProductRoles, permission.Permissions)
		}, options,
	)
	if err != nil {
		r.logger.Error().Err(err).Str(
			"productID", productID,
		).Msg("failed to search product roles")
		return search.Result[role.ProductRole]{}, err
	}

	return search.Result[role.ProductRole]{
		Items: slice,
		Total: resultCount,
		Count: len(slice),
	}, nil
}
