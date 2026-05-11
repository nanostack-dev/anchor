package repository

import (
	"context"
	"database/sql"

	"github.com/nanostack-dev/shared/toolkit"
	"github.com/nanostack-dev/shared/toolkit/search"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	resourcepermission "anchor/internal/domain/product/resource_permission"
	"anchor/internal/mapper"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"
)

type ProductResourcePermissionRepository interface {
	// Create creates a new resource permission
	Create(
		ctx context.Context, perm resourcepermission.ProductResourcePermission,
		options *toolkit.DBOptions,
	) (resourcepermission.ProductResourcePermission, error)

	// FindByName finds resource permission by Name
	FindByName(
		ctx context.Context, productID, id string, options *toolkit.DBOptions,
	) (*resourcepermission.ProductResourcePermission, error)

	// Update updates an existing resource permission
	Update(
		ctx context.Context, perm resourcepermission.ProductResourcePermission,
		options *toolkit.DBOptions,
	) (resourcepermission.ProductResourcePermission, error)

	// DeleteByID deletes a resource permission by Name
	DeleteByID(
		ctx context.Context, productID, id string, options *toolkit.DBOptions,
	) error

	// SearchByProduct searches resource permissions within a product
	SearchByProduct(
		ctx context.Context, productID string,
		input search.Request[resourcepermission.SearchProductResourcePermissionFilter, resourcepermission.SortFieldProductResourcePermission],
		options *toolkit.DBOptions,
	) (search.Result[resourcepermission.ProductResourcePermission], error)
	// GetByRole gets resource permissions assigned to a role
	GetByRole(
		ctx context.Context, productRoleID string, options *toolkit.DBOptions,
	) ([]resourcepermission.ProductResourcePermission, error)

	// CountRoleAssignments counts how many roles use this resource permission
	CountRoleAssignments(
		ctx context.Context, productID, permissionName string, options *toolkit.DBOptions,
	) (int, error)
	FindByProductIDAndPermissionNames(
		ctx context.Context, productID string, permissionNames []string, options *toolkit.DBOptions,
	) ([]resourcepermission.ProductResourcePermission, error)
}

func productResourcePermissionsUpdatableColumns() postgres.ColumnList {
	return table.ProductResourcePermissions.AllColumns.Except(
		table.ProductPermissions.CreatedAt, table.ProductPermissions.UpdatedAt,
	)
}

type productResourcePermissionRepository struct {
	db     *sql.DB
	logger zerolog.Logger
	mapper *mapper.ProductResourcePermissionMapper
}

func NewProductResourcePermissionRepository(
	db *sql.DB,
	mapper *mapper.ProductResourcePermissionMapper,
	logger zerolog.Logger,
) ProductResourcePermissionRepository {
	return &productResourcePermissionRepository{
		db:     db,
		logger: logger,
		mapper: mapper,
	}
}

func (r *productResourcePermissionRepository) Create(
	ctx context.Context, perm resourcepermission.ProductResourcePermission,
	options *toolkit.DBOptions,
) (resourcepermission.ProductResourcePermission, error) {
	entity := r.mapper.ToEntity(perm)

	stmt := table.ProductResourcePermissions.INSERT(
		productResourcePermissionsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.ProductResourcePermissions.AllColumns)

	return toolkit.QueryMap[model.ProductResourcePermissions, resourcepermission.ProductResourcePermission](
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *productResourcePermissionRepository) FindByName(
	ctx context.Context, productID, name string, options *toolkit.DBOptions,
) (*resourcepermission.ProductResourcePermission, error) {
	stmt := table.ProductResourcePermissions.SELECT(
		table.ProductResourcePermissions.AllColumns,
	).WHERE(
		table.ProductResourcePermissions.ProductID.EQ(postgres.String(productID)).
			AND(table.ProductResourcePermissions.Name.EQ(postgres.String(name))),
	)

	return toolkit.QueryOptionalMap[model.ProductResourcePermissions, resourcepermission.ProductResourcePermission](
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *productResourcePermissionRepository) Update(
	ctx context.Context, perm resourcepermission.ProductResourcePermission,
	options *toolkit.DBOptions,
) (resourcepermission.ProductResourcePermission, error) {
	entity := r.mapper.ToEntity(perm)

	stmt := table.ProductResourcePermissions.UPDATE(
		table.ProductResourcePermissions.Name,
		table.ProductResourcePermissions.Description,
		table.ProductResourcePermissions.ScopeModifier,
	).MODEL(entity).WHERE(
		table.ProductResourcePermissions.ProductID.EQ(postgres.String(entity.ProductID)).
			AND(table.ProductResourcePermissions.Name.EQ(postgres.String(entity.Name))),
	).RETURNING(table.ProductResourcePermissions.AllColumns)

	return toolkit.QueryMap[model.ProductResourcePermissions, resourcepermission.ProductResourcePermission](
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *productResourcePermissionRepository) DeleteByID(
	ctx context.Context, productID, name string, options *toolkit.DBOptions,
) error {
	stmt := table.ProductResourcePermissions.DELETE().WHERE(
		table.ProductResourcePermissions.ProductID.EQ(postgres.String(productID)).
			AND(table.ProductResourcePermissions.Name.EQ(postgres.String(name))),
	)

	return toolkit.Exec(ctx, r.db, stmt, options)
}

func (r *productResourcePermissionRepository) SearchByProduct(
	ctx context.Context,
	productID string,
	input search.Request[resourcepermission.SearchProductResourcePermissionFilter, resourcepermission.SortFieldProductResourcePermission],
	options *toolkit.DBOptions,
) (search.Result[resourcepermission.ProductResourcePermission], error) {
	whereStmt := table.ProductResourcePermissions.ProductID.EQ(postgres.String(productID))

	if input.Filter != nil {
		if len(input.Filter.Names) > 0 {
			expressions := search.ToStringExpressions(input.Filter.Names)
			whereStmt = whereStmt.AND(table.ProductResourcePermissions.Name.IN(expressions...))
		}

		if len(input.Filter.ScopeModifiers) > 0 {
			expressions := search.ToStringExpressions(input.Filter.ScopeModifiers)
			whereStmt = whereStmt.AND(table.ProductResourcePermissions.ScopeModifier.IN(expressions...))
		}
	}

	query := table.ProductResourcePermissions.SELECT(
		table.ProductResourcePermissions.AllColumns,
	).WHERE(whereStmt)

	resultCount, err := toolkit.QueryCountWithBoolExpression(
		ctx, r.db, table.ProductResourcePermissions, whereStmt, options,
	)
	if err != nil {
		return search.Result[resourcepermission.ProductResourcePermission]{}, err
	}

	if input.Sort != nil {
		if len(input.Sort) > 0 {
			for _, sort := range input.Sort {
				switch sort.Field {
				case resourcepermission.SortFieldProductResourcePermissionCreatedAt:
					query = query.ORDER_BY(
						search.OrderBy(table.ProductResourcePermissions.CreatedAt, sort.Direction),
					)
				case resourcepermission.SortFieldProductResourcePermissionUpdatedAt:
					query = query.ORDER_BY(
						search.OrderBy(table.ProductResourcePermissions.UpdatedAt, sort.Direction),
					)
				case resourcepermission.SortFieldProductResourcePermissionName:
					query = query.ORDER_BY(
						search.OrderBy(table.ProductResourcePermissions.Name, sort.Direction),
					)
				}
			}
		}
	}

	query = query.LIMIT(int64(input.Pagination.Limit)).OFFSET(int64(input.Pagination.Offset))

	slice, err := toolkit.QueryMapSlice(
		ctx, r.db, query, r.mapper.ToDomain, options,
	)
	if err != nil {
		return search.Result[resourcepermission.ProductResourcePermission]{}, err
	}

	return search.Result[resourcepermission.ProductResourcePermission]{
		Items: slice,
		Total: resultCount,
		Count: len(slice),
	}, nil
}

func (r *productResourcePermissionRepository) GetByRole(
	ctx context.Context, productRoleID string, options *toolkit.DBOptions,
) ([]resourcepermission.ProductResourcePermission, error) {
	stmt := table.ProductResourcePermissions.SELECT(
		table.ProductResourcePermissions.AllColumns,
	).FROM(
		table.ProductResourcePermissions.
			INNER_JOIN(
				table.ProductRoleResourcePermissions,
				table.ProductResourcePermissions.Name.EQ(
					table.ProductRoleResourcePermissions.
						PermissionName,
				),
			),
	).WHERE(
		table.ProductRoleResourcePermissions.ProductRoleID.EQ(postgres.String(productRoleID)),
	).ORDER_BY(table.ProductResourcePermissions.Name.ASC())

	return toolkit.QueryMapSlice(
		ctx, r.db, stmt, r.mapper.ToDomain, options,
	)
}

func (r *productResourcePermissionRepository) CountRoleAssignments(
	ctx context.Context, productID, permissionName string, options *toolkit.DBOptions,
) (int, error) {
	whereStmt := table.ProductRoleResourcePermissions.ProductID.EQ(postgres.String(productID)).
		AND(table.ProductRoleResourcePermissions.PermissionName.EQ(postgres.String(permissionName)))

	count, err := toolkit.QueryCountWithBoolExpression(
		ctx, r.db, table.ProductRoleResourcePermissions, whereStmt, options,
	)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *productResourcePermissionRepository) FindByProductIDAndPermissionNames(
	ctx context.Context,
	productID string,
	permissionNames []string,
	options *toolkit.DBOptions,
) ([]resourcepermission.ProductResourcePermission, error) {
	if len(permissionNames) == 0 {
		return nil, nil
	}

	stmt := table.ProductResourcePermissions.SELECT(
		table.ProductResourcePermissions.AllColumns,
	).WHERE(
		table.ProductResourcePermissions.ProductID.EQ(postgres.String(productID)).
			AND(table.ProductResourcePermissions.Name.IN(search.ToStringExpressions(permissionNames)...)),
	)

	return toolkit.QueryMapSlice[
		model.ProductResourcePermissions,
		resourcepermission.ProductResourcePermission,
	](ctx, r.db, stmt, r.mapper.ToDomain, options)
}
