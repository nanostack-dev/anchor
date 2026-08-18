-- =============================================
-- Migration 000034: MIGRATED change_type renamed to SET
-- =============================================
-- The migrate route now writes SET rather than MIGRATED, for both an
-- Organization moved onto another template and one granted its first through
-- the same route — see docs/adr/0015-migrate-grants-a-first-license.md.
--
-- change_type carries no CHECK constraint (migration 000032), so nothing else
-- needs a matching change: this is a data-only rewrite of existing rows, kept
-- consistent with what the service now writes going forward.

UPDATE organization_license_changes
SET change_type = 'SET'
WHERE change_type = 'MIGRATED';
