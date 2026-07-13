-- General audit log: append-only record of management-plane mutations.
-- Design doc: docs/audit-logs.md
CREATE TABLE audit_logs (
    id                 VARCHAR(255) PRIMARY KEY, -- KSUID prefix: alog_
    platform_tenant_id VARCHAR(255) NOT NULL,
    product_id         VARCHAR(255) NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    organization_id    VARCHAR(255),             -- NULL for product-level events
    action             VARCHAR(100) NOT NULL,    -- dotted resource.verb: organization.created
    outcome            VARCHAR(20)  NOT NULL DEFAULT 'SUCCESS',
    actor_type         VARCHAR(30)  NOT NULL,    -- PLATFORM_USER | PRODUCT_API_KEY | SYSTEM
    actor_id           VARCHAR(255),
    actor_name         VARCHAR(255),             -- denormalized snapshot
    target_type        VARCHAR(50)  NOT NULL,    -- organization | workspace | membership | ...
    target_id          VARCHAR(255),
    target_name        VARCHAR(255),             -- denormalized snapshot
    request_id         VARCHAR(255),
    metadata_json      JSONB,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW()
    -- No updated_at: audit logs are immutable.
);

CREATE INDEX idx_audit_logs_product_created ON audit_logs (product_id, created_at DESC);
CREATE INDEX idx_audit_logs_org_created ON audit_logs (organization_id, created_at DESC)
    WHERE organization_id IS NOT NULL;
CREATE INDEX idx_audit_logs_product_action ON audit_logs (product_id, action, created_at DESC);
CREATE INDEX idx_audit_logs_product_actor ON audit_logs (product_id, actor_id, created_at DESC);
CREATE INDEX idx_audit_logs_product_target ON audit_logs (product_id, target_type, target_id, created_at DESC);
