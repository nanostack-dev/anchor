-- A member may hold no role (ADR-0022): they belong to the Organization and
-- hold no permission. This release reads a NULL role; the next one writes it.
SET LOCAL lock_timeout = '5s';

ALTER TABLE organization_memberships ALTER COLUMN product_role_id DROP NOT NULL;
ALTER TABLE organization_invitations ALTER COLUMN product_role_id DROP NOT NULL;
