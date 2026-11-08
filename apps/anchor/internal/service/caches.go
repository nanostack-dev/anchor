package service

import (
	"github.com/nanostack-dev/nanostack-framework/modules/cache"
	"github.com/rs/zerolog"

	"anchor/internal/domain/product"
	"anchor/internal/domain/product/apikey"
)

// NewProductCache builds the product cache of the product service.
// modules/cache still takes a zerolog.Logger, the one the deprecated framework
// fx modules share; it goes with the move of the cache to redis/nanocache.
func NewProductCache(store cache.Store, logger zerolog.Logger) cache.Cache[product.Product] {
	return cache.New[product.Product](store, productCachePrefix, productCacheTTL, logger)
}

// NewProductAPIKeyCache builds the API key cache of the product API key
// service. See NewProductCache for the zerolog.Logger.
func NewProductAPIKeyCache(store cache.Store, logger zerolog.Logger) cache.Cache[apikey.ProductAPIKey] {
	return cache.New[apikey.ProductAPIKey](store, apiKeyCachePrefix, apiKeyCacheTTL, logger)
}
