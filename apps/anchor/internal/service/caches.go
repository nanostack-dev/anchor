package service

import (
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/redis/nanocache"

	"anchor/internal/domain/product"
	"anchor/internal/domain/product/apikey"
)

// NewProductCache builds the product cache of the product service.
func NewProductCache(store nanocache.Store, logger *slog.Logger) (nanocache.Cache[product.Product], error) {
	return nanocache.New[product.Product](store, productCachePrefix, productCacheTTL, logger)
}

// NewProductAPIKeyCache builds the API key cache of the product API key
// service.
func NewProductAPIKeyCache(store nanocache.Store, logger *slog.Logger) (nanocache.Cache[apikey.ProductAPIKey], error) {
	return nanocache.New[apikey.ProductAPIKey](store, apiKeyCachePrefix, apiKeyCacheTTL, logger)
}
