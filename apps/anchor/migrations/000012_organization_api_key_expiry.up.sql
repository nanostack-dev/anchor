ALTER TABLE organization_api_keys
    ADD COLUMN expires_at TIMESTAMPTZ;

CREATE INDEX idx_organization_api_keys_expires_at
    ON organization_api_keys(expires_at);
