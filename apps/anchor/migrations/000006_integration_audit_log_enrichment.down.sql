DROP INDEX IF EXISTS idx_integration_audit_logs_severity;

ALTER TABLE integration_audit_logs
    DROP CONSTRAINT IF EXISTS chk_integration_audit_logs_severity;

ALTER TABLE integration_audit_logs
    DROP COLUMN IF EXISTS metadata_json,
    DROP COLUMN IF EXISTS message,
    DROP COLUMN IF EXISTS severity;
