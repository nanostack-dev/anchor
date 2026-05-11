-- =============================================
-- Migration 000005: Integration Base Schema
-- =============================================
-- Core tables for external provider integrations (Clerk, Auth0, etc.)
-- Supports: webhook ingress, event processing, audit trail.
-- Adds external_id to product_users for identity mapping.

-- 0. Add composite unique constraint on products for tenant-consistent FKs
-- Enables composite FK references that enforce tenant isolation at the DB level.
ALTER TABLE products ADD CONSTRAINT uq_products_id_tenant UNIQUE (id, platform_tenant_id);

-- 1. Integration Instances
-- Scoped to Product: each product has its own integration config per provider.
-- Uses composite FK (product_id, platform_tenant_id) to enforce tenant consistency:
-- prevents rows where the tenant doesn't match the referenced product's tenant.
CREATE TABLE integration_instances (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: iin_
    platform_tenant_id VARCHAR(255) NOT NULL,
    product_id VARCHAR(255) NOT NULL,
    provider_type VARCHAR(50) NOT NULL, -- 'clerk', 'auth0', etc.
    webhook_secret TEXT, -- Encrypted at application layer
    config_json JSONB NOT NULL DEFAULT '{}',
    config_version INTEGER NOT NULL DEFAULT 1,
    status VARCHAR(50) NOT NULL, -- 'ACTIVE', 'INACTIVE', 'ERROR'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (product_id, provider_type),
    CONSTRAINT chk_integration_instances_status CHECK (status IN ('ACTIVE', 'INACTIVE', 'ERROR')),
    CONSTRAINT fk_integration_instances_product_tenant
        FOREIGN KEY (product_id, platform_tenant_id)
        REFERENCES products(id, platform_tenant_id)
        ON DELETE CASCADE
);

CREATE INDEX idx_integration_instances_product_id ON integration_instances(product_id);
CREATE INDEX idx_integration_instances_platform_tenant_id ON integration_instances(platform_tenant_id);
CREATE INDEX idx_integration_instances_status ON integration_instances(status);

CREATE TRIGGER update_integration_instances_updated_at BEFORE UPDATE ON integration_instances FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 2. Integration Events
-- Raw webhook payloads stored for processing and idempotency.
CREATE TABLE integration_events (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: iev_
    integration_instance_id VARCHAR(255) NOT NULL REFERENCES integration_instances(id) ON DELETE CASCADE,
    external_event_id VARCHAR(255) NOT NULL, -- Idempotency key from provider
    event_type VARCHAR(100) NOT NULL, -- e.g. 'user.created', 'organization.updated'
    payload_json JSONB NOT NULL,
    headers_json JSONB NOT NULL,
    status VARCHAR(50) NOT NULL, -- 'PENDING', 'PROCESSING', 'PROCESSED', 'FAILED'
    error TEXT,
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (integration_instance_id, external_event_id),
    CONSTRAINT chk_integration_events_status CHECK (status IN ('PENDING', 'PROCESSING', 'PROCESSED', 'FAILED'))
);

CREATE INDEX idx_integration_events_instance_id ON integration_events(integration_instance_id);
CREATE INDEX idx_integration_events_status ON integration_events(status);
CREATE INDEX idx_integration_events_event_type ON integration_events(event_type);
CREATE INDEX idx_integration_events_processing ON integration_events(integration_instance_id, status, created_at)
    WHERE status IN ('PENDING', 'PROCESSING');

CREATE TRIGGER update_integration_events_updated_at BEFORE UPDATE ON integration_events FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 3. Add external identity column to product_users
-- Maps external provider IDs (e.g. Clerk user_id) directly on the user record.
-- Used by /me endpoint: WHERE product_id=? AND external_id=?
ALTER TABLE product_users
    ADD COLUMN external_id VARCHAR(255);

CREATE UNIQUE INDEX idx_product_users_external_lookup ON product_users(product_id, external_id)
    WHERE external_id IS NOT NULL;

-- 4. Integration Audit Logs
-- Append-only log tracking all integration actions for traceability.
CREATE TABLE integration_audit_logs (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: ial_
    integration_instance_id VARCHAR(255) NOT NULL REFERENCES integration_instances(id) ON DELETE CASCADE,
    action VARCHAR(100) NOT NULL, -- 'create', 'update', 'delete', 'sync'
    entity_type VARCHAR(50) NOT NULL, -- 'user', 'organization', 'membership'
    external_id VARCHAR(255),
    internal_id VARCHAR(255),
    diff_json JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    -- No updated_at: audit logs are immutable
);

CREATE INDEX idx_integration_audit_logs_instance_id ON integration_audit_logs(integration_instance_id);
CREATE INDEX idx_integration_audit_logs_internal_id ON integration_audit_logs(internal_id);
CREATE INDEX idx_integration_audit_logs_entity_type ON integration_audit_logs(entity_type);
