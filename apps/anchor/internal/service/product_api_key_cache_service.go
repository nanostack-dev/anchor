package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nanostack-dev/nanostack-framework/modules/cache"

	"anchor/internal/domain/product/apikey"

	"github.com/rs/zerolog"
)

const (
	APIKeyCacheTTL    = 15 * time.Minute
	APIKeyCachePrefix = "product_apikey:"
)

type ProductAPIKeyCacheService interface {
	GetAPIKeyHashedValue(ctx context.Context, productID, apiKeyValue string) (
		*apikey.ProductAPIKey, error,
	)
	SetAPIKeyByHashedValue(
		ctx context.Context, productID, apiKeyValue string, apiKey *apikey.ProductAPIKey,
	) error
	GetOrElseAPIKeyHashedValue(
		ctx context.Context, productID, apiKeyValue string,
		fallback func() (*apikey.ProductAPIKey, error),
	) (*apikey.ProductAPIKey, error)
	EvictAPIKeyByHashedValue(ctx context.Context, productID, apiKeyValue string) error
	EvictAllAPIKeysForProduct(ctx context.Context, productID string) error
}

type productAPIKeyCacheService struct {
	cache  cache.Cache
	logger zerolog.Logger
}

func NewProductAPIKeyCacheService(cacheInstance cache.Cache, logger zerolog.Logger) ProductAPIKeyCacheService {
	return &productAPIKeyCacheService{
		cache:  cacheInstance,
		logger: logger.With().Str("component", "product_api_key_cache_service").Logger(),
	}
}

func (c *productAPIKeyCacheService) getAPIKeyCacheKey(productID, apiKeyValue string) string {
	return fmt.Sprintf("%s%s:%s", APIKeyCachePrefix, productID, apiKeyValue)
}

func (c *productAPIKeyCacheService) GetAPIKeyHashedValue(
	ctx context.Context, productID, apiKeyValue string,
) (*apikey.ProductAPIKey, error) {
	var apiKey apikey.ProductAPIKey
	err := c.cache.GetStruct(ctx, c.getAPIKeyCacheKey(productID, apiKeyValue), &apiKey)
	if err != nil {
		if errors.Is(err, cache.ErrCacheKeyNotFound) {
			return nil, cache.ErrCacheKeyNotFound
		}
		c.logger.Warn().Err(err).Str(
			"product_id", productID,
		).Msg("Failed to get API key from cache")
		return nil, err
	}
	return &apiKey, nil
}

func (c *productAPIKeyCacheService) SetAPIKeyByHashedValue(
	ctx context.Context, productID, apiKeyValue string, apiKey *apikey.ProductAPIKey,
) error {
	err := c.cache.SetStruct(
		ctx, c.getAPIKeyCacheKey(productID, apiKeyValue), apiKey,
		APIKeyCacheTTL,
	)
	if err != nil {
		c.logger.Warn().Err(err).Str("product_id", productID).Msg("Failed to set API key in cache")
	}
	return err
}

func (c *productAPIKeyCacheService) GetOrElseAPIKeyHashedValue(
	ctx context.Context, productID, apiKeyValue string,
	fallback func() (*apikey.ProductAPIKey, error),
) (*apikey.ProductAPIKey, error) {
	var apiKey *apikey.ProductAPIKey
	err := c.cache.GetOrElseStruct(
		ctx, c.getAPIKeyCacheKey(productID, apiKeyValue), &apiKey, func() (interface{}, error) {
			result, err := fallback()
			if err != nil {
				return nil, err
			}
			if result == nil {
				return nil, nil
			}
			return result, nil
		}, APIKeyCacheTTL,
	)

	if err != nil {
		c.logger.Warn().Err(err).Str(
			"product_id", productID,
		).Msg("Failed to get or set API key in cache")
		return nil, err
	}
	return apiKey, nil
}

func (c *productAPIKeyCacheService) EvictAPIKeyByHashedValue(
	ctx context.Context, productID, apiKeyValue string,
) error {
	err := c.cache.Evict(ctx, c.getAPIKeyCacheKey(productID, apiKeyValue))
	if err != nil {
		c.logger.Warn().Err(err).Str(
			"product_id", productID,
		).Msg("Failed to evict API key from cache")
	}
	return err
}

func (c *productAPIKeyCacheService) EvictAllAPIKeysForProduct(ctx context.Context, productID string) error {
	pattern := fmt.Sprintf("%s%s:*", APIKeyCachePrefix, productID)
	err := c.cache.EvictPattern(ctx, pattern)
	if err != nil {
		c.logger.Warn().Err(err).Str("product_id", productID).Str(
			"pattern", pattern,
		).Msg("Failed to evict API keys by pattern")
	} else {
		c.logger.Debug().Str("product_id", productID).Str(
			"pattern", pattern,
		).Msg("Evicted API key caches for product")
	}
	return err
}
