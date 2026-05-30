package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nanostack-dev/nanostack-framework/modules/cache"

	"anchor/internal/domain/product"

	"github.com/rs/zerolog"
)

const (
	ProductCacheTTL    = 30 * time.Minute
	ProductCachePrefix = "product:"
)

type ProductCacheService interface {
	GetProduct(ctx context.Context, tenantID, productID string) (*product.Product, error)
	SetProduct(ctx context.Context, tenantID, productID string, prod *product.Product) error
	GetOrElseProduct(
		ctx context.Context, tenantID, productID string, fallback func() (*product.Product, error),
	) (*product.Product, error)
	EvictProduct(ctx context.Context, tenantID, productID string) error
	EvictAllProductsForTenant(ctx context.Context, tenantID string) error
}

type productCacheService struct {
	cache  cache.Cache
	logger zerolog.Logger
}

func NewProductCacheService(cacheInstance cache.Cache, logger zerolog.Logger) ProductCacheService {
	return &productCacheService{
		cache:  cacheInstance,
		logger: logger.With().Str("component", "product_cache_service").Logger(),
	}
}

func (c *productCacheService) getProductCacheKey(tenantID, productID string) string {
	return fmt.Sprintf("%s%s:%s", ProductCachePrefix, tenantID, productID)
}

func (c *productCacheService) GetProduct(
	ctx context.Context, tenantID, productID string,
) (*product.Product, error) {
	var prod product.Product
	err := c.cache.GetStruct(ctx, c.getProductCacheKey(tenantID, productID), &prod)
	if err != nil {
		if errors.Is(err, cache.ErrCacheKeyNotFound) {
			return nil, cache.ErrCacheKeyNotFound
		}
		c.logger.Warn().Err(err).Str("tenant_id", tenantID).Str(
			"product_id", productID,
		).Msg("Failed to get product from cache")
		return nil, err
	}
	return &prod, nil
}

func (c *productCacheService) SetProduct(
	ctx context.Context, tenantID, productID string, prod *product.Product,
) error {
	err := c.cache.SetStruct(ctx, c.getProductCacheKey(tenantID, productID), prod, ProductCacheTTL)
	if err != nil {
		c.logger.Warn().Err(err).Str("tenant_id", tenantID).Str(
			"product_id", productID,
		).Msg("Failed to set product in cache")
	}
	return err
}

func (c *productCacheService) GetOrElseProduct(
	ctx context.Context, tenantID, productID string, fallback func() (*product.Product, error),
) (*product.Product, error) {
	var prod product.Product
	err := c.cache.GetOrElseStruct(
		ctx, c.getProductCacheKey(tenantID, productID), &prod, func() (interface{}, error) {
			result, err := fallback()
			if err != nil {
				return nil, err
			}
			if result == nil {
				return nil, cache.ErrCacheKeyNotFound
			}
			return result, nil
		}, ProductCacheTTL,
	)

	if err != nil {
		if errors.Is(err, cache.ErrCacheKeyNotFound) {
			return nil, nil //nolint:nilnil // Valid pattern for optional queries - no rows found is not an error
		}
		c.logger.Warn().Err(err).Str("tenant_id", tenantID).Str(
			"product_id", productID,
		).Msg("Failed to get product even via fallback")
		return nil, err
	}
	return &prod, nil
}

func (c *productCacheService) EvictProduct(ctx context.Context, tenantID, productID string) error {
	err := c.cache.Evict(ctx, c.getProductCacheKey(tenantID, productID))
	if err != nil {
		c.logger.Warn().Err(err).Str("tenant_id", tenantID).Str(
			"product_id", productID,
		).Msg("Failed to evict product from cache")
	}
	return err
}

func (c *productCacheService) EvictAllProductsForTenant(ctx context.Context, tenantID string) error {
	pattern := fmt.Sprintf("%s%s:*", ProductCachePrefix, tenantID)
	err := c.cache.EvictPattern(ctx, pattern)
	if err != nil {
		c.logger.Warn().Err(err).Str("tenant_id", tenantID).Str(
			"pattern", pattern,
		).Msg("Failed to evict products by pattern")
	} else {
		c.logger.Debug().Str("tenant_id", tenantID).Str(
			"pattern", pattern,
		).Msg("Evicted product caches for tenant")
	}
	return err
}
