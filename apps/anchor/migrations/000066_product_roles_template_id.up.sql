CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_product_roles_template_id
    ON product_roles (template_id) WHERE template_id IS NOT NULL;
