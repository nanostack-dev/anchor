CREATE TABLE workspaces (
    id VARCHAR(255) PRIMARY KEY,
    organization_id VARCHAR(255) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, name)
);
CREATE INDEX idx_workspaces_organization_id ON workspaces (organization_id);
CREATE INDEX idx_workspaces_organization_created_id ON workspaces (organization_id, created_at, id);
CREATE TRIGGER update_workspaces_updated_at BEFORE UPDATE ON workspaces
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE workspace_memberships (
    workspace_id VARCHAR(255) NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_user_id VARCHAR(255) NOT NULL REFERENCES product_users(id) ON DELETE CASCADE,
    product_role_id VARCHAR(255) NOT NULL REFERENCES product_roles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, product_user_id)
);
CREATE INDEX idx_workspace_memberships_product_user_id ON workspace_memberships (product_user_id);
CREATE INDEX idx_workspace_memberships_product_role_id ON workspace_memberships (product_role_id);
CREATE INDEX idx_workspace_memberships_ws_role ON workspace_memberships (workspace_id, product_role_id);
CREATE TRIGGER update_workspace_memberships_updated_at BEFORE UPDATE ON workspace_memberships
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE workspace_resources (
    resource_id VARCHAR(255) PRIMARY KEY REFERENCES resources(id) ON DELETE CASCADE
);
