package repository

import (
	"context"
	"database/sql"
	"errors"
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

	result := transactor.QueryOptionalMap(
		ctx, r.db, stmt,
		func(entity productWithOrganizationAPIKeyConfig) product.Product {
			return r.productMapper.ToDomain(entity.Products, entity.ProductOrganizationAPIKeyConfigs)
		},
	)
	if err := result.Err(); err != nil {
		return nil, err
	}
	if !result.IsPresent() {
		return nil, nil
	}
	value := result.Value()
	return &value, nil
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

	result := transactor.QueryOptionalMap(
		ctx, r.db, stmt,
		func(entity productWithOrganizationAPIKeyConfig) product.Product {
			return r.productMapper.ToDomain(entity.Products, entity.ProductOrganizationAPIKeyConfigs)
		},
	)
	if err := result.Err(); err != nil {
		return nil, err
	}
	if !result.IsPresent() {
		return nil, nil
	}
	value := result.Value()
	return &value, nil
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

	result := transactor.QueryOptionalMap(
		ctx, r.db, stmt,
		func(entity productWithOrganizationAPIKeyConfig) product.Product {
			return r.productMapper.ToDomain(entity.Products, entity.ProductOrganizationAPIKeyConfigs)
		},
	)
	if err := result.Err(); err != nil {
		return nil, err
	}
	if !result.IsPresent() {
		return nil, nil
	}
	value := result.Value()
	return &value, nil
}

// Product names are guarded by two unique constraints, and a racing create can
// trip either one:
//   - productNameUniqueConstraint — the original UNIQUE (platform_tenant_id,
//     name) from 000001_init, violated by a byte-identical name;
//   - productNameLowerUniqueIndex — the case-insensitive index added in
//     000019_case_insensitive_names, violated by a name differing only in case.
//     That is the one the service's pre-check compares on, so it is where a race
//     most often lands.
const (
	productNameUniqueConstraint = "products_platform_tenant_id_name_key"
	productNameLowerUniqueIndex = "idx_products_tenant_lower_name_unique"
)

// ErrProductExists is returned by Create when the tenant already has a product
// with that name. The service pre-checks, but that check is not race-free: a
// concurrent insert landing between it and the INSERT trips one of the unique
// guards, and the DB is the only race-free arbiter.
var ErrProductExists = errors.New("product already exists for tenant and name")

func (r *productRepositoryImpl) Create(
	ctx context.Context, prod product.Product,
) (product.Product, error) {
	// Name generation should be handled elsewhere
	entity := r.productMapper.ToEntity(prod)

	stmt := table.Products.INSERT(
		productsUpdatableColumns(),
	).MODEL(entity).RETURNING(table.Products.AllColumns)

	created, err := transactor.Query[model.Products](ctx, r.db, stmt).
		OnUnique(ErrProductExists, productNameUniqueConstraint, productNameLowerUniqueIndex).
		Value()
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

	return transactor.Exec(ctx, r.db, stmt).Err()
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

	updated, err := transactor.Query[model.Products](ctx, r.db, updateStmt).Value()
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

	return transactor.Exec(ctx, r.db, stmt).Err()
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

	return transactor.Page(
		r.db,
		func(entity productWithOrganizationAPIKeyConfig) product.Product {
			return r.productMapper.ToDomain(entity.Products, entity.ProductOrganizationAPIKeyConfigs)
		},
		table.Products.AllColumns, table.ProductOrganizationAPIKeyConfigs.AllColumns,
	).
		From(
			table.Products.LEFT_JOIN(
				table.ProductOrganizationAPIKeyConfigs,
				table.Products.ID.EQ(table.ProductOrganizationAPIKeyConfigs.ProductID),
			),
		).
		Where(whereStmt).
		OrderBy(transactor.SortColumns(
			input.Sort,
			map[product.SortFieldProduct]postgres.Column{
				product.SortFieldProductCreatedAt: table.Products.CreatedAt,
				product.SortFieldProductUpdatedAt: table.Products.UpdatedAt,
				product.SortFieldProductName:      table.Products.Name,
			},
		)...).
		Run(ctx, input.Pagination).
		Value()
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

	return transactor.QueryMapSlice(
		ctx, r.db, stmt, func(entity productWithOrganizationAPIKeyConfig) product.Product {
			return r.productMapper.ToDomain(entity.Products, entity.ProductOrganizationAPIKeyConfigs)
		},
	).Value()
}
