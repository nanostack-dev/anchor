-- A Product finds an invitation by the verified email address of the person
-- who signed in and accepts it by id, so the invitation token is gone. See
-- docs/adr/0019-organization-invitations-live-in-anchor.md.
ALTER TABLE organization_invitations DROP COLUMN token_hash;

CREATE INDEX idx_organization_invitations_product_email ON organization_invitations(product_id, LOWER(email));
