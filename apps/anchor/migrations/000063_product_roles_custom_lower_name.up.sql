CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_product_roles_custom_lower_name
    ON product_roles (organization_id, lower(name)) WHERE organization_id IS NOT NULL;
