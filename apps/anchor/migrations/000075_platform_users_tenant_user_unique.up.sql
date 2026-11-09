CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_platform_users_tenant_user
    ON platform_users (platform_tenant_id, user_id);
