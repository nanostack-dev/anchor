-- =============================================
-- Migration 000030: Usage observation aggregate cascade
-- =============================================
-- Granularity for the usage series read, not the raw report path. Three
-- continuous aggregates, each built on the one before it: minute from the raw
-- hypertable, hour from minute, day from hour. Mirrors echopoint's
-- request_metrics cascade. See docs/adr/0005-timescaledb-for-usage-history.md.
--
-- Every level uses last(value, <finer bucket>) per bucket, never sum or avg.
-- Values are absolute snapshots (docs/adr/0003-usage-reported-as-snapshots.md),
-- so the last observation in a bucket already *is* the bucket's value, and that
-- is what makes dropping the raw chunk underneath it safe once the aggregate
-- has captured it.
--
-- This migration is the repository's one sanctioned exception to "no business
-- logic in SQL": time-series aggregation. Interpreting a series against a
-- limit is still service-layer work — see the licensing service package.
--
-- window_from / window_to ride along through last() too, so a windowed
-- counter's bucket still reports the window it was observed over. A gauge
-- carries both as null throughout, unchanged by the aggregation.
--
-- materialized_only = true on all three levels, not just the two with a
-- dependent aggregate on top. TimescaleDB requires it on a continuous
-- aggregate that is itself the source of another one (minute, hour), and the
-- same setting on the top-level day aggregate keeps every level's freshness
-- governed by the same explicit refresh policy rather than mixing in
-- real-time aggregation over an underlying view that is itself
-- materialized-only. Freshness is bounded by the refresh policies below, not
-- by query-time computation.

-- ---------------------------------------------------------------------------
-- Minute: built directly from the raw hypertable.
-- ---------------------------------------------------------------------------
CREATE MATERIALIZED VIEW usage_observations_minute
WITH (timescaledb.continuous, timescaledb.materialized_only = true) AS
SELECT
    platform_tenant_id,
    product_id,
    organization_id,
    key,
    time_bucket('1 minute', observed_at) AS bucket,
    last(value, observed_at) AS value,
    last(window_from, observed_at) AS window_from,
    last(window_to, observed_at) AS window_to
FROM usage_observations
GROUP BY platform_tenant_id, product_id, organization_id, key, time_bucket('1 minute', observed_at)
WITH NO DATA;

CREATE INDEX idx_usage_observations_minute_org_key
    ON usage_observations_minute(organization_id, key, bucket DESC);

-- Refreshes the trailing hour every minute, leaving a one-minute buffer so a
-- bucket is not refreshed while it may still receive a write. observed_at is
-- stamped by Anchor at write time (never caller-supplied), so there is no
-- late-arriving data to account for beyond that buffer.
SELECT add_continuous_aggregate_policy('usage_observations_minute',
    start_offset => INTERVAL '1 hour',
    end_offset => INTERVAL '1 minute',
    schedule_interval => INTERVAL '1 minute');

-- Fine granularity is a short-lived view: a month of minute buckets is already
-- enough resolution history for most limits, and the hour/day levels below
-- keep the record beyond it.
SELECT add_retention_policy('usage_observations_minute', drop_after => INTERVAL '30 days');

-- ---------------------------------------------------------------------------
-- Hour: built from minute, not from raw. A cascading aggregate reads less
-- data per refresh than re-bucketing the hypertable at every level would.
-- ---------------------------------------------------------------------------
CREATE MATERIALIZED VIEW usage_observations_hour
WITH (timescaledb.continuous, timescaledb.materialized_only = true) AS
SELECT
    platform_tenant_id,
    product_id,
    organization_id,
    key,
    time_bucket('1 hour', bucket) AS bucket,
    last(value, bucket) AS value,
    last(window_from, bucket) AS window_from,
    last(window_to, bucket) AS window_to
FROM usage_observations_minute
GROUP BY platform_tenant_id, product_id, organization_id, key, time_bucket('1 hour', bucket)
WITH NO DATA;

CREATE INDEX idx_usage_observations_hour_org_key
    ON usage_observations_hour(organization_id, key, bucket DESC);

SELECT add_continuous_aggregate_policy('usage_observations_hour',
    start_offset => INTERVAL '1 day',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '30 minutes');

-- A little over a year, so a year-over-year comparison at hourly resolution
-- still resolves right up to the boundary.
SELECT add_retention_policy('usage_observations_hour', drop_after => INTERVAL '400 days');

-- ---------------------------------------------------------------------------
-- Day: built from hour. The coarse, long-lived tier — no retention policy, so
-- a day bucket outlives the minute and hour data it was rolled up from.
-- ---------------------------------------------------------------------------
CREATE MATERIALIZED VIEW usage_observations_day
WITH (timescaledb.continuous, timescaledb.materialized_only = true) AS
SELECT
    platform_tenant_id,
    product_id,
    organization_id,
    key,
    time_bucket('1 day', bucket) AS bucket,
    last(value, bucket) AS value,
    last(window_from, bucket) AS window_from,
    last(window_to, bucket) AS window_to
FROM usage_observations_hour
GROUP BY platform_tenant_id, product_id, organization_id, key, time_bucket('1 day', bucket)
WITH NO DATA;

CREATE INDEX idx_usage_observations_day_org_key
    ON usage_observations_day(organization_id, key, bucket DESC);

SELECT add_continuous_aggregate_policy('usage_observations_day',
    start_offset => INTERVAL '3 days',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '6 hours');

-- ---------------------------------------------------------------------------
-- Raw hypertable: compress once the minute aggregate has had time to capture
-- a chunk, then drop it once the whole cascade above has. Compression and
-- retention are storage-engine policies, not business rules — the exception
-- ADR-0005 grants is for exactly this.
-- ---------------------------------------------------------------------------
ALTER TABLE usage_observations SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'platform_tenant_id, product_id, organization_id, key'
);
SELECT add_compression_policy('usage_observations', compress_after => INTERVAL '3 days');

-- Raw detail is kept only long enough to be useful for debugging a recent
-- report, not as history — the aggregates above are what keeps that. Eight
-- days is well past both the minute aggregate's one-hour refresh buffer and
-- the three-day compression policy, so a raw chunk is never dropped before
-- the aggregate above it has captured every row it held.
SELECT add_retention_policy('usage_observations', drop_after => INTERVAL '8 days');
