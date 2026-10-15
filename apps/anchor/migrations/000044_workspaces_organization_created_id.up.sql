CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_workspaces_organization_created_id
    ON workspaces (organization_id, created_at, id);
