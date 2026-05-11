
create TABLE organization_api_keys (
    id VARCHAR(255) PRIMARY KEY,
    organization_id VARCHAR(255) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    hashed_value TEXT NOT NULL,
    obfuscated_value VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, name)
);

CREATE INDEX idx_organization_api_keys_organization_id ON organization_api_keys(organization_id);
CREATE INDEX idx_organization_api_keys_status ON organization_api_keys(status);
CREATE INDEX idx_organization_api_keys_hashed_value ON organization_api_keys(hashed_value);
CREATE TRIGGER update_organization_api_keys_updated_at BEFORE UPDATE ON organization_api_keys FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE organization_api_key_permissions
(
    api_key_id      VARCHAR(255) NOT NULL REFERENCES organization_api_keys (id) ON DELETE CASCADE,
    organization_id VARCHAR(255) NOT NULL,
    product_id      VARCHAR(255) NOT NULL,
    permission_name VARCHAR(100) NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (api_key_id, organization_id, permission_name),
    FOREIGN KEY (organization_id) REFERENCES organizations (id) ON DELETE CASCADE,
    FOREIGN KEY (product_id, permission_name) REFERENCES product_permissions (product_id, name) ON DELETE CASCADE,
    PRIMARY KEY (product_id, organization_id, api_key_id, permission_name)
);

CREATE INDEX idx_organization_api_key_permissions_api_key ON organization_api_key_permissions (api_key_id);
CREATE INDEX idx_organization_api_key_permissions_org ON organization_api_key_permissions (organization_id);
CREATE INDEX idx_organization_api_key_permissions_product ON organization_api_key_permissions (product_id);
CREATE INDEX idx_organization_api_key_permissions_name ON organization_api_key_permissions (permission_name);
