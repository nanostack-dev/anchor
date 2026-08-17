-- =============================================
-- Migration 000033: Which template a migrated Organization came from
-- =============================================
-- A MIGRATED entry names the template the Organization moved to in
-- template_id. The one it moved from has no column, and walking the history
-- back to find it is not what an audit entry is for.
--
-- Nullable and unconstrained: every entry written before this column existed
-- has no answer, and only a MIGRATED entry ever has one. Which change types
-- fill which columns stays a service-layer concern, as migration 000032 says.

ALTER TABLE organization_license_changes
    ADD COLUMN previous_template_id VARCHAR(255);
