-- =============================================
-- Migration 000031: Usage shape is declared per license field
-- =============================================
-- Whether a limit's usage is a gauge or a windowed counter is now pinned on
-- the field, not left to whatever shape the caller happened to report last.
-- See docs/adr/0013-usage-shape-is-declared-not-inferred.md.
--
-- Nullable, no CHECK constraint: "required for a limit, forbidden otherwise"
-- is a service-layer rule, per this repository's invariant against business
-- logic in SQL. GAUGE / WINDOWED_COUNTER is kept out of a CHECK for the same
-- reason field_type is — adding a shape is a service change, not a migration.

ALTER TABLE license_schema_fields ADD COLUMN usage_shape VARCHAR(50);
