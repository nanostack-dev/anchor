CREATE TABLE stripe_billing_states (
    integration_instance_id VARCHAR(255) PRIMARY KEY REFERENCES integration_instances(id) ON DELETE CASCADE,
    platform_tenant_id VARCHAR(255) NOT NULL REFERENCES platform_tenants(id) ON DELETE CASCADE,
    product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    state_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_stripe_billing_states_tenant_product ON stripe_billing_states(platform_tenant_id, product_id);
CREATE TRIGGER update_stripe_billing_states_updated_at BEFORE UPDATE ON stripe_billing_states FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
