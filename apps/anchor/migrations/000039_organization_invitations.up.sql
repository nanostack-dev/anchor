-- =============================================
-- Migration 000039: Organization Invitations
-- =============================================
-- An offer to become a member of one Organization, addressed to an email
-- address, with one role. See docs/adr/0019-organization-invitations-live-in-anchor.md.
--
-- The status is never stored. A pending invitation has accepted_at NULL and an
-- expires_at in the future; it reads expired once expires_at passes. The rule
-- "at most one pending invitation per email per Organization" therefore cannot
-- be a partial unique index, and the service enforces it inside a transaction
-- that locks the Organization row.
--
-- The token itself is never stored, only its SHA-256 hash.
--
-- No CHECK constraints and no business triggers.

CREATE TABLE organization_invitations (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: oinv_
    product_id VARCHAR(255) NOT NULL,
    organization_id VARCHAR(255) NOT NULL,
    email VARCHAR(320) NOT NULL,
    product_role_id VARCHAR(255) NOT NULL REFERENCES product_roles(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_organization_invitations_organization_product
        FOREIGN KEY (organization_id, product_id)
        REFERENCES organizations(id, product_id)
        ON DELETE CASCADE
);

CREATE INDEX idx_organization_invitations_organization_id ON organization_invitations(organization_id);
CREATE INDEX idx_organization_invitations_organization_email ON organization_invitations(organization_id, LOWER(email));
CREATE INDEX idx_organization_invitations_product_role_id ON organization_invitations(product_role_id);

CREATE TRIGGER update_organization_invitations_updated_at BEFORE UPDATE ON organization_invitations FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
