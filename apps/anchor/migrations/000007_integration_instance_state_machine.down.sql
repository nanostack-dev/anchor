DROP INDEX IF EXISTS idx_integration_instances_is_enabled;

ALTER TABLE integration_instances
    DROP CONSTRAINT IF EXISTS chk_integration_instances_status;

UPDATE integration_instances
SET status = 'INACTIVE'
WHERE status = 'CONFIGURING';

ALTER TABLE integration_instances
    ADD CONSTRAINT chk_integration_instances_status
        CHECK (status IN ('ACTIVE', 'INACTIVE', 'ERROR'));

ALTER TABLE integration_instances
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS is_enabled;
