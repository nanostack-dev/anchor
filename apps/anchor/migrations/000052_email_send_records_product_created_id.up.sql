CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_email_send_records_product_created_id
    ON email_send_records (product_id, created_at, id);
