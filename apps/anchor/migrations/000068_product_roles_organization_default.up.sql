CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_product_roles_organization_default
    ON product_roles (organization_id) WHERE organization_id IS NOT NULL AND is_default;
