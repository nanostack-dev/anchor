package service

import (
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/redis/nanoredis/cache"

	"anchor/internal/domain/product"
	"anchor/internal/domain/product/apikey"
)

// NewProductCache builds the product cache of the product service.
func NewProductCache(store cache.Store, logger *slog.Logger) (cache.Cache[product.Product], error) {
	return cache.New[product.Product](store, productCachePrefix, productCacheTTL, logger)
}

// NewProductAPIKeyCache builds the API key cache of the product API key
// service.
func NewProductAPIKeyCache(store cache.Store, logger *slog.Logger) (cache.Cache[apikey.ProductAPIKey], error) {
	return cache.New[apikey.ProductAPIKey](store, apiKeyCachePrefix, apiKeyCacheTTL, logger)
}
