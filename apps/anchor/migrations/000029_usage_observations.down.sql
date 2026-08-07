-- Dropping the hypertable drops its chunks. The timescaledb extension stays:
-- it is an engine capability, not this table's property.
DROP TABLE usage_observations;
