CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_product_api_keys_product_created_id
    ON product_api_keys (product_id, created_at, id);
