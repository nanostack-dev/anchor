CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_product_roles_organization_lower_key
    ON product_roles (organization_id, lower(key)) WHERE organization_id IS NOT NULL;
