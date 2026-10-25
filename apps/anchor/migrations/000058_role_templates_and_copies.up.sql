-- Roles become Organization data (ADR-0022 D8): a product role with no
-- organization_id is a role template; a row with one is that Organization's
-- role, a copy of a template (template_id set) or a custom role. Templates and
-- Organization roles share the table so that memberships, invitations and
-- workspace memberships keep their product_role_id foreign keys across a
-- rolling deploy. Keys, copies and the move of memberships onto the copies are
-- made at startup (internal/organizationrole), because copy IDs are KSUIDs.
ALTER TABLE product_roles ADD COLUMN organization_id VARCHAR(255);
ALTER TABLE product_roles ADD COLUMN template_id VARCHAR(255);
ALTER TABLE product_roles ADD COLUMN key VARCHAR(100);
ALTER TABLE product_roles ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT FALSE;
-- The ID callers see: a copy keeps its template's ID inside its Organization.
ALTER TABLE product_roles ADD COLUMN public_id VARCHAR(255)
    GENERATED ALWAYS AS (COALESCE(template_id, id)) VIRTUAL;

ALTER TABLE product_roles
    ADD CONSTRAINT product_roles_organization_id_fkey
        FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE NOT VALID;
ALTER TABLE product_roles
    ADD CONSTRAINT product_roles_template_id_fkey
        FOREIGN KEY (template_id) REFERENCES product_roles(id) ON DELETE SET NULL NOT VALID;
