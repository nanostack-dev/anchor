-- =============================================
-- Migration 000023: License & Entitlement System
-- =============================================
-- Plans and per-organization licenses.
-- Business rules (status enums, entitlement shape, refresh-interval ranges)
-- live in the service layer; the DB keeps PK/FK/UNIQUE/NOT NULL/defaults and
-- the shared updated_at trigger only.

-- 1. Plans
-- Per-product plan definitions. `key` is the stable identifier (future Stripe
-- lookup_key); `entitlements` is a JSONB map key -> {type, value}.
CREATE TABLE plans (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: plan_
    product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    key VARCHAR(100) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    entitlements JSONB NOT NULL DEFAULT '{}',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (product_id, key)
);

CREATE INDEX idx_plans_product_id ON plans(product_id);

CREATE TRIGGER update_plans_updated_at BEFORE UPDATE ON plans FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 2. Licenses
-- One license per organization. Status values ('ACTIVE', 'SUSPENDED',
-- 'REVOKED') are validated in the service layer. `entitlement_overrides`
-- shadows plan entitlements per organization (override wins).
-- `refresh_interval_seconds` is the cadence hint returned to consumers as
-- `refresh_after`; it does not expire anything on its own.
CREATE TABLE licenses (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: lic_
    product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    organization_id VARCHAR(255) NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    plan_id VARCHAR(255) NOT NULL REFERENCES plans(id) ON DELETE RESTRICT,
    status VARCHAR(50) NOT NULL,
    expires_at TIMESTAMPTZ,
    grace_until TIMESTAMPTZ,
    entitlement_overrides JSONB NOT NULL DEFAULT '{}',
    refresh_interval_seconds INTEGER NOT NULL DEFAULT 86400,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_licenses_product_id ON licenses(product_id);
CREATE INDEX idx_licenses_plan_id ON licenses(plan_id);

CREATE TRIGGER update_licenses_updated_at BEFORE UPDATE ON licenses FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
