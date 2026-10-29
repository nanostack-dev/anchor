-- Role assignments on registered resources (ADR-0022 D3, D4): a member holds
-- an Organization's role on one resource. The key holds the role, so the
-- list-shaped contract can later let a member hold several roles on one
-- resource; while a Product allows one role per resource, the service keeps
-- one row per member.
CREATE TABLE resource_role_assignments (
    resource_id VARCHAR(255) NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    product_user_id VARCHAR(255) NOT NULL,
    organization_id VARCHAR(255) NOT NULL,
    -- A role the Product manages, or a custom role of the Organization.
    -- Cascades like organization_memberships.product_role_id; a delete is
    -- refused while assignments hold the role (internal/service).
    role_id VARCHAR(255) NOT NULL REFERENCES product_roles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (resource_id, product_user_id, role_id),
    -- Only a member of the Organization holds an assignment, and removing the
    -- member removes them. The member removal emits their events first
    -- (internal/roleassignment).
    CONSTRAINT resource_role_assignments_membership_fkey
        FOREIGN KEY (organization_id, product_user_id)
        REFERENCES organization_memberships (organization_id, product_user_id) ON DELETE CASCADE
);

CREATE INDEX idx_resource_role_assignments_resource_created_at
    ON resource_role_assignments (resource_id, created_at, product_user_id);
CREATE INDEX idx_resource_role_assignments_role_id ON resource_role_assignments (role_id);
CREATE INDEX idx_resource_role_assignments_organization_user
    ON resource_role_assignments (organization_id, product_user_id);

CREATE TRIGGER update_resource_role_assignments_updated_at
    BEFORE UPDATE ON resource_role_assignments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
