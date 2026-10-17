CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_organization_invitations_organization_created_id
    ON organization_invitations (organization_id, created_at, id);
