CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_organization_invitations_product_created_id
    ON organization_invitations (product_id, created_at, id);
