-- =============================================
-- Migration 000029: Usage Observations
-- =============================================
-- What an Organization has actually consumed, against the limits its license
-- grants. A consumer reports an absolute snapshot and Anchor appends it here.
-- Anchor never sums, so a retried report is harmless and a report that never
-- arrived self-corrects on the next one. See
-- docs/adr/0003-usage-reported-as-snapshots.md.
--
-- Rows are immutable. There is no updated_at column and no update trigger: a
-- correction is a new observation, never an overwrite. That is what makes the
-- series a history rather than a current value with a timestamp on it.
--
-- No CHECK constraints and no business triggers. Whether a reported key names a
-- declared limit is a service-layer concern.

CREATE TABLE usage_observations (
    id VARCHAR(255) NOT NULL, -- KSUID prefix: uobs_
    platform_tenant_id VARCHAR(255) NOT NULL,
    product_id VARCHAR(255) NOT NULL,
    organization_id VARCHAR(255) NOT NULL,
    -- The license field this value was reported against. Not a foreign key to
    -- license_schema_fields: a schema edit that drops a field must not delete
    -- the record of what was used while the field existed.
    key VARCHAR(255) NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    -- Both set, or both null. Null is a gauge — "37 flows exist right now".
    -- Set is a windowed counter over the half-open period [start, end). Two
    -- timestamps rather than a formatted period, because real billing periods
    -- follow the subscription anniversary rather than the calendar.
    window_start TIMESTAMPTZ,
    window_end TIMESTAMPTZ,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- observed_at is in the key because it is the partition column, and every
    -- unique index on a hypertable must carry it.
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

-- A hypertable, so the continuous aggregates, retention and compression a usage
-- history needs are policies rather than a hand-rolled thinning pass. Those
-- land with the series read; this migration establishes the storage only. See
-- docs/adr/0005-timescaledb-for-usage-history.md.
--
-- Anchor runs TimescaleDB everywhere — compose, tests and production all pin
-- the same image — so this is a hard dependency rather than an optional
-- enhancement.
CREATE EXTENSION IF NOT EXISTS timescaledb;
SELECT create_hypertable('usage_observations', by_range('observed_at'), if_not_exists => TRUE);

-- The only read shape there is: one Organization's series for one license
-- field, newest first. create_hypertable already indexes observed_at on its
-- own, which serves chunk pruning.
--
-- There is no index on platform_tenant_id alone. Every statement reaches this
-- table through an Organization, so a tenant-wide scan has no caller.
CREATE INDEX idx_usage_observations_organization_key
    ON usage_observations(organization_id, key, observed_at DESC);
