package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/permission"
	"anchor/internal/mapper"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"
)

type ProductPermissionRepository interface {
	// FindByProductIDAndPermissionName finds permission by product_id and name
	FindByProductIDAndPermissionName(
		ctx context.Context, productID, name string,
	) (*permission.ProductPermission, error)

	// Create creates a new permission
	Create(
		ctx context.Context, perm permission.ProductPermission,
	) (permission.ProductPermission, error)

	// Update updates an existing permission (by composite key)
	Update(
		ctx context.Context, perm permission.ProductPermission,
	) (permission.ProductPermission, error)

	// DeleteByID deletes a permission by product_id and name
	DeleteByID(
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

var _ ProductPermissionRepository = (*productPermissionRepositoryImpl)(nil)

func productPermissionsUpdatableColumns() postgres.ColumnList {
	return table.ProductPermissions.AllColumns.Except(
		table.ProductPermissions.CreatedAt, table.ProductPermissions.UpdatedAt,
	)
}

type productPermissionRepositoryImpl struct {
	db                      *sql.DB
	productPermissionMapper *mapper.ProductPermissionMapper
	logger                  zerolog.Logger
}

func NewProductPermissionRepository(
	db *sql.DB,
	productPermissionMapper *mapper.ProductPermissionMapper,
	logger zerolog.Logger,
) ProductPermissionRepository {
	return &productPermissionRepositoryImpl{
		db:                      db,
		productPermissionMapper: productPermissionMapper,
		logger: logger.With().Str(
			"component", "product_permission_repository",
		).Logger(),
	}
}

func (r *productPermissionRepositoryImpl) FindByProductIDAndPermissionName(
	ctx context.Context, productID, name string,
) (*permission.ProductPermission, error) {
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
	).Value()
}

func (r *productPermissionRepositoryImpl) Create(
	ctx context.Context, perm permission.ProductPermission,
) (permission.ProductPermission, error) {
	entity := r.productPermissionMapper.ToEntity(perm)

	stmt := table.ProductPermissions.INSERT(
		productPermissionsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.ProductPermissions.AllColumns)

	return transactor.QueryMap(
		ctx, r.db, stmt, r.productPermissionMapper.ToDomain,
	).Value()
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

func (r *productPermissionRepositoryImpl) DeleteByID(
	ctx context.Context, productID, name string,
) error {
	stmt := table.ProductPermissions.DELETE().WHERE(
		table.ProductPermissions.ProductID.EQ(postgres.String(productID)).
			AND(table.ProductPermissions.Name.EQ(postgres.String(name))),
	)

	return transactor.Exec(ctx, r.db, stmt).Err()
}

func (r *productPermissionRepositoryImpl) SearchByProduct(
	ctx context.Context, productID string,
	input search.Request[permission.SearchProductPermissionFilter, permission.SortFieldProductPermission],
) (search.Result[permission.ProductPermission], error) {
	whereStmt := table.ProductPermissions.ProductID.EQ(postgres.String(productID))

	if input.Filter != nil {
		if len(input.Filter.Names) > 0 {
			lowerNames := make([]string, len(input.Filter.Names))
			for i, name := range input.Filter.Names {
				lowerNames[i] = strings.ToLower(name)
			}
			expressions := jetx.ToStringExpressions(lowerNames)
			whereStmt = whereStmt.AND(postgres.LOWER(table.ProductPermissions.Name).IN(expressions...))
		}
	}

	if input.FullTextSearch != nil {
		whereStmt = whereStmt.AND(
			table.ProductPermissions.Name.LIKE(postgres.String("%" + *input.FullTextSearch + "%")).
				OR(table.ProductPermissions.Description.LIKE(postgres.String("%" + *input.FullTextSearch + "%"))),
		)
	}

	query := table.ProductPermissions.SELECT(
		table.ProductPermissions.AllColumns,
	).WHERE(whereStmt)

	resultCount, err := transactor.QueryCount(
		ctx,
		r.db,
		table.ProductPermissions.SELECT(postgres.COUNT(postgres.STAR)).WHERE(whereStmt),
	).Value()
	if err != nil {
		r.logger.Error().Err(err).Str(
			"productID", productID,
		).Msg("failed to count product permissions")
		return search.Result[permission.ProductPermission]{}, err
	}
	if input.Sort != nil {
		if len(input.Sort) > 0 {
			for _, sort := range input.Sort {
				switch sort.Field {
				case permission.SortFieldProductPermissionCreatedAt:
					fieldToOrderBy := table.ProductPermissions.CreatedAt
					query = query.ORDER_BY(
						jetx.OrderBy(fieldToOrderBy, sort.Direction),
					)
				case permission.SortFieldProductPermissionUpdatedAt:
					fieldToOrderBy := table.ProductPermissions.UpdatedAt
					query = query.ORDER_BY(
						jetx.OrderBy(fieldToOrderBy, sort.Direction),
					)
				case permission.SortFieldProductPermissionName:
					fieldToOrderBy := table.ProductPermissions.Name
					query = query.ORDER_BY(
						jetx.OrderBy(fieldToOrderBy, sort.Direction),
					)
				}
			}
		}
	}

	query = query.LIMIT(int64(input.Pagination.Limit)).OFFSET(int64(input.Pagination.Offset))

	slice, err := transactor.QueryMapSlice(
		ctx, r.db, query, r.productPermissionMapper.ToDomain,
	).Value()
	if err != nil {
		r.logger.Error().Err(err).Str(
			"productID", productID,
		).Msg("failed to search product permissions")
		return search.Result[permission.ProductPermission]{}, err
	}

	return search.Result[permission.ProductPermission]{
		Items: slice,
		Total: resultCount,
		Count: len(slice),
	}, nil
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
	lowered := make([]string, len(values))
	for i, value := range values {
		lowered[i] = strings.ToLower(value)
	}
	return lowered
}
