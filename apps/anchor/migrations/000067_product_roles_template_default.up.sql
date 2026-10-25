CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_product_roles_template_default
    ON product_roles (product_id) WHERE organization_id IS NULL AND is_default;
