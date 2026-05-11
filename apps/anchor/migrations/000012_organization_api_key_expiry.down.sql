DROP INDEX IF EXISTS idx_organization_api_keys_expires_at;

ALTER TABLE organization_api_keys
    DROP COLUMN IF EXISTS expires_at;
