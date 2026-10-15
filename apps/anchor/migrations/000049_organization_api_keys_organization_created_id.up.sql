CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_organization_api_keys_organization_created_id
    ON organization_api_keys (organization_id, created_at, id);
