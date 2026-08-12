-- Policies are dropped explicitly rather than relying on cascade: retention
-- and compression policies on the raw hypertable are not owned by the
-- continuous aggregates and would otherwise survive this migration's reversal.
SELECT remove_retention_policy('usage_observations', if_exists => TRUE);
SELECT remove_compression_policy('usage_observations', if_exists => TRUE);
ALTER TABLE usage_observations SET (timescaledb.compress = FALSE);

-- Dropped coarsest-first: day depends on hour, hour depends on minute.
-- Dropping a continuous aggregate removes its own refresh and retention
-- policies along with it.
DROP MATERIALIZED VIEW IF EXISTS usage_observations_day;
DROP MATERIALIZED VIEW IF EXISTS usage_observations_hour;
DROP MATERIALIZED VIEW IF EXISTS usage_observations_minute;
