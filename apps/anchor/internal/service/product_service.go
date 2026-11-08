package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"anchor/internal/applog"

	"anchor/internal/searchx"
	"anchor/internal/security"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"

	"anchor/internal/domain/product"
	"anchor/internal/events"

	"anchor/internal/repository"
	rtservice "anchor/internal/resourcetype/service"

	"github.com/nanostack-dev/nanostack-framework/modules/cache"
)

const (
	productCacheTTL    = 30 * time.Minute
	productCachePrefix = "product"
)

type ProductService interface {
	Get(ctx context.Context, input product.GetProductInput) (product.Product, error)
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
	EventsCatalog(ctx context.Context) []events.Definition
}

type productService struct {
	productRepo           repository.ProductRepository
	productPermissionRepo repository.ProductPermissionRepository
	eventEndpoints        events.EndpointService
	resourceTypes         rtservice.Service
	products              cache.Cache[product.Product]
	logger                *slog.Logger
	transactor            transactor.Transactor
}

func NewProductService(
	productRepo repository.ProductRepository,
	productPermissionRepo repository.ProductPermissionRepository,
	eventEndpoints events.EndpointService,
	resourceTypes rtservice.Service,
	products cache.Cache[product.Product], transactor transactor.Transactor, logger *slog.Logger,
) ProductService {
	return &productService{
		productRepo:           productRepo,
		productPermissionRepo: productPermissionRepo,
		resourceTypes:         resourceTypes,
		eventEndpoints:        eventEndpoints,
		products:              products,
		logger:                logger.With(slog.String("component", "product_service")),
		transactor:            transactor,
	}
}

func (s *productService) Get(
	ctx context.Context, input product.GetProductInput,
) (product.Product, error) {
	logger := s.logger.With(slog.String("operation", "Get"))

	if err := validate.ValidateStruct(input); err != nil {
		return product.Product{}, err
	}
	found, err := s.productRepo.FindByID(ctx, input.TenantID, input.ProductID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to find product", slog.String("product_id", input.ProductID), log.Err(err))
		return product.Product{}, err
	}
	if found.IsAbsent() {
		return product.Product{}, product.ErrProductNotFound
	}
	prod := found.Value()
	s.attachEventsConfig(ctx, input.TenantID, &prod)
	return prod, nil
}

func (s *productService) GetInternal(
	ctx context.Context, productID string,
) (*product.Product, error) {
	logger := s.logger.With(slog.String("operation", "GetInternal"))

	found, err := s.productRepo.FindByIDInternal(ctx, productID)
	if err != nil {
		applog.Error(ctx, logger, err, "failed to find product internally", slog.String("product_id", productID))
		return nil, err
	}

	return found.ToPtr(), nil
}

func (s *productService) GetWithCache(
	ctx context.Context, input product.GetProductInput,
) (*product.Product, error) {
	logger := s.logger.With(slog.String("operation", "GetWithCache"))

	if err := validate.ValidateStruct(input); err != nil {
		return nil, err
	}
	cachedProduct, err := s.products.Key(input.TenantID, input.ProductID).GetOrElse(
		ctx, func() (*product.Product, error) {
			found, err := s.productRepo.FindByID(ctx, input.TenantID, input.ProductID)
			if err != nil {
				return nil, err
			}
			return found.ToPtr(), nil
		},
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to get product with cache",
			slog.String("product_id", input.ProductID),
			log.Err(err),
		)
		return nil, err
	}
	return cachedProduct, nil
}

func (s *productService) Create(
	ctx context.Context, input product.CreateProductInput,
) (product.Product, error) {
	logger := s.logger.With(slog.String("operation", "Create"))

	if err := validate.ValidateStruct(input); err != nil {
		return product.Product{}, err
	}
	config, configErr := s.normalizeConfig(input.Config)
	if configErr != nil {
		return product.Product{}, configErr
	}

	existingProduct, err := s.productRepo.FindByTenantIDAndName(ctx, input.TenantID, input.Name)
	if err != nil {
		logger.ErrorContext(ctx, "failed to look up existing product",
			slog.String("name", input.Name),
			log.Err(err),
		)
		return product.Product{}, err
	}
	if existingProduct.IsPresent() {
		logger.DebugContext(ctx, "product already exists", slog.String("name", input.Name))
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
			// A concurrent create can win the race after the FindByTenantIDAndName
			// check above passed, tripping one of the name unique guards. That is
			// the same logical condition as the pre-check, so surface the same
			// client error instead of leaking the raw DB error as a 500.
			if errors.Is(createErr, repository.ErrProductExists) {
				logger.DebugContext(txCtx, "product already exists (unique constraint)",
					slog.String("name", prod.Name),
				)
				return ErrProductAlreadyExists
			}
			logger.ErrorContext(txCtx, "failed to create product",
				slog.String("name", prod.Name),
				log.Err(createErr),
			)
			return createErr
		}
		if upsertConfigErr := s.productRepo.UpsertOrganizationAPIKeyConfig(
			txCtx,
			productCreated.ID,
			config.OrganizationAPIKeys,
		); upsertConfigErr != nil {
			logger.ErrorContext(txCtx, "failed to create product organization API key config",
				slog.String("product_id", productCreated.ID),
				log.Err(upsertConfigErr),
			)
			return upsertConfigErr
		}
		if eventsErr := s.persistEventsConfig(
			txCtx,
			input.TenantID,
			productCreated.ID,
			config.Events,
		); eventsErr != nil {
			return eventsErr
		}
		productCreated.Config = config
		s.attachEventsConfig(txCtx, input.TenantID, &productCreated)
		logger.InfoContext(txCtx, "product created",
			slog.String("product_id", productCreated.ID),
			slog.String("name", prod.Name),
		)
		for _, perm := range defaultPermissions {
			perm.ProductID = productCreated.ID
			_, permErr := s.productPermissionRepo.Create(txCtx, perm)
			if permErr != nil {
				logger.ErrorContext(txCtx, "failed to create default permission",
					slog.String("permission", perm.Name),
					log.Err(permErr),
				)
				return permErr
			}
		}
		logger.InfoContext(txCtx, "created default permissions",
			slog.String("product_id", prod.ID),
			slog.Int("permission_count", len(defaultPermissions)),
		)
		return s.resourceTypes.SeedOrganization(txCtx, productCreated.ID)
	})
	return productCreated, err
}

func (s *productService) Update(
	ctx context.Context, input product.UpdateProductInput,
) (product.Product, error) {
	logger := s.logger.With(slog.String("operation", "Update"))

	if err := validate.ValidateStruct(input); err != nil {
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
	ctx context.Context, input product.UpdateProductInput, logger *slog.Logger,
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
	if errors.Is(err, repository.ErrProductExists) {
		return product.Product{}, ErrProductAlreadyExists
	}
	if err != nil {
		logger.ErrorContext(ctx, "failed to update product",
			slog.String("product_id", input.ProductID),
			log.Err(err),
		)
		return result, err
	}
	if input.Config != nil {
		if configErr := s.productRepo.UpsertOrganizationAPIKeyConfig(
			ctx,
			input.ProductID,
			updatedProduct.Config.OrganizationAPIKeys,
		); configErr != nil {
			logger.ErrorContext(ctx, "failed to update product organization API key config",
				slog.String("product_id", input.ProductID),
				log.Err(configErr),
			)
			return product.Product{}, configErr
		}
		result.Config = updatedProduct.Config
		if eventsErr := s.persistEventsConfig(
			ctx,
			input.TenantID,
			input.ProductID,
			updatedProduct.Config.Events,
		); eventsErr != nil {
			return product.Product{}, eventsErr
		}
	}

	s.attachEventsConfig(ctx, input.TenantID, &result)
	s.evictProductFromCache(ctx, input.TenantID, input.ProductID, logger)
	logger.InfoContext(ctx, "product updated", slog.String("product_id", input.ProductID))
	return result, nil
}

func (s *productService) findProductForUpdate(
	ctx context.Context, tenantID, productID string, logger *slog.Logger,
) (*product.Product, error) {
	if err := s.productRepo.LockByID(ctx, tenantID, productID); err != nil {
		return nil, err
	}
	found, err := s.productRepo.FindByID(ctx, tenantID, productID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to find product", slog.String("product_id", productID), log.Err(err))
		return nil, err
	}
	if found.IsAbsent() {
		logger.DebugContext(ctx, "product not found for update", slog.String("product_id", productID))
		return nil, product.ErrProductNotFound
	}
	return found.ToPtr(), nil
}

func (s *productService) updateProductFields(
	ctx context.Context, input product.UpdateProductInput, prod *product.Product, logger *slog.Logger,
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
		config, err := s.normalizeConfig(product.Config{
			Protected:           functional.FromPtr(input.Config.Protected).OrElse(prod.Config.Protected),
			OrganizationAPIKeys: input.Config.OrganizationAPIKeys,
			Events:              input.Config.Events,
		})
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
		return product.Config{}, fault.BadRequest(
			"INVALID_PRODUCT_ORGANIZATION_API_KEY_PREFIX",
			"Product organization API key prefix must be 2-32 characters, start with a lowercase letter, and contain only lowercase letters, numbers, and underscores without ending in an underscore",
		).Metadata(map[string]any{
			"prefix": config.OrganizationAPIKeys.Prefix,
		})
	}

	return config, nil
}

func (s *productService) validateNameUniqueness(
	ctx context.Context, tenantID, productID, name string, logger *slog.Logger,
) error {
	found, err := s.productRepo.FindByTenantIDAndName(ctx, tenantID, name)
	if err != nil {
		logger.ErrorContext(ctx, "failed to look up product", slog.String("name", name), log.Err(err))
		return err
	}
	if found.IsPresent() && found.Value().ID != productID {
		logger.DebugContext(ctx, "product already exists",
			slog.String("name", name),
			slog.String("tenant_id", tenantID),
		)
		return ErrProductAlreadyExists
	}
	return nil
}

func (s *productService) persistEventsConfig(
	ctx context.Context, tenantID, productID string, cfg *product.EventsConfig,
) error {
	if cfg == nil {
		return nil
	}
	if strings.TrimSpace(cfg.EndpointURL) == "" {
		return s.eventEndpoints.Clear(ctx, tenantID, productID)
	}
	stored, err := s.eventEndpoints.Upsert(ctx, events.UpsertEndpointInput{
		TenantID:  tenantID,
		ProductID: productID,
		URL:       cfg.EndpointURL,
		Events:    cfg.Events,
	})
	if err != nil {
		return err
	}
	cfg.SigningSecret = stored.SigningSecretClear
	cfg.SigningSecretObfuscated = stored.SigningSecretObfuscated
	cfg.Events = stored.Events
	cfg.DeliveryStatus = stored.DeliveryStatus
	cfg.ConsecutiveFailedCalls = int(stored.ConsecutiveFailedCalls)
	return nil
}

func (s *productService) attachEventsConfig(ctx context.Context, tenantID string, prod *product.Product) {
	if prod == nil {
		return
	}
	endpoint, ok, err := s.eventEndpoints.Get(ctx, tenantID, prod.ID)
	if err != nil || !ok {
		return
	}
	if prod.Config.Events != nil && prod.Config.Events.SigningSecret != "" {
		prod.Config.Events.EndpointURL = endpoint.URL
		prod.Config.Events.SigningSecretObfuscated = endpoint.SigningSecretObfuscated
		prod.Config.Events.Events = endpoint.Events
		prod.Config.Events.DeliveryStatus = endpoint.DeliveryStatus
		prod.Config.Events.ConsecutiveFailedCalls = int(endpoint.ConsecutiveFailedCalls)
		return
	}
	prod.Config.Events = &product.EventsConfig{
		EndpointURL:             endpoint.URL,
		SigningSecretObfuscated: endpoint.SigningSecretObfuscated,
		Events:                  endpoint.Events,
		DeliveryStatus:          endpoint.DeliveryStatus,
		ConsecutiveFailedCalls:  int(endpoint.ConsecutiveFailedCalls),
	}
}

func (s *productService) EventsCatalog(_ context.Context) []events.Definition {
	if s.eventEndpoints == nil || s.eventEndpoints.Catalog() == nil {
		return []events.Definition{}
	}
	return s.eventEndpoints.Catalog().All()
}

func (s *productService) evictProductFromCache(
	ctx context.Context, tenantID, productID string, logger *slog.Logger,
) {
	if evictErr := s.products.Key(tenantID, productID).Evict(ctx); evictErr != nil {
		logger.WarnContext(ctx, "failed to evict product from cache", log.Err(evictErr))
	}
}

func (s *productService) Delete(ctx context.Context, input product.DeleteProductInput) error {
	logger := s.logger.With(slog.String("operation", "Delete"))

	if err := validate.ValidateStruct(input); err != nil {
		return err
	}

	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		prod, findErr := s.findProductForUpdate(txCtx, input.TenantID, input.ProductID, logger)
		if findErr != nil {
			return findErr
		}
		if prod.Config.Protected {
			return product.ErrProductProtected
		}
		return s.productRepo.DeleteByID(txCtx, input.TenantID, input.ProductID)
	})
	if err == nil {
		if evictErr := s.products.Key(input.TenantID, input.ProductID).Evict(ctx); evictErr != nil {
			logger.WarnContext(ctx, "failed to evict product from cache after delete", log.Err(evictErr))
		}
		logger.InfoContext(ctx, "product deleted", slog.String("product_id", input.ProductID))
	} else {
		applog.Error(ctx, logger, err, "failed to delete product", slog.String("product_id", input.ProductID))
	}
	return err
}

func (s *productService) Search(
	ctx context.Context, input product.SearchProductInput,
) (search.Result[product.Product], error) {
	logger := s.logger.With(slog.String("operation", "Search"))

	if err := input.Validate(); err != nil {
		return search.Result[product.Product]{}, err
	}

	result, err := s.productRepo.SearchByTenantID(ctx, input.TenantID, input.Request)
	if err != nil {
		if !searchx.IsClientError(err) {
			logger.ErrorContext(ctx, "failed to search products",
				slog.String("tenant_id", input.TenantID),
				log.Err(err),
			)
		}
		return search.Result[product.Product]{}, err
	}
	for i := range result.Items {
		s.attachEventsConfig(ctx, input.TenantID, &result.Items[i])
	}
	return result, nil
}
