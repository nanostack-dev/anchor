CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_product_roles_resource_type_id
    ON product_roles (resource_type_id);
