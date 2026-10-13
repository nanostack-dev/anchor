DROP INDEX workspaces_one_default_per_organization;
ALTER TABLE workspaces DROP COLUMN is_default;
