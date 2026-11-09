package repository

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/db/search"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/permission"
	"anchor/internal/mapper"
	"anchor/internal/searchx"

	"github.com/nanostack-dev/nanostack-framework/db/nanorepo"
	"github.com/nanostack-dev/nanostack-framework/db/postgres"
)

type ProductPermissionRepository interface {
	// FindByProductIDAndPermissionName finds permission by product_id and name
	FindByProductIDAndPermissionName(
		ctx context.Context, productID, name string,
	) (functional.Option[permission.ProductPermission], error)

	// Create creates a new permission
	Create(
		ctx context.Context, perm permission.ProductPermission,
	) (permission.ProductPermission, error)

	// Update updates an existing permission (by composite key)
	Update(
		ctx context.Context, perm permission.ProductPermission,
	) (permission.ProductPermission, error)

	// DeleteByProductIDAndName deletes a permission by product_id and name.
	DeleteByProductIDAndName(
		ctx context.Context, productID, name string,
	) error

	// SearchByProduct searches permissions within a product
	SearchByProduct(
		ctx context.Context, productID string,
		input search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission],
	) (search.Result[permission.ProductPermission], error)

	// CountRoleAssignments counts how many roles use this permission
	CountAPIKeyAssignments(
		ctx context.Context, productID, permissionName string,
	) (int, error)
	FindByProductIDAndPermissionNames(
		ctx context.Context, id string, permissions []string,
	) ([]permission.ProductPermission, error)
}

// productPermissionToDomain is the stateless mapper that the generated base calls.
func productPermissionToDomain(row model.ProductPermissions) permission.ProductPermission {
	return (&mapper.ProductPermissionMapper{}).ToDomain(row)
}

func _() {
	nanorepo.Implement[ProductPermissionRepository, productPermissionRepositoryImpl](
		table.ProductPermissions, productPermissionToDomain,
	)
}

func productPermissionsUpdatableColumns() postgres.ColumnList {
	return table.ProductPermissions.AllColumns.Except(
		table.ProductPermissions.CreatedAt, table.ProductPermissions.UpdatedAt,
	)
}

type productPermissionRepositoryImpl struct {
	productPermissionsBase

	db                      *sql.DB
	productPermissionMapper *mapper.ProductPermissionMapper
	logger                  *slog.Logger
}

func NewProductPermissionRepository(
	db *sql.DB,
	productPermissionMapper *mapper.ProductPermissionMapper,
	logger *slog.Logger,
) ProductPermissionRepository {
	return &productPermissionRepositoryImpl{
		productPermissionsBase:  newProductPermissionsBase(db),
		db:                      db,
		productPermissionMapper: productPermissionMapper,
		logger:                  logger.With(slog.String("component", "product_permission_repository")),
	}
}

func (r *productPermissionRepositoryImpl) FindByProductIDAndPermissionName(
	ctx context.Context, productID, name string,
) (functional.Option[permission.ProductPermission], error) {
	stmt := table.ProductPermissions.SELECT(
		table.ProductPermissions.AllColumns,
	).FROM(
		table.ProductPermissions,
	).WHERE(
		table.ProductPermissions.ProductID.EQ(postgres.String(productID)).
			AND(postgres.LOWER(table.ProductPermissions.Name).EQ(postgres.LOWER(postgres.String(name)))),
	).LIMIT(1)

	return transactor.QueryOptionalMap(
		ctx, r.db, stmt,
		r.productPermissionMapper.ToDomain,
	)
}

// The name is the second half of the primary key, so the primary key guards a
// byte-identical name and the 000019 index a name differing only in case.
const (
	productPermissionPrimaryKey           = "product_permissions_pkey"
	productPermissionNameLowerUniqueIndex = "idx_product_permissions_product_lower_name_unique"
)

// ErrProductPermissionExists is returned by Create when the product already has
// a permission with that name: the unique guards settle the race the service
// pre-check cannot.
var ErrProductPermissionExists = errors.New("product permission already exists in product")

func (r *productPermissionRepositoryImpl) Create(
	ctx context.Context, perm permission.ProductPermission,
) (permission.ProductPermission, error) {
	entity := r.productPermissionMapper.ToEntity(perm)

	stmt := table.ProductPermissions.INSERT(
		productPermissionsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.ProductPermissions.AllColumns)

	return transactor.QueryMap(
		ctx, r.db, stmt, r.productPermissionMapper.ToDomain,
	).OnUnique(ErrProductPermissionExists, productPermissionPrimaryKey, productPermissionNameLowerUniqueIndex).Value()
}

func (r *productPermissionRepositoryImpl) Update(
	ctx context.Context, perm permission.ProductPermission,
) (permission.ProductPermission, error) {
	perm.UpdatedAt = time.Now()
	entity := r.productPermissionMapper.ToEntity(perm)

	stmt := table.ProductPermissions.UPDATE(
		productPermissionsUpdatableColumns().Except(
			table.ProductPermissions.ProductID,
			table.ProductPermissions.Name,
			table.ProductPermissions.CreatedAt,
		),
	).MODEL(
		entity,
	).WHERE(
		table.ProductPermissions.ProductID.EQ(postgres.String(perm.ProductID)).
			AND(table.ProductPermissions.Name.EQ(postgres.String(perm.Name))),
	).RETURNING(table.ProductPermissions.AllColumns)

	return transactor.QueryMap(
		ctx, r.db, stmt, r.productPermissionMapper.ToDomain,
	).Value()
}

func (r *productPermissionRepositoryImpl) SearchByProduct(
	ctx context.Context, productID string,
	input search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission],
) (search.Result[permission.ProductPermission], error) {
	whereStmt := table.ProductPermissions.ProductID.EQ(postgres.String(productID))

	if input.Filter != nil {
		if len(input.Filter.Names) > 0 {
			lowerNames := functional.Slice(input.Filter.Names).Map(strings.ToLower)
			expressions := jetx.ToStringExpressions(lowerNames)
			whereStmt = whereStmt.AND(postgres.LOWER(table.ProductPermissions.Name).IN(expressions...))
		}
	}

	if input.FullTextSearch != nil {
		whereStmt = whereStmt.AND(
			searchx.Contains(
				*input.FullTextSearch,
				table.ProductPermissions.Name,
				table.ProductPermissions.Description,
			),
		)
	}

	nameKey := transactor.StringKey(
		table.ProductPermissions.Name,
		func(row model.ProductPermissions) string { return row.Name },
	)
	order, err := transactor.StrictKeys(
		input.Sort,
		map[permission.SortFieldProductPermission]transactor.Key[model.ProductPermissions]{
			permission.SortFieldProductPermissionCreatedAt: transactor.TimeKey(
				table.ProductPermissions.CreatedAt,
				func(row model.ProductPermissions) time.Time { return row.CreatedAt },
			),
			permission.SortFieldProductPermissionUpdatedAt: transactor.TimeKey(
				table.ProductPermissions.UpdatedAt,
				func(row model.ProductPermissions) time.Time { return row.UpdatedAt },
			),
			permission.SortFieldProductPermissionName:     nameKey,
			permission.SortFieldProductPermissionID:       nameKey,
			permission.SortFieldProductPermissionResource: nameKey,
		},
		nameKey,
	)
	if err != nil {
		return search.Result[permission.ProductPermission]{}, err
	}

	return transactor.Page(r.db, r.productPermissionMapper.ToDomain, table.ProductPermissions.AllColumns).
		From(table.ProductPermissions).
		Where(whereStmt).
		Keyset(order).
		Run(ctx, input.Pagination).
		Value()
}

func (r *productPermissionRepositoryImpl) CountAPIKeyAssignments(
	ctx context.Context, productID, permissionName string,
) (int, error) {
	whereStmt := table.ProductAPIKeyPermissions.ProductID.EQ(postgres.String(productID)).
		AND(table.ProductAPIKeyPermissions.PermissionName.EQ(postgres.String(permissionName)))

	count, err := transactor.QueryCount(
		ctx,
		r.db,
		table.ProductAPIKeyPermissions.SELECT(postgres.COUNT(postgres.STAR)).WHERE(whereStmt),
	).Value()
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *productPermissionRepositoryImpl) FindByProductIDAndPermissionNames(
	ctx context.Context, id string, permissions []string,
) ([]permission.ProductPermission, error) {
	if len(permissions) == 0 {
		return nil, nil
	}

	stmt := table.ProductPermissions.SELECT(
		table.ProductPermissions.AllColumns,
	).FROM(
		table.ProductPermissions,
	).WHERE(
		table.ProductPermissions.ProductID.EQ(postgres.String(id)).
			AND(postgres.LOWER(table.ProductPermissions.Name).IN(jetx.ToStringExpressions(lowerStrings(permissions))...)),
	)

	return transactor.QueryMapSlice(
		ctx, r.db, stmt, r.productPermissionMapper.ToDomain,
	).Value()
}

func lowerStrings(values []string) []string {
	return functional.Slice(values).Map(strings.ToLower)
}
