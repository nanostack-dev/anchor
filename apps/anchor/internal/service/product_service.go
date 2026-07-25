package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	"anchor/internal/security"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/product"
	"anchor/internal/repository"

	"github.com/lib/pq"
	"github.com/rs/zerolog"
)

// Product names are guarded by two unique constraints, and a racing create can
// trip either one:
//   - productNameUniqueConstraint — the original UNIQUE (platform_tenant_id,
//     name) from 000001_init, violated by a byte-identical name;
//   - productNameLowerUniqueIndex — the case-insensitive index added in
//     000019_case_insensitive_names, violated by a name that differs only in
//     case. That is the one the service's own pre-check compares on (it matches
//     with LOWER), so it is the constraint a race most often lands on.
//
// A violation means a concurrent request created the product in the window
// between the service's pre-insert existence check and the INSERT — the DB is
// the only race-free arbiter of uniqueness.
const (
	productNameUniqueConstraint = "products_platform_tenant_id_name_key"
	productNameLowerUniqueIndex = "idx_products_tenant_lower_name_unique"
)

// isProductNameConflict reports whether err is the Postgres unique-violation
// (SQLSTATE 23505) raised when two concurrent creates race on the same product
// name. go-jet wraps the driver error with a "jet: " prefix, but errors.As
// still unwraps to the underlying *pq.Error. Without this translation the race
// loser's constraint violation escapes as an unhandled 500 instead of the
// intended PRODUCT_ALREADY_EXISTS client error.
func isProductNameConflict(err error) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) || pqErr.Code != "23505" {
		return false
	}
	return pqErr.Constraint == productNameUniqueConstraint ||
		pqErr.Constraint == productNameLowerUniqueIndex
}

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
	transactor            transactor.Transactor
}

func NewProductService(
	productRepo repository.ProductRepository,
	productPermissionRepo repository.ProductPermissionRepository,
	cacheService ProductCacheService, transactor transactor.Transactor, logger zerolog.Logger,
) ProductService {
	return &productService{
		productRepo:           productRepo,
		productPermissionRepo: productPermissionRepo,
		cacheService:          cacheService,
		logger:                logger.With().Str("component", "product_service").Logger(),
		transactor:            transactor,
	}
}

func (s *productService) Get(
	ctx context.Context, input product.GetProductInput,
) (*product.Product, error) {
	logger := s.logger.With().Str("operation", "Get").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}
	prod, err := s.productRepo.FindByID(ctx, input.TenantID, input.ProductID)
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

	prod, err := s.productRepo.FindByIDInternal(ctx, productID)
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

	if err := validateStruct(input); err != nil {
		return nil, err
	}
	cachedProduct, err := s.cacheService.GetOrElseProduct(
		ctx, input.TenantID, input.ProductID, func() (*product.Product, error) {
			return s.productRepo.FindByID(ctx, input.TenantID, input.ProductID)
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

	if err := validateStruct(input); err != nil {
		return product.Product{}, err
	}
	config, configErr := s.normalizeConfig(input.Config)
	if configErr != nil {
		return product.Product{}, configErr
	}

	existingProduct, err := s.productRepo.FindByTenantIDAndName(ctx, input.TenantID, input.Name)
	if err != nil {
		logger.Error().Str("name", input.Name).Err(err).Msg("failed to look up existing product")
		return product.Product{}, err
	}
	if existingProduct != nil {
		logger.Debug().Str("name", input.Name).Msg("product already exists")
		return product.Product{}, ErrProductAlreadyExists
	}

	prod := product.Product{
		PlatformTenantID: input.TenantID,
		Name:             input.Name,
		Description:      input.Description,
		Config:           config,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	prod.GenerateID()

	var productCreated product.Product
	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		defaultPermissions := GeneratePermissions()

		var createErr error
		productCreated, createErr = s.productRepo.Create(txCtx, prod)
		if createErr != nil {
			// A concurrent create can win the race after our FindByTenantIDAndName
			// check above passed, tripping the unique constraint. That is the same
			// logical condition as the pre-check (a duplicate name), so surface the
			// same client error instead of leaking the raw DB error as a 500.
			if isProductNameConflict(createErr) {
				logger.Debug().Str("name", prod.Name).Msg("product already exists (unique constraint)")
				return ErrProductAlreadyExists
			}
			logger.Error().Str("name", prod.Name).Err(createErr).Msg("failed to create product")
			return createErr
		}
		if upsertConfigErr := s.productRepo.UpsertOrganizationAPIKeyConfig(
			txCtx,
			productCreated.ID,
			config.OrganizationAPIKeys,
		); upsertConfigErr != nil {
			logger.Error().
				Str("product_id", productCreated.ID).
				Err(upsertConfigErr).
				Msg("failed to create product organization API key config")
			return upsertConfigErr
		}
		productCreated.Config = config
		logger.Info().Str("product_id", productCreated.ID).Str("name", prod.Name).Msg("product created")
		for _, perm := range defaultPermissions {
			perm.ProductID = productCreated.ID
			_, permErr := s.productPermissionRepo.Create(txCtx, perm)
			if permErr != nil {
				logger.Error().Str("permission", perm.Name).Err(permErr).Msg("failed to create default permission")
				return permErr
			}
		}
		logger.Info().
			Str("product_id", prod.ID).
			Int("permission_count", len(defaultPermissions)).
			Msg("created default permissions")
		return nil
	})
	return productCreated, err
}

func (s *productService) Update(
	ctx context.Context, input product.UpdateProductInput,
) (product.Product, error) {
	logger := s.logger.With().Str("operation", "Update").Logger()

	if err := validateStruct(input); err != nil {
		return product.Product{}, err
	}
	var updated product.Product
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		var updateErr error
		updated, updateErr = s.updateProductInTransaction(txCtx, input, logger)
		return updateErr
	})
	return updated, err
}

func (s *productService) updateProductInTransaction(
	ctx context.Context, input product.UpdateProductInput, logger zerolog.Logger,
) (product.Product, error) {
	existingProduct, err := s.findProductForUpdate(ctx, input.TenantID, input.ProductID, logger)
	if err != nil {
		return product.Product{}, err
	}

	updatedProduct := *existingProduct
	if updateErr := s.updateProductFields(ctx, input, &updatedProduct, logger); updateErr != nil {
		return product.Product{}, updateErr
	}

	result, err := s.productRepo.Update(ctx, input.TenantID, updatedProduct)
	if err != nil {
		logger.Error().Str("product_id", input.ProductID).Err(err).Msg("failed to update product")
		return result, err
	}
	if input.Config != nil {
		if configErr := s.productRepo.UpsertOrganizationAPIKeyConfig(
			ctx,
			input.ProductID,
			updatedProduct.Config.OrganizationAPIKeys,
		); configErr != nil {
			logger.Error().
				Str("product_id", input.ProductID).
				Err(configErr).
				Msg("failed to update product organization API key config")
			return product.Product{}, configErr
		}
		result.Config = updatedProduct.Config
	}

	s.evictProductFromCache(ctx, input.TenantID, input.ProductID, logger)
	logger.Info().Str("product_id", input.ProductID).Msg("product updated")
	return result, nil
}

func (s *productService) findProductForUpdate(
	ctx context.Context, tenantID, productID string, logger zerolog.Logger,
) (*product.Product, error) {
	optProduct, err := s.productRepo.FindByID(ctx, tenantID, productID)
	if err != nil {
		logger.Error().Str("product_id", productID).Err(err).Msg("failed to find product")
		return nil, err
	}
	if optProduct == nil {
		logger.Debug().Str("product_id", productID).Msg("product not found for update")
		return nil, fault.ErrNotFound
	}
	return optProduct, nil
}

func (s *productService) updateProductFields(
	ctx context.Context, input product.UpdateProductInput, prod *product.Product, logger zerolog.Logger,
) error {
	if input.Name != nil && *input.Name != prod.Name {
		if err := s.validateNameUniqueness(ctx, input.TenantID, input.ProductID, *input.Name, logger); err != nil {
			return err
		}
		prod.Name = *input.Name
	}
	if input.Description != nil {
		prod.Description = *input.Description
	}
	if input.Config != nil {
		config, err := s.normalizeConfig(*input.Config)
		if err != nil {
			return err
		}
		prod.Config = config
	}
	return nil
}

func (s *productService) normalizeConfig(config product.Config) (product.Config, error) {
	config = config.WithDefaults()
	if !security.IsValidOrganizationAPIKeyRootPrefix(config.OrganizationAPIKeys.Prefix) {
		return product.Config{}, fault.NewWithStatus(
			"INVALID_PRODUCT_ORGANIZATION_API_KEY_PREFIX",
			"Product organization API key prefix must be 2-32 characters, start with a lowercase letter, and contain only lowercase letters, numbers, and underscores without ending in an underscore",
			http.StatusBadRequest,
		)
	}

	return config, nil
}

func (s *productService) validateNameUniqueness(
	ctx context.Context, tenantID, productID, name string, logger zerolog.Logger,
) error {
	existingProduct, err := s.productRepo.FindByTenantIDAndName(ctx, tenantID, name)
	if err != nil {
		logger.Error().Str("name", name).Err(err).Msg("failed to look up product")
		return err
	}
	if existingProduct != nil && existingProduct.ID != productID {
		logger.Debug().Str("name", name).Str("tenant_id", tenantID).Msg("product already exists")
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

	if err := validateStruct(input); err != nil {
		return err
	}

	err := s.productRepo.DeleteByID(ctx, input.TenantID, input.ProductID)
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

	if err := validateStruct(input); err != nil {
		return search.Result[product.Product]{}, err
	}

	result, err := s.productRepo.SearchByTenantID(ctx, input.TenantID, input.Request)
	if err != nil {
		logger.Error().Str("tenant_id", input.TenantID).Err(err).Msg("failed to search products")
		return search.Result[product.Product]{}, err
	}
	return result, nil
}
