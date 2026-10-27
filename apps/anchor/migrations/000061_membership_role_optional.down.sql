-- Succeeds only while no membership or invitation holds a NULL role: give
-- them a role, or delete them, before rolling back past this migration.
SET LOCAL lock_timeout = '5s';

ALTER TABLE organization_invitations ALTER COLUMN product_role_id SET NOT NULL;
ALTER TABLE organization_memberships ALTER COLUMN product_role_id SET NOT NULL;
