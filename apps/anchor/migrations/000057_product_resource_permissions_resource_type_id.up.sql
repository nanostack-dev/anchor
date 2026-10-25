CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_product_resource_permissions_resource_type_id
    ON product_resource_permissions (resource_type_id);
