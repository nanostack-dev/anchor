package service

import (
	"context"
	"database/sql"
	"time"

	"github.com/nanostack-dev/shared/toolkit"
	"github.com/nanostack-dev/shared/toolkit/search"

	"anchor/internal/domain/product"
	"anchor/internal/repository"

	"github.com/rs/zerolog"
)

type ProductService interface {
	Get(ctx context.Context, input product.GetProductInput) (*product.Product, error)
	// GetInternal returns a product by ID without tenant scoping.
	// Allowed only for trusted internal paths such as auth middleware resolving
	// tenant context from authenticated product API keys.
	GetInternal(ctx context.Context, productID string) (*product.Product, error)
	GetWithCache(
		ctx context.Context, input product.GetProductInput,
	) (*product.Product, error)
	Create(ctx context.Context, input product.CreateProductInput) (product.Product, error)
	Update(ctx context.Context, input product.UpdateProductInput) (product.Product, error)
	Delete(ctx context.Context, input product.DeleteProductInput) error
	Search(ctx context.Context, input product.SearchProductInput) (
		search.Result[product.Product], error,
	)
}

type productService struct {
	productRepo           repository.ProductRepository
	productPermissionRepo repository.ProductPermissionRepository
	cacheService          ProductCacheService
	logger                zerolog.Logger
	db                    *sql.DB
}

func NewProductService(
	db *sql.DB, productRepo repository.ProductRepository,
	productPermissionRepo repository.ProductPermissionRepository,
	cacheService ProductCacheService, logger zerolog.Logger,
) ProductService {
	return &productService{
		productRepo:           productRepo,
		productPermissionRepo: productPermissionRepo,
		cacheService:          cacheService,
		logger:                logger.With().Str("component", "product_service").Logger(),
		db:                    db,
	}
}

func (s *productService) Get(
	ctx context.Context, input product.GetProductInput,
) (*product.Product, error) {
	logger := s.logger.With().Str("operation", "Get").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return nil, err
	}
	prod, err := s.productRepo.FindByID(ctx, input.TenantID, input.ProductID, nil)
	if err != nil {
		logger.Error().Str("product_id", input.ProductID).Err(err).Msg("failed to find product")
		return nil, err
	}
	return prod, nil
}

func (s *productService) GetInternal(
	ctx context.Context, productID string,
) (*product.Product, error) {
	logger := s.logger.With().Str("operation", "GetInternal").Logger()

	prod, err := s.productRepo.FindByIDInternal(ctx, productID, nil)
	if err != nil {
		logger.Error().Str("product_id", productID).Err(err).Msg("failed to find product internally")
		return nil, err
	}

	return prod, nil
}

func (s *productService) GetWithCache(
	ctx context.Context, input product.GetProductInput,
) (*product.Product, error) {
	logger := s.logger.With().Str("operation", "GetWithCache").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return nil, err
	}
	cachedProduct, err := s.cacheService.GetOrElseProduct(
		ctx, input.TenantID, input.ProductID, func() (*product.Product, error) {
			return s.productRepo.FindByID(ctx, input.TenantID, input.ProductID, nil)
		},
	)
	if err != nil {
		logger.Error().Str("product_id", input.ProductID).Err(err).Msg("failed to get product with cache")
		return nil, err
	}
	return cachedProduct, nil
}

func (s *productService) Create(
	ctx context.Context, input product.CreateProductInput,
) (product.Product, error) {
	logger := s.logger.With().Str("operation", "Create").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return product.Product{}, err
	}

	searchRes, err := s.Search(
		ctx, product.SearchProductInput{
			TenantID: input.TenantID,
			Request: search.Request[product.SearchProductFilter, product.SortFieldProduct]{
				Filter: &product.SearchProductFilter{
					Names: []string{input.Name},
				},
				Pagination: search.Pagination{
					Limit:  int32(1),
					Offset: int32(0),
				},
			},
		},
	)
	if err != nil {
		logger.Error().Str("name", input.Name).Err(err).Msg("failed to search for existing product")
		return product.Product{}, err
	}
	if searchRes.Count > 0 {
		logger.Error().Str("name", input.Name).Msg("product already exists")
		return product.Product{}, ErrProductAlreadyExists
	}

	prod := product.Product{
		PlatformTenantID: input.TenantID,
		Name:             input.Name,
		Description:      input.Description,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	prod.GenerateID()

	return toolkit.WithTxReturn(
		s.db, func(tx *sql.Tx) (product.Product, error) {
			defaultPermissions := GeneratePermissions()

			productCreated, createErr := s.productRepo.Create(ctx, prod, &toolkit.DBOptions{Tx: tx})
			if createErr != nil {
				logger.Error().Str("name", prod.Name).Err(createErr).Msg("failed to create product")
				return product.Product{}, createErr
			}
			logger.Info().Str("product_id", productCreated.ID).Str("name", prod.Name).Msg("product created")
			for _, perm := range defaultPermissions {
				perm.ProductID = productCreated.ID
				_, permErr := s.productPermissionRepo.Create(ctx, perm, &toolkit.DBOptions{Tx: tx})
				if permErr != nil {
					logger.Error().Str("permission", perm.Name).Err(permErr).Msg("failed to create default permission")
					return product.Product{}, permErr
				}
			}
			logger.Info().
				Str("product_id", prod.ID).
				Int("permission_count", len(defaultPermissions)).
				Msg("created default permissions")
			return productCreated, nil
		},
	)
}

func (s *productService) Update(
	ctx context.Context, input product.UpdateProductInput,
) (product.Product, error) {
	logger := s.logger.With().Str("operation", "Update").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return product.Product{}, err
	}
	return toolkit.WithTxReturn(
		s.db, func(tx *sql.Tx) (product.Product, error) {
			return s.updateProductInTransaction(ctx, input, tx, logger)
		},
	)
}

func (s *productService) updateProductInTransaction(
	ctx context.Context, input product.UpdateProductInput, tx *sql.Tx, logger zerolog.Logger,
) (product.Product, error) {
	existingProduct, err := s.findProductForUpdate(ctx, input.TenantID, input.ProductID, tx, logger)
	if err != nil {
		return product.Product{}, err
	}

	updatedProduct := *existingProduct
	if updateErr := s.updateProductFields(ctx, input, &updatedProduct, tx, logger); updateErr != nil {
		return product.Product{}, updateErr
	}

	result, err := s.productRepo.Update(
		ctx, input.TenantID, updatedProduct, &toolkit.DBOptions{Tx: tx},
	)
	if err == nil {
		s.evictProductFromCache(ctx, input.TenantID, input.ProductID, logger)
		logger.Info().Str("product_id", input.ProductID).Msg("product updated")
	} else {
		logger.Error().Str("product_id", input.ProductID).Err(err).Msg("failed to update product")
	}
	return result, err
}

func (s *productService) findProductForUpdate(
	ctx context.Context, tenantID, productID string, tx *sql.Tx, logger zerolog.Logger,
) (*product.Product, error) {
	optProduct, err := s.productRepo.FindByID(ctx, tenantID, productID, &toolkit.DBOptions{Tx: tx})
	if err != nil {
		logger.Error().Str("product_id", productID).Err(err).Msg("failed to find product")
		return nil, err
	}
	if optProduct == nil {
		logger.Error().Str("product_id", productID).Msg("product not found for update")
		return nil, toolkit.ErrNotFound
	}
	return optProduct, nil
}

func (s *productService) updateProductFields(
	ctx context.Context, input product.UpdateProductInput, prod *product.Product, tx *sql.Tx, logger zerolog.Logger,
) error {
	if input.Name != nil && *input.Name != prod.Name {
		if err := s.validateNameUniqueness(ctx, input.TenantID, *input.Name, tx, logger); err != nil {
			return err
		}
		prod.Name = *input.Name
	}
	if input.Description != nil {
		prod.Description = *input.Description
	}
	return nil
}

func (s *productService) validateNameUniqueness(
	ctx context.Context, tenantID, name string, tx *sql.Tx, logger zerolog.Logger,
) error {
	searchProduct, err := s.productRepo.SearchByTenantID(
		ctx, tenantID,
		search.Request[product.SearchProductFilter, product.SortFieldProduct]{
			Filter:     &product.SearchProductFilter{Names: []string{name}},
			Pagination: search.Pagination{Limit: int32(1), Offset: int32(0)},
		},
		&toolkit.DBOptions{Tx: tx},
	)
	if err != nil {
		logger.Error().Str("name", name).Err(err).Msg("failed to search for product")
		return err
	}
	if searchProduct.Count > 0 {
		logger.Error().Str("name", name).Str("tenant_id", tenantID).Msg("product already exists")
		return ErrProductAlreadyExists
	}
	return nil
}

func (s *productService) evictProductFromCache(
	ctx context.Context, tenantID, productID string, logger zerolog.Logger,
) {
	if evictErr := s.cacheService.EvictProduct(ctx, tenantID, productID); evictErr != nil {
		logger.Warn().Err(evictErr).Msg("failed to evict product from cache")
	}
}

func (s *productService) Delete(ctx context.Context, input product.DeleteProductInput) error {
	logger := s.logger.With().Str("operation", "Delete").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return err
	}

	err := s.productRepo.DeleteByID(ctx, input.TenantID, input.ProductID, nil)
	if err == nil {
		if evictErr := s.cacheService.EvictProduct(
			ctx, input.TenantID, input.ProductID,
		); evictErr != nil {
			logger.Warn().Err(evictErr).Msg("failed to evict product from cache after delete")
		}
		logger.Info().Str("product_id", input.ProductID).Msg("product deleted")
	} else {
		logger.Error().Str("product_id", input.ProductID).Err(err).Msg("failed to delete product")
	}
	return err
}

func (s *productService) Search(
	ctx context.Context, input product.SearchProductInput,
) (search.Result[product.Product], error) {
	logger := s.logger.With().Str("operation", "Search").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return search.Result[product.Product]{}, err
	}

	result, err := s.productRepo.SearchByTenantID(ctx, input.TenantID, input.Request, nil)
	if err != nil {
		logger.Error().Str("tenant_id", input.TenantID).Err(err).Msg("failed to search products")
		return search.Result[product.Product]{}, err
	}
	return result, nil
}
