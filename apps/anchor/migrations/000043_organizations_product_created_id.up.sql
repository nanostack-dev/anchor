CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_organizations_product_created_id
    ON organizations (product_id, created_at, id);
