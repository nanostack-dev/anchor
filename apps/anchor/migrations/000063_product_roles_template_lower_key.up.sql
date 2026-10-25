CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_product_roles_template_lower_key
    ON product_roles (product_id, lower(key)) WHERE organization_id IS NULL;
