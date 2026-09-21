CREATE TABLE product_event_endpoint_configs (
    product_id VARCHAR(255) PRIMARY KEY,
    platform_tenant_id VARCHAR(255) NOT NULL,
    endpoint_url TEXT NOT NULL,
    signing_secret TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_product_event_endpoint_configs_product_tenant
        FOREIGN KEY (product_id, platform_tenant_id)
        REFERENCES products (id, platform_tenant_id)
        ON DELETE CASCADE
);

CREATE INDEX idx_product_event_endpoint_configs_platform_tenant_id
    ON product_event_endpoint_configs (platform_tenant_id);

CREATE TRIGGER update_product_event_endpoint_configs_updated_at
    BEFORE UPDATE ON product_event_endpoint_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
