-- =============================================
-- Migration 000030: License field expected reporting interval
-- =============================================
-- Lets a schema declare how often a limit field's usage is expected to
-- arrive. The license read uses it to derive the `stale` status: nothing in
-- Anchor pulls a report or checks a schedule, so declaring the expectation is
-- the only way "stale" becomes reachable for a limit. See
-- docs/adr/0012-license-status-derived-on-read.md.
--
-- Nullable and unconstrained by a CHECK: whether it applies (limit fields
-- only) and whether it is positive are service-layer concerns, validated by
-- internal/license/service before the write, matching every other rule on
-- this table.

ALTER TABLE license_schema_fields
    ADD COLUMN expected_reporting_interval_seconds INTEGER;
