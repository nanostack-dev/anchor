-- =============================================
-- Migration 000035: Which license fields an Organization adjusted
-- =============================================
-- A license now follows its template: a template value update is propagated
-- onto every license instantiated from it, except on the fields the
-- Organization adjusted for itself. See
-- docs/adr/0017-license-follows-its-template.md.
--
-- The adjusted fields are recorded on the license row rather than replayed
-- from the history at propagation time. A JSON array of license field names,
-- like values_json a single document read and written whole.
--
-- No CHECK constraints and no business triggers. Which writes add to, keep,
-- or clear this set is a service-layer concern.

ALTER TABLE organization_licenses
    ADD COLUMN adjusted_fields JSONB NOT NULL DEFAULT '[]';
