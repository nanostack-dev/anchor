CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_organization_memberships_organization_created_user
    ON organization_memberships (organization_id, created_at, product_user_id);
