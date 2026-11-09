package repository

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	resourcepermission "anchor/internal/domain/product/resource_permission"
	"anchor/internal/mapper"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"
)

// The base returns model rows: every method maps them with the permission mapper.
func _() {
	nanorepo.Implement[ProductResourcePermissionRepository, productResourcePermissionRepository](
		table.ProductResourcePermissions, nanorepo.NoMapper[model.ProductResourcePermissions],
	)

	// POST /v1/products/{product_id}/resource-permissions/search. The service
	// resolves the resource_types slugs of the request to type IDs and sets
	// ResourceTypeID; the API never offers that filter.
	nanorepo.Search[model.ProductResourcePermissions]("ProductResourcePermission", table.ProductResourcePermissions,
		nanorepo.Map((*mapper.ProductResourcePermissionMapper).ToDomain),
		nanorepo.Scope(table.ProductResourcePermissions.ProductID),
		nanorepo.Filter(table.ProductResourcePermissions.Name, nanorepo.InFold),
		nanorepo.GoOnly(nanorepo.Filter(table.ProductResourcePermissions.ResourceTypeID, nanorepo.In)),
		nanorepo.FullText(table.ProductResourcePermissions.Name, table.ProductResourcePermissions.Description),
		nanorepo.Sort(
			table.ProductResourcePermissions.CreatedAt, table.ProductResourcePermissions.UpdatedAt,
			table.ProductResourcePermissions.Name,
		),
		nanorepo.DefaultSort(table.ProductResourcePermissions.CreatedAt, nanorepo.Asc),
		nanorepo.DefaultLimit(defaultPageLimit),
	)
}

type ProductResourcePermissionRepository interface {
	// Create creates a new resource permission
	Create(
		ctx context.Context, perm resourcepermission.ProductResourcePermission,
	) (resourcepermission.ProductResourcePermission, error)

	// FindByName finds resource permission by Name
	FindByName(
		ctx context.Context, productID, id string,
	) (functional.Option[resourcepermission.ProductResourcePermission], error)

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
		ctx context.Context, q ProductResourcePermissionSearch,
	) (search.Result[resourcepermission.ProductResourcePermission], error)
	// GetByRole gets resource permissions assigned to a role
	GetByRole(
		ctx context.Context, productRoleID string,
	) ([]resourcepermission.ProductResourcePermission, error)

	LockByName(ctx context.Context, productID, name string) error
	// FindAssignedRoles lists the roles that bundle the permission, managed
	// and custom.
	FindAssignedRoles(
		ctx context.Context, productID, permissionName string,
	) ([]AssignedRole, error)
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
	productResourcePermissionsBase

	db     *sql.DB
	logger *slog.Logger
	mapper *mapper.ProductResourcePermissionMapper
}

func NewProductResourcePermissionRepository(
	db *sql.DB,
	mapper *mapper.ProductResourcePermissionMapper,
	logger *slog.Logger,
) ProductResourcePermissionRepository {
	return &productResourcePermissionRepository{
		productResourcePermissionsBase: newProductResourcePermissionsBase(db),
		db:                             db,
		logger:                         logger,
		mapper:                         mapper,
	}
}

// The name is the second half of the primary key, so the primary key guards a
// byte-identical name and the 000019 index a name differing only in case.
const (
	productResourcePermissionPrimaryKey           = "product_resource_permissions_pkey"
	productResourcePermissionNameLowerUniqueIndex = "idx_product_resource_permissions_product_lower_name_unique"
)

// ErrProductResourcePermissionExists is returned by Create when the product
// already has a resource permission with that name: the unique guards settle
// the race the service pre-check cannot.
var ErrProductResourcePermissionExists = errors.New("product resource permission already exists in product")

func (r *productResourcePermissionRepository) Create(
	ctx context.Context, perm resourcepermission.ProductResourcePermission,
) (resourcepermission.ProductResourcePermission, error) {
	entity := r.mapper.ToEntity(perm)

	stmt := table.ProductResourcePermissions.INSERT(
		productResourcePermissionsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.ProductResourcePermissions.AllColumns)

	return transactor.QueryMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	).OnUnique(
		ErrProductResourcePermissionExists,
		productResourcePermissionPrimaryKey,
		productResourcePermissionNameLowerUniqueIndex,
	).Value()
}

func (r *productResourcePermissionRepository) FindByName(
	ctx context.Context, productID, name string,
) (functional.Option[resourcepermission.ProductResourcePermission], error) {
	stmt := table.ProductResourcePermissions.SELECT(
		table.ProductResourcePermissions.AllColumns,
	).WHERE(
		table.ProductResourcePermissions.ProductID.EQ(postgres.String(productID)).
			AND(postgres.LOWER(table.ProductResourcePermissions.Name).EQ(postgres.LOWER(postgres.String(name)))),
	)

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt, r.mapper.ToDomain,
	)
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

func (r *productResourcePermissionRepository) LockByName(
	ctx context.Context, productID, name string,
) error {
	stmt := table.ProductResourcePermissions.
		SELECT(table.ProductResourcePermissions.Name).
		WHERE(
			table.ProductResourcePermissions.ProductID.EQ(postgres.String(productID)).
				AND(table.ProductResourcePermissions.Name.EQ(postgres.String(name))),
		).
		FOR(postgres.UPDATE())
	return transactor.Exec(ctx, r.db, stmt).Err()
}

// AssignedRole is a role that bundles a permission. OrganizationID is set on
// a custom role only.
type AssignedRole struct {
	RoleID         string
	OrganizationID string
}

func (r *productResourcePermissionRepository) FindAssignedRoles(
	ctx context.Context, productID, permissionName string,
) ([]AssignedRole, error) {
	stmt := postgres.SELECT(table.ProductRoles.ID, table.ProductRoles.OrganizationID).
		FROM(table.ProductRoleResourcePermissions.INNER_JOIN(
			table.ProductRoles, table.ProductRoles.ID.EQ(table.ProductRoleResourcePermissions.ProductRoleID),
		)).
		WHERE(
			table.ProductRoleResourcePermissions.ProductID.EQ(postgres.String(productID)).
				AND(table.ProductRoleResourcePermissions.PermissionName.EQ(postgres.String(permissionName))),
		).
		ORDER_BY(table.ProductRoles.ID)
	return transactor.QueryMapSlice(
		ctx, r.db, stmt,
		func(assigned model.ProductRoles) AssignedRole {
			return AssignedRole{
				RoleID:         assigned.ID,
				OrganizationID: functional.FromPtr(assigned.OrganizationID).OrElse(""),
			}
		},
	).Value()
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
	return functional.Slice(values).Map(strings.ToLower)
}
