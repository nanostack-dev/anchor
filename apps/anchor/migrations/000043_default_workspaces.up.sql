ALTER TABLE workspaces ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT false;

CREATE UNIQUE INDEX workspaces_one_default_per_organization
    ON workspaces (organization_id) WHERE is_default;
