CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_email_templates_product_created_id
    ON email_templates (product_id, created_at, id);
