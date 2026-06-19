# Product API key permission cache

Anchor caches each product API key together with its permissions, keyed by the
hashed key value, for **15 minutes** (`APIKeyCacheTTL` in
`internal/service/product_api_key_cache_service.go`). Auth checks the cached
permission set, so a key's scope changes are only visible once the cache entry
is refreshed.

## Granting/revoking a scope must go through the API

The cache is evicted (`EvictAPIKeyByHashedValue`) **only** when the key is
changed through anchor's own endpoints:

- `PUT /v1/products/{product_id}/api-keys/{api_key_id}` (`updateProductAPIKey`)
- `DELETE …` (`deleteProductAPIKey`)

So **always change a key's `permissions` via `updateProductAPIKey`** (full
replacement set). That evicts the cache immediately and — with a shared Redis
cache — across every instance.

Granting a scope **out of band** (editing the DB, Terraform that only manages
the permission catalog/roles, etc.) does **not** evict the cache: the running
fleet keeps serving the stale permission set and the key 403s on the new scope
for up to 15 minutes (or until anchor restarts). This is a frequent foot-gun
when adding a scope like `email:send` to an existing key.

## Cache backing (framework ≥ v0.2.9)

`nanostack-framework/modules/cache` caches **only when Redis is connected**:

- Redis reachable → shared cache; eviction propagates to all instances.
- Redis absent or unreachable → no-op cache; every instance reads fresh from the
  DB on each request (no per-instance staleness, no boot panic).

This means eviction-on-update keeps the whole fleet consistent **when Redis is
up**; when it is down there is simply no cache to be stale.
