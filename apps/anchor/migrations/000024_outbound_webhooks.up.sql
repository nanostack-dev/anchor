-- =============================================
-- Migration 000024: Outbound Webhooks
-- =============================================
-- Product-scoped outbound webhook subscriptions, the transactional outbox that
-- feeds them, and the delivery log.
--
-- Business rules (status enums, event-type grammar, subscription matching,
-- retry ladder, auto-disable thresholds, SSRF policy) live in the service and
-- domain layers. The DB keeps PK/FK/UNIQUE/NOT NULL/defaults and the shared
-- updated_at trigger only.

-- 1. Endpoints
-- One row per product-scoped subscription. `event_types` is a JSONB array of
-- exact event types and `group.*` wildcards; matching happens in Go.
-- The failure counters drive the two-condition auto-disable rule.
CREATE TABLE webhook_endpoints (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: whe_
    product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    description TEXT,
    event_types JSONB NOT NULL DEFAULT '[]',
    status VARCHAR(50) NOT NULL,
    disabled_reason TEXT,
    consecutive_failure_count INTEGER NOT NULL DEFAULT 0,
    first_failure_at TIMESTAMPTZ,
    last_failure_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_endpoints_product_id ON webhook_endpoints(product_id);
CREATE INDEX idx_webhook_endpoints_status ON webhook_endpoints(status);

CREATE TRIGGER update_webhook_endpoints_updated_at BEFORE UPDATE ON webhook_endpoints FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 2. Endpoint signing secrets
-- A table rather than a column because rotation needs two live secrets at
-- once: the new ACTIVE one and the previous EXPIRING one until `expires_at`.
-- `encrypted_secret` holds the framework VersionedCipher ciphertext; plaintext
-- leaves Anchor only in the create and rotate responses.
CREATE TABLE webhook_endpoint_secrets (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: whs_
    endpoint_id VARCHAR(255) NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    encrypted_secret TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_endpoint_secrets_endpoint_id ON webhook_endpoint_secrets(endpoint_id);

CREATE TRIGGER update_webhook_endpoint_secrets_updated_at BEFORE UPDATE ON webhook_endpoint_secrets FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 3. Events (the outbox)
-- Written in the same transaction as the business change that produced it,
-- alongside a pgqueue `webhook.fanout` job. `target_endpoint_id` is set only
-- for synthetic events aimed at a single endpoint (ping); it is NULL for
-- ordinary broadcast events.
CREATE TABLE webhook_events (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: evt_
    product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    organization_id VARCHAR(255) REFERENCES organizations(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    api_version VARCHAR(20) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    target_endpoint_id VARCHAR(255) REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_events_product_id ON webhook_events(product_id, created_at DESC);
CREATE INDEX idx_webhook_events_event_type ON webhook_events(event_type);

-- 4. Deliveries
-- One row per (event x endpoint) carrying the exact bytes to send. The body is
-- frozen at fan-out time and never re-marshalled, so a deploy that changes JSON
-- field ordering cannot invalidate an in-flight signature.
--
-- The uniqueness guard that makes fan-out idempotent is a PARTIAL unique index
-- on the original deliveries only (`replay_of_delivery_id IS NULL`). A manual
-- replay deliberately creates a second row for the same (event, endpoint) pair,
-- so a total UNIQUE constraint would forbid the retry feature.
CREATE TABLE webhook_deliveries (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: whd_
    event_id VARCHAR(255) NOT NULL REFERENCES webhook_events(id) ON DELETE CASCADE,
    endpoint_id VARCHAR(255) NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 8,
    target_url TEXT NOT NULL,
    signed_body TEXT NOT NULL,
    last_status_code INTEGER,
    last_error TEXT,
    completed_at TIMESTAMPTZ,
    replay_of_delivery_id VARCHAR(255) REFERENCES webhook_deliveries(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_webhook_deliveries_event_endpoint
    ON webhook_deliveries(event_id, endpoint_id)
    WHERE replay_of_delivery_id IS NULL;

CREATE INDEX idx_webhook_deliveries_endpoint_id ON webhook_deliveries(endpoint_id, created_at DESC);
CREATE INDEX idx_webhook_deliveries_event_id ON webhook_deliveries(event_id);
CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(status);

CREATE TRIGGER update_webhook_deliveries_updated_at BEFORE UPDATE ON webhook_deliveries FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 5. Delivery attempts
-- Append-only attempt log powering the customer-visible delivery view. Rows are
-- never updated, so the table carries no updated_at column or trigger.
CREATE TABLE webhook_delivery_attempts (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: wha_
    delivery_id VARCHAR(255) NOT NULL REFERENCES webhook_deliveries(id) ON DELETE CASCADE,
    attempt_number INTEGER NOT NULL,
    status_code INTEGER,
    error TEXT,
    response_snippet TEXT,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    attempted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_delivery_attempts_delivery_id ON webhook_delivery_attempts(delivery_id, attempt_number);
