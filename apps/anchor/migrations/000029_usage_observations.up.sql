-- =============================================
-- Migration 000029: Usage Observations
-- =============================================
-- What an Organization has actually consumed. Rows are immutable: no
-- updated_at, no update trigger, because a correction is a new observation.

CREATE TABLE usage_observations (
    id VARCHAR(255) NOT NULL, -- KSUID prefix: uobs_
    platform_tenant_id VARCHAR(255) NOT NULL,
    product_id VARCHAR(255) NOT NULL,
    organization_id VARCHAR(255) NOT NULL,
    -- Not a foreign key to license_schema_fields: dropping a field from a
    -- schema must not delete the record of what was used while it existed.
    key VARCHAR(255) NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    -- Prefixed because FROM and TO are reserved words. The API calls them
    -- from and to. Both set is a counter over [from, to), both null is a gauge.
    window_from TIMESTAMPTZ,
    window_to TIMESTAMPTZ,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- observed_at is in the key because every unique index on a hypertable
    -- must carry the partition column.
    PRIMARY KEY (id, observed_at),
    CONSTRAINT fk_usage_observations_organization_product
        FOREIGN KEY (organization_id, product_id)
        REFERENCES organizations(id, product_id)
        ON DELETE CASCADE,
    CONSTRAINT fk_usage_observations_product_tenant
        FOREIGN KEY (product_id, platform_tenant_id)
        REFERENCES products(id, platform_tenant_id)
        ON DELETE CASCADE
);

-- A hypertable, so aggregation, retention and compression are policies rather
-- than a hand-rolled thinning pass. See
-- docs/adr/0005-timescaledb-for-usage-history.md.
CREATE EXTENSION IF NOT EXISTS timescaledb;
SELECT create_hypertable('usage_observations', by_range('observed_at'), if_not_exists => TRUE);

-- One Organization's series for one license field, newest first.
-- create_hypertable already indexes observed_at on its own.
CREATE INDEX idx_usage_observations_organization_key
    ON usage_observations(organization_id, key, observed_at DESC);
