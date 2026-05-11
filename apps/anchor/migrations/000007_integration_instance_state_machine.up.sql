-- =============================================
-- Migration 000007: Integration Instance State Machine
-- =============================================
-- Adds explicit enable flag and error reason, plus CONFIGURING status.

ALTER TABLE integration_instances
    ADD COLUMN is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN last_error TEXT;

-- Backfill explicit enable flag from legacy INACTIVE values.
UPDATE integration_instances
SET is_enabled = FALSE
WHERE status = 'INACTIVE';

-- Replace status constraint to allow CONFIGURING transitional state.
ALTER TABLE integration_instances
    DROP CONSTRAINT IF EXISTS chk_integration_instances_status;

ALTER TABLE integration_instances
    ADD CONSTRAINT chk_integration_instances_status
        CHECK (status IN ('ACTIVE', 'INACTIVE', 'CONFIGURING', 'ERROR'));

-- Seed default reason for already-disabled instances that were never configured.
UPDATE integration_instances
SET last_error = 'Not configured yet. Complete provider setup before enabling ingestion.'
WHERE status = 'INACTIVE' AND (last_error IS NULL OR last_error = '');

CREATE INDEX idx_integration_instances_is_enabled
    ON integration_instances(is_enabled);
