CREATE TABLE product_workflows (
    id VARCHAR(255) PRIMARY KEY,
    product_id VARCHAR(255) NOT NULL,
    platform_tenant_id VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    trigger_event_type VARCHAR(255) NOT NULL,
    definition JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_product_workflows_product_tenant
        FOREIGN KEY (product_id, platform_tenant_id)
        REFERENCES products (id, platform_tenant_id)
        ON DELETE CASCADE
);

CREATE INDEX idx_product_workflows_trigger
    ON product_workflows (product_id, trigger_event_type);

CREATE TRIGGER update_product_workflows_updated_at
    BEFORE UPDATE ON product_workflows
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE product_workflow_runs (
    id VARCHAR(255) PRIMARY KEY,
    workflow_id VARCHAR(255) NOT NULL REFERENCES product_workflows (id) ON DELETE CASCADE,
    product_id VARCHAR(255) NOT NULL,
    event_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    event_data JSONB NOT NULL,
    trigger VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    steps JSONB NOT NULL,
    error TEXT,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_product_workflow_runs_event UNIQUE (workflow_id, event_id)
);

CREATE INDEX idx_product_workflow_runs_workflow_started
    ON product_workflow_runs (workflow_id, started_at DESC);
