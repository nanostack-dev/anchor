-- Drop triggers
DROP TRIGGER IF EXISTS update_platform_users_updated_at ON platform_users;
DROP TRIGGER IF EXISTS update_workspace_memberships_updated_at ON workspace_memberships;
DROP TRIGGER IF EXISTS update_organization_memberships_updated_at ON organization_memberships;
DROP TRIGGER IF EXISTS update_product_users_updated_at ON product_users;
DROP TRIGGER IF EXISTS update_workspaces_updated_at ON workspaces;
DROP TRIGGER IF EXISTS update_organizations_updated_at ON organizations;
DROP TRIGGER IF EXISTS update_product_roles_updated_at ON product_roles;
DROP TRIGGER IF EXISTS update_product_permissions_updated_at ON product_permissions;
DROP TRIGGER IF EXISTS update_products_updated_at ON products;
DROP TRIGGER IF EXISTS update_platform_invitations_updated_at ON platform_invitations;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP TRIGGER IF EXISTS update_platform_tenants_updated_at ON platform_tenants;

-- Drop indexes (if not dropped with tables)
DROP INDEX IF EXISTS idx_platform_users_role;
DROP INDEX IF EXISTS idx_platform_users_email;
DROP INDEX IF EXISTS idx_platform_users_tenant_id;
DROP INDEX IF EXISTS idx_platform_users_tenant_email;
DROP INDEX IF EXISTS idx_workspace_memberships_ws_role;
DROP INDEX IF EXISTS idx_workspace_memberships_product_role_id;
DROP INDEX IF EXISTS idx_workspace_memberships_product_user_id;
DROP INDEX IF EXISTS idx_organization_memberships_org_role;
DROP INDEX IF EXISTS idx_organization_memberships_product_role_id;
DROP INDEX IF EXISTS idx_organization_memberships_product_user_id;
DROP INDEX IF EXISTS idx_product_users_status;
DROP INDEX IF EXISTS idx_product_users_email;
DROP INDEX IF EXISTS idx_product_users_product_id;
DROP INDEX IF EXISTS idx_workspaces_organization_id;
DROP INDEX IF EXISTS idx_organizations_product_id;
DROP INDEX IF EXISTS idx_product_role_permissions_product_role_id;
DROP INDEX IF EXISTS idx_product_role_permissions_product_permission_id;
DROP INDEX IF EXISTS idx_product_roles_product_id;
DROP INDEX IF EXISTS idx_product_permissions_product_id;
DROP INDEX IF EXISTS idx_products_platform_tenant_id;
DROP INDEX IF EXISTS idx_platform_tenant_email;
DROP INDEX IF EXISTS idx_platform_invitations_email;
DROP INDEX IF EXISTS idx_users_status;
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_platform_tenants_status;

-- Drop tables in dependency order
DROP TABLE IF EXISTS workspace_memberships CASCADE;
DROP TABLE IF EXISTS organization_memberships CASCADE;
DROP TABLE IF EXISTS product_users CASCADE;
DROP TABLE IF EXISTS workspaces CASCADE;
DROP TABLE IF EXISTS organizations CASCADE;
DROP TABLE IF EXISTS product_role_permissions CASCADE;
DROP TABLE IF EXISTS product_roles CASCADE;
DROP TABLE IF EXISTS product_permissions CASCADE;
DROP TABLE IF EXISTS products CASCADE;
DROP TABLE IF EXISTS platform_users CASCADE;
DROP TABLE IF EXISTS platform_invitations CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS platform_tenants CASCADE;

DROP TRIGGER IF EXISTS update_product_api_keys_updated_at ON product_api_keys;
DROP INDEX IF EXISTS idx_product_api_key_permissions_permission;
DROP INDEX IF EXISTS idx_product_api_key_permissions_api_key_id;
DROP INDEX IF EXISTS idx_product_api_keys_status;
DROP INDEX IF EXISTS idx_product_api_keys_product_id;
DROP TABLE IF EXISTS product_api_key_permissions CASCADE;
DROP TABLE IF EXISTS product_api_keys CASCADE;
-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();
