package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"github.com/nanostack-dev/nanostack-framework/redis/nanocache"

	"anchor/internal/domain/license"
)

// The organization license read is cached the same way the product API key
// permission cache is: keyed by (product, organization), evicted on every
// write, degrading to a no-op when Redis is unavailable. See
// docs/api-key-permission-cache.md and
// docs/adr/0012-license-status-derived-on-read.md.
//
// Usage is deliberately not part of the cached value — see
// [OrganizationLicenseService.GetLicense] — so a usage report becomes visible
// on the next read without touching this cache.
const (
	organizationLicenseCacheTTL    = 15 * time.Minute
	organizationLicenseCachePrefix = "organization_license"
)

// organizationLicenseCache holds every service that writes an Organization's
// license to one eviction discipline. Both the per-Organization writes and a
// bulk migration go through it, so neither can acquire its own key layout.
type organizationLicenseCache struct {
	entries nanocache.Cache[license.OrganizationLicense]
	logger  *slog.Logger
}

// NewOrganizationLicenseEntries builds the cache of Organization licenses
// that the licence services share.
func NewOrganizationLicenseEntries(
	store nanocache.Store, logger *slog.Logger,
) (nanocache.Cache[license.OrganizationLicense], error) {
	return nanocache.New[license.OrganizationLicense](
		store, organizationLicenseCachePrefix, organizationLicenseCacheTTL, logger,
	)
}

func newOrganizationLicenseCache(
	entries nanocache.Cache[license.OrganizationLicense], logger *slog.Logger,
) *organizationLicenseCache {
	return &organizationLicenseCache{entries: entries, logger: logger}
}

// key addresses one Organization's cached license. Product and Organization
// IDs are KSUIDs unique platform-wide, so — as with the API key cache — the
// tenant does not need to be part of the key.
func (c *organizationLicenseCache) key(
	productID, organizationID string,
) nanocache.Entry[license.OrganizationLicense] {
	return c.entries.Key(productID, organizationID)
}

// evict logs and swallows a failure, matching ProductAPIKeyService: a cache
// that cannot be evicted should not fail the write that triggered it, only
// degrade the next read.
func (c *organizationLicenseCache) evict(ctx context.Context, productID, organizationID string) {
	if err := c.key(productID, organizationID).Evict(ctx); err != nil {
		c.logger.ErrorContext(ctx, "failed to evict organization license from cache",
			slog.String("product_id", productID),
			slog.String("organization_id", organizationID),
			log.Err(err),
		)
	}
}
