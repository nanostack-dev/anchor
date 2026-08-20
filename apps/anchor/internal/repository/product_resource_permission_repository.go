package repository

import (
	"context"
	"database/sql"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

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
	) (resourcepermission.ProductResourcePermission, error)

	// FindByName finds resource permission by Name
	FindByName(
		ctx context.Context, productID, id string,
	) (*resourcepermission.ProductResourcePermission, error)

	// Update updates an existing resource permission
	Update(
		ctx context.Context, perm resourcepermission.ProductResourcePermission,
	) (resourcepermission.ProductResourcePermission, error)

	// DeleteByID deletes a resource permission by Name
	DeleteByID(
		ctx context.Context, productID, id string,
	) error

	// SearchByProduct searches resource permissions within a product
	SearchByProduct(
		ctx context.Context, productID string,
		input search.Request[resourcepermission.SearchProductResourcePermissionFilter, resourcepermission.SortFieldProductResourcePermission],
	) (search.Result[resourcepermission.ProductResourcePermission], error)
	// GetByRole gets resource permissions assigned to a role
	GetByRole(
		ctx context.Context, productRoleID string,
	) ([]resourcepermission.ProductResourcePermission, error)

	// CountRoleAssignments counts how many roles use this resource permission
	CountRoleAssignments(
		ctx context.Context, productID, permissionName string,
	) (int, error)
	FindByProductIDAndPermissionNames(
		ctx context.Context, productID string, permissionNames []string,
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
) (resourcepermission.ProductResourcePermission, error) {
	entity := r.mapper.ToEntity(perm)

	stmt := table.ProductResourcePermissions.INSERT(
		productResourcePermissionsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.ProductResourcePermissions.AllColumns)

	return transactor.QueryMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	).Value()
}

func (r *productResourcePermissionRepository) FindByName(
	ctx context.Context, productID, name string,
) (*resourcepermission.ProductResourcePermission, error) {
	stmt := table.ProductResourcePermissions.SELECT(
		table.ProductResourcePermissions.AllColumns,
	).WHERE(
		table.ProductResourcePermissions.ProductID.EQ(postgres.String(productID)).
			AND(postgres.LOWER(table.ProductResourcePermissions.Name).EQ(postgres.LOWER(postgres.String(name)))),
	)

	result := transactor.QueryOptionalMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
	if err := result.Err(); err != nil {
		return nil, err
	}
	return result.ToPtr(), nil
}

func (r *productResourcePermissionRepository) Update(
	ctx context.Context, perm resourcepermission.ProductResourcePermission,
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

	return transactor.QueryMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	).Value()
}

func (r *productResourcePermissionRepository) DeleteByID(
	ctx context.Context, productID, name string,
) error {
	stmt := table.ProductResourcePermissions.DELETE().WHERE(
		table.ProductResourcePermissions.ProductID.EQ(postgres.String(productID)).
			AND(table.ProductResourcePermissions.Name.EQ(postgres.String(name))),
	)

	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *productResourcePermissionRepository) SearchByProduct(
	ctx context.Context,
	productID string,
	input search.Request[resourcepermission.SearchProductResourcePermissionFilter, resourcepermission.SortFieldProductResourcePermission],
) (search.Result[resourcepermission.ProductResourcePermission], error) {
	whereStmt := table.ProductResourcePermissions.ProductID.EQ(postgres.String(productID))

	if input.Filter != nil {
		if len(input.Filter.Names) > 0 {
			expressions := jetx.ToStringExpressions(lowerResourcePermissionStrings(input.Filter.Names))
			whereStmt = whereStmt.AND(postgres.LOWER(table.ProductResourcePermissions.Name).IN(expressions...))
		}

		if len(input.Filter.ScopeModifiers) > 0 {
			expressions := jetx.ToStringExpressions(input.Filter.ScopeModifiers)
			whereStmt = whereStmt.AND(table.ProductResourcePermissions.ScopeModifier.IN(expressions...))
		}
	}

	return transactor.Page(r.db, r.mapper.ToDomain, table.ProductResourcePermissions.AllColumns).
		From(table.ProductResourcePermissions).
		Where(whereStmt).
		OrderBy(transactor.SortColumns(
			input.Sort,
			map[resourcepermission.SortFieldProductResourcePermission]postgres.Column{
				resourcepermission.SortFieldProductResourcePermissionCreatedAt: table.ProductResourcePermissions.CreatedAt,
				resourcepermission.SortFieldProductResourcePermissionUpdatedAt: table.ProductResourcePermissions.UpdatedAt,
				resourcepermission.SortFieldProductResourcePermissionName:      table.ProductResourcePermissions.Name,
			},
		)...).
		Run(ctx, input.Pagination).
		Value()
}

func (r *productResourcePermissionRepository) GetByRole(
	ctx context.Context, productRoleID string,
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

	return transactor.QueryMapSlice(
		ctx, r.db, stmt, r.mapper.ToDomain,
	).Value()
}

func (r *productResourcePermissionRepository) CountRoleAssignments(
	ctx context.Context, productID, permissionName string,
) (int, error) {
	whereStmt := table.ProductRoleResourcePermissions.ProductID.EQ(postgres.String(productID)).
		AND(table.ProductRoleResourcePermissions.PermissionName.EQ(postgres.String(permissionName)))

	count, err := transactor.QueryCount(
		ctx,
		r.db,
		table.ProductRoleResourcePermissions.SELECT(postgres.COUNT(postgres.STAR)).WHERE(whereStmt),
	).Value()
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *productResourcePermissionRepository) FindByProductIDAndPermissionNames(
	ctx context.Context,
	productID string,
	permissionNames []string,
) ([]resourcepermission.ProductResourcePermission, error) {
	if len(permissionNames) == 0 {
		return nil, nil
	}

	stmt := table.ProductResourcePermissions.SELECT(
		table.ProductResourcePermissions.AllColumns,
	).WHERE(
		table.ProductResourcePermissions.ProductID.EQ(postgres.String(productID)).
			AND(postgres.LOWER(table.ProductResourcePermissions.Name).IN(jetx.ToStringExpressions(lowerResourcePermissionStrings(permissionNames))...)),
	)

	return transactor.QueryMapSlice(ctx, r.db, stmt, r.mapper.ToDomain).Value()
}

func lowerResourcePermissionStrings(values []string) []string {
	lowered := make([]string, len(values))
	for i, value := range values {
		lowered[i] = strings.ToLower(value)
	}
	return lowered
}
