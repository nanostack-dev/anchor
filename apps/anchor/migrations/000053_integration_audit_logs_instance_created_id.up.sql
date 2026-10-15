CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_integration_audit_logs_instance_created_id
    ON integration_audit_logs (integration_instance_id, created_at, id);
