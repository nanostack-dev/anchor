CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_product_roles_managed_lower_name
    ON product_roles (product_id, lower(name)) WHERE organization_id IS NULL;
