package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

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

type productWithOrganizationAPIKeyConfig struct {
	model.Products
	ProductOrganizationAPIKeyConfigs model.ProductOrganizationAPIKeyConfigs
}

func productsUpdatableColumns() postgres.ColumnList {
	return table.Products.AllColumns.Except(
		table.Products.CreatedAt, table.Products.UpdatedAt,
	)
}

type ProductRepository interface {
	FindByID(
		ctx context.Context, tenantID string, id string,
	) (*product.Product, error)
	FindByTenantIDAndName(
		ctx context.Context, tenantID string, name string,
	) (*product.Product, error)
	// FindByIDInternal returns a product by ID without tenant scoping.
	// Allowed only for trusted system-internal paths such as auth middleware
	// resolving tenant context for authenticated product API keys.
	FindByIDInternal(ctx context.Context, id string) (*product.Product, error)
	Create(ctx context.Context, prod product.Product) (
		product.Product, error,
	)
	UpsertOrganizationAPIKeyConfig(
		ctx context.Context,
		productID string,
		config product.OrganizationAPIKeysConfig,
	) error
	Update(
		ctx context.Context, tenantID string, product product.Product,
	) (product.Product, error)
	DeleteByID(ctx context.Context, tenantID string, id string) error
	SearchByTenantID(
		ctx context.Context, tenantID string,
		input search.Request[product.SearchProductFilter, product.SortFieldProduct],
	) (search.Result[product.Product], error)

	// FindAllInternal returns all products without tenant scoping.
	// Allowed only for trusted system-internal paths such as startup reconciliation.
	FindAllInternal(ctx context.Context) ([]product.Product, error)
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
	ctx context.Context, tenantID string, id string,
) (*product.Product, error) {
	stmt := postgres.SELECT(
		table.Products.AllColumns,
		table.ProductOrganizationAPIKeyConfigs.AllColumns,
	).FROM(
		table.Products.LEFT_JOIN(
			table.ProductOrganizationAPIKeyConfigs,
			table.Products.ID.EQ(table.ProductOrganizationAPIKeyConfigs.ProductID),
		),
	).WHERE(
		table.Products.ID.EQ(postgres.String(id)).AND(
			table.Products.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap[productWithOrganizationAPIKeyConfig, product.Product](
		ctx, r.db, stmt,
		func(entity productWithOrganizationAPIKeyConfig) product.Product {
			return r.productMapper.ToDomain(entity.Products, entity.ProductOrganizationAPIKeyConfigs)
		},
	)
}

func (r *productRepositoryImpl) FindByIDInternal(
	ctx context.Context, id string,
) (*product.Product, error) {
	stmt := postgres.SELECT(
		table.Products.AllColumns,
		table.ProductOrganizationAPIKeyConfigs.AllColumns,
	).FROM(
		table.Products.LEFT_JOIN(
			table.ProductOrganizationAPIKeyConfigs,
			table.Products.ID.EQ(table.ProductOrganizationAPIKeyConfigs.ProductID),
		),
	).WHERE(
		table.Products.ID.EQ(postgres.String(id)),
	).LIMIT(1)

	return transactor.QueryOptionalMap[productWithOrganizationAPIKeyConfig, product.Product](
		ctx, r.db, stmt,
		func(entity productWithOrganizationAPIKeyConfig) product.Product {
			return r.productMapper.ToDomain(entity.Products, entity.ProductOrganizationAPIKeyConfigs)
		},
	)
}

func (r *productRepositoryImpl) FindByTenantIDAndName(
	ctx context.Context, tenantID string, name string,
) (*product.Product, error) {
	stmt := postgres.SELECT(
		table.Products.AllColumns,
		table.ProductOrganizationAPIKeyConfigs.AllColumns,
	).FROM(
		table.Products.LEFT_JOIN(
			table.ProductOrganizationAPIKeyConfigs,
			table.Products.ID.EQ(table.ProductOrganizationAPIKeyConfigs.ProductID),
		),
	).WHERE(
		table.Products.PlatformTenantID.EQ(postgres.String(tenantID)).AND(
			postgres.LOWER(table.Products.Name).EQ(postgres.LOWER(postgres.String(name))),
		),
	).LIMIT(1)

	return transactor.QueryOptionalMap[productWithOrganizationAPIKeyConfig, product.Product](
		ctx, r.db, stmt,
		func(entity productWithOrganizationAPIKeyConfig) product.Product {
			return r.productMapper.ToDomain(entity.Products, entity.ProductOrganizationAPIKeyConfigs)
		},
	)
}

func (r *productRepositoryImpl) Create(
	ctx context.Context, prod product.Product,
) (product.Product, error) {
	// Name generation should be handled elsewhere
	entity := r.productMapper.ToEntity(prod)

	stmt := table.Products.INSERT(
		productsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.Products.AllColumns)

	created, err := transactor.Query[model.Products](ctx, r.db, stmt)
	if err != nil {
		return product.Product{}, err
	}

	return r.productMapper.ToDomain(
		created,
		r.productMapper.OrganizationAPIKeyConfigToEntity(
			created.ID,
			prod.Config.WithDefaults().OrganizationAPIKeys,
		),
	), nil
}

func (r *productRepositoryImpl) UpsertOrganizationAPIKeyConfig(
	ctx context.Context,
	productID string,
	config product.OrganizationAPIKeysConfig,
) error {
	entity := r.productMapper.OrganizationAPIKeyConfigToEntity(productID, config)

	stmt := table.ProductOrganizationAPIKeyConfigs.INSERT(
		table.ProductOrganizationAPIKeyConfigs.ProductID,
		table.ProductOrganizationAPIKeyConfigs.Prefix,
	).MODEL(entity).
		ON_CONFLICT(table.ProductOrganizationAPIKeyConfigs.ProductID).
		DO_UPDATE(
			postgres.SET(
				table.ProductOrganizationAPIKeyConfigs.Prefix.SET(
					table.ProductOrganizationAPIKeyConfigs.EXCLUDED.Prefix,
				),
			),
		)

	return transactor.Exec(ctx, r.db, stmt)
}

func (r *productRepositoryImpl) Update(
	ctx context.Context, tenantID string, currentProd product.Product,
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

	updated, err := transactor.Query[model.Products](ctx, r.db, updateStmt)
	if err != nil {
		return product.Product{}, err
	}

	return r.productMapper.ToDomain(
		updated,
		r.productMapper.OrganizationAPIKeyConfigToEntity(
			updated.ID,
			currentProd.Config.WithDefaults().OrganizationAPIKeys,
		),
	), nil
}

func (r *productRepositoryImpl) DeleteByID(
	ctx context.Context, tenantID string, id string,
) error {
	stmt := table.Products.DELETE().WHERE(
		table.Products.ID.EQ(postgres.String(id)).AND(
			table.Products.PlatformTenantID.EQ(postgres.String(tenantID)),
		),
	)

	return transactor.Exec(ctx, r.db, stmt)
}

func (r *productRepositoryImpl) SearchByTenantID(
	ctx context.Context, tenantID string,
	input search.Request[product.SearchProductFilter, product.SortFieldProduct],
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

	query := postgres.SELECT(
		table.Products.AllColumns,
		table.ProductOrganizationAPIKeyConfigs.AllColumns,
	).FROM(
		table.Products.LEFT_JOIN(
			table.ProductOrganizationAPIKeyConfigs,
			table.Products.ID.EQ(table.ProductOrganizationAPIKeyConfigs.ProductID),
		),
	).WHERE(whereStmt)

	resultCount, err := transactor.QueryCount(
		ctx,
		r.db,
		table.Products.SELECT(postgres.COUNT(postgres.STAR)).WHERE(whereStmt),
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
	slice, err := transactor.QueryMapSlice(
		ctx,
		r.db,
		query,
		func(entity productWithOrganizationAPIKeyConfig) product.Product {
			return r.productMapper.ToDomain(entity.Products, entity.ProductOrganizationAPIKeyConfigs)
		},
	)
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
	ctx context.Context,
) ([]product.Product, error) {
	stmt := postgres.SELECT(
		table.Products.AllColumns,
		table.ProductOrganizationAPIKeyConfigs.AllColumns,
	).FROM(
		table.Products.LEFT_JOIN(
			table.ProductOrganizationAPIKeyConfigs,
			table.Products.ID.EQ(table.ProductOrganizationAPIKeyConfigs.ProductID),
		),
	)

	return transactor.QueryMapSlice[productWithOrganizationAPIKeyConfig, product.Product](
		ctx, r.db, stmt, func(entity productWithOrganizationAPIKeyConfig) product.Product {
			return r.productMapper.ToDomain(entity.Products, entity.ProductOrganizationAPIKeyConfigs)
		},
	)
}
