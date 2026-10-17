CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_product_users_product_created_id
    ON product_users (product_id, created_at, id);
