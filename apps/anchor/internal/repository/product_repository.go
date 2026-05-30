package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/jetx"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/product"
	"anchor/internal/mapper"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/rs/zerolog"
)

var _ ProductRepository = (*productRepositoryImpl)(nil)

func productsUpdatableColumns() postgres.ColumnList {
	return table.Products.AllColumns.Except(
		table.Products.CreatedAt, table.Products.UpdatedAt,
	)
}

type ProductRepository interface {
	FindByID(
		ctx context.Context, tenantID string, id string, options *jetx.DBOptions,
	) (*product.Product, error)
	// FindByIDInternal returns a product by ID without tenant scoping.
	// Allowed only for trusted system-internal paths such as auth middleware
	// resolving tenant context for authenticated product API keys.
	FindByIDInternal(ctx context.Context, id string, options *jetx.DBOptions) (*product.Product, error)
	Create(ctx context.Context, prod product.Product, options *jetx.DBOptions) (
		product.Product, error,
	)
	Update(
		ctx context.Context, tenantID string, product product.Product,
		options *jetx.DBOptions,
	) (product.Product, error)
	DeleteByID(ctx context.Context, tenantID string, id string, options *jetx.DBOptions) error
	SearchByTenantID(
		ctx context.Context, tenantID string,
		input search.Request[product.SearchProductFilter, product.SortFieldProduct],
		options *jetx.DBOptions,
	) (search.Result[product.Product], error)

	// FindAllInternal returns all products without tenant scoping.
	// Allowed only for trusted system-internal paths such as startup reconciliation.
	FindAllInternal(ctx context.Context, options *jetx.DBOptions) ([]product.Product, error)
}

type productRepositoryImpl struct {
	db            *sql.DB
	productMapper *mapper.ProductMapper
	logger        zerolog.Logger
}

func NewProductRepository(db *sql.DB, productMapper *mapper.ProductMapper, logger zerolog.Logger) ProductRepository {
	return &productRepositoryImpl{
		db:            db,
		productMapper: productMapper,
		logger: logger.With().Str(
			"component", "product_repository",
		).Logger(),
	}
}

func (r *productRepositoryImpl) FindByID(
	ctx context.Context, tenantID string, id string, options *jetx.DBOptions,
) (*product.Product, error) {
	stmt := table.Products.SELECT(
		table.Products.AllColumns,
	).FROM(
		table.Products,
	).WHERE(
		table.Products.ID.EQ(postgres.String(id)).AND(
			table.Products.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	).LIMIT(1)

	return jetx.QueryOptionalMap[model.Products, product.Product](
		ctx, r.db, stmt,
		r.productMapper.ToDomain, options,
	)
}

func (r *productRepositoryImpl) FindByIDInternal(
	ctx context.Context, id string, options *jetx.DBOptions,
) (*product.Product, error) {
	stmt := table.Products.SELECT(
		table.Products.AllColumns,
	).FROM(
		table.Products,
	).WHERE(
		table.Products.ID.EQ(postgres.String(id)),
	).LIMIT(1)

	return jetx.QueryOptionalMap[model.Products, product.Product](
		ctx, r.db, stmt,
		r.productMapper.ToDomain, options,
	)
}

func (r *productRepositoryImpl) Create(
	ctx context.Context, prod product.Product, options *jetx.DBOptions,
) (product.Product, error) {
	// Name generation should be handled elsewhere
	entity := r.productMapper.ToEntity(prod)

	stmt := table.Products.INSERT(
		productsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.Products.AllColumns)

	return jetx.QueryMap[model.Products, product.Product](
		ctx, r.db, stmt, r.productMapper.ToDomain, options,
	)
}

func (r *productRepositoryImpl) Update(
	ctx context.Context, tenantID string, currentProd product.Product,
	options *jetx.DBOptions,
) (product.Product, error) {
	currentProd.UpdatedAt = time.Now()
	entityToUpdate := r.productMapper.ToEntity(currentProd)

	updateStmt := table.Products.UPDATE(
		productsUpdatableColumns().Except(
			table.Products.ID,
			table.Products.PlatformTenantID,
		),
	).MODEL(
		entityToUpdate,
	).WHERE(
		table.Products.ID.EQ(postgres.String(currentProd.ID)).AND(
			table.Products.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	).RETURNING(table.Products.AllColumns)

	return jetx.QueryMap[model.Products, product.Product](
		ctx, r.db, updateStmt, r.productMapper.ToDomain, options,
	)
}

func (r *productRepositoryImpl) DeleteByID(
	ctx context.Context, tenantID string, id string, options *jetx.DBOptions,
) error {
	stmt := table.Products.DELETE().WHERE(
		table.Products.ID.EQ(postgres.String(id)).AND(
			table.Products.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	)

	return jetx.Exec(ctx, r.db, stmt, options)
}

func (r *productRepositoryImpl) SearchByTenantID(
	ctx context.Context, tenantID string,
	input search.Request[product.SearchProductFilter, product.SortFieldProduct],
	options *jetx.DBOptions,
) (search.Result[product.Product], error) {
	whereStmt := table.Products.PlatformTenantID.EQ(postgres.String(tenantID))

	if input.Filter != nil {
		if len(input.Filter.IDs) > 0 {
			expressions := jetx.ToStringExpressions(input.Filter.IDs)
			whereStmt = whereStmt.AND(table.Products.ID.IN(expressions...))
		}

		if len(input.Filter.Names) > 0 {
			expressions := jetx.ToStringExpressions(input.Filter.Names)
			whereStmt = whereStmt.AND(table.Products.Name.IN(expressions...))
		}
	}

	if input.FullTextSearch != nil {
		whereStmt = whereStmt.AND(
			table.Products.Name.LIKE(postgres.String("%" + *input.FullTextSearch + "%")).
				OR(table.Products.Description.LIKE(postgres.String("%" + *input.FullTextSearch + "%"))),
		)
	}

	query := table.Products.SELECT(
		table.Products.AllColumns,
	).WHERE(whereStmt)

	resultCount, err := jetx.QueryCountWithBoolExpression(
		ctx, r.db, table.Products, whereStmt, options,
	)
	if err != nil {
		r.logger.Error().Err(err).Str(
			"tenantID", tenantID,
		).Msg("failed to count products")
		return search.Result[product.Product]{}, err
	}

	if input.Sort != nil {
		if len(input.Sort) > 0 {
			for _, sort := range input.Sort {
				switch sort.Field {
				case product.SortFieldProductCreatedAt:
					fieldToOrderBy := table.Products.CreatedAt
					query = query.ORDER_BY(
						jetx.OrderBy(fieldToOrderBy, sort.Direction),
					)
				case product.SortFieldProductUpdatedAt:
					fieldToOrderBy := table.Products.UpdatedAt
					query = query.ORDER_BY(
						jetx.OrderBy(fieldToOrderBy, sort.Direction),
					)
				case product.SortFieldProductName:
					fieldToOrderBy := table.Products.Name
					query = query.ORDER_BY(
						jetx.OrderBy(fieldToOrderBy, sort.Direction),
					)
				}
			}
		}
	}

	query = query.LIMIT(int64(input.Pagination.Limit)).OFFSET(int64(input.Pagination.Offset))
	slice, err := jetx.QueryMapSlice(ctx, r.db, query, r.productMapper.ToDomain, options)
	if err != nil {
		r.logger.Error().Err(err).Str(
			"tenantID", tenantID,
		).Msg("failed to search products")
		return search.Result[product.Product]{}, err
	}

	return search.Result[product.Product]{
		Items: slice,
		Total: resultCount,
		Count: len(slice),
	}, nil
}

func (r *productRepositoryImpl) FindAllInternal(
	ctx context.Context, options *jetx.DBOptions,
) ([]product.Product, error) {
	stmt := table.Products.SELECT(table.Products.AllColumns).FROM(table.Products)

	return jetx.QueryMapSlice[model.Products, product.Product](
		ctx, r.db, stmt, r.productMapper.ToDomain, options,
	)
}
