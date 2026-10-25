CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_product_roles_organization_template
    ON product_roles (organization_id, template_id) WHERE template_id IS NOT NULL;
