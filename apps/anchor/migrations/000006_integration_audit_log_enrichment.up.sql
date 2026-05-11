-- =============================================
-- Migration 000006: Integration Audit Log Enrichment
-- =============================================

ALTER TABLE integration_audit_logs
    ADD COLUMN severity VARCHAR(20) NOT NULL,
    ADD COLUMN message TEXT NOT NULL,
    ADD COLUMN metadata_json JSONB;

ALTER TABLE integration_audit_logs
    ADD CONSTRAINT chk_integration_audit_logs_severity
        CHECK (severity IN ('INFO', 'SUCCESS', 'WARNING', 'ERROR'));

CREATE INDEX idx_integration_audit_logs_severity
    ON integration_audit_logs(severity);
