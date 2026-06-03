-- Function to update updated_at column
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
   NEW.updated_at = now();
   RETURN NEW;
END;
$$ language 'plpgsql';

-- Platform Tenant Table
CREATE TABLE platform_tenants (
	id VARCHAR(255) PRIMARY KEY,
	name VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_platform_tenants_status ON platform_tenants(status);

CREATE TRIGGER update_platform_tenants_updated_at BEFORE UPDATE ON platform_tenants FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Platform User Table (Admin users)
CREATE TABLE users (
	id VARCHAR(255) PRIMARY KEY,
    external_id VARCHAR(255) UNIQUE,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(100),
    hashed_password TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status);

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Create the platform_invitations table
CREATE TABLE platform_invitations (
                                      id VARCHAR(255) PRIMARY KEY,
                                      code VARCHAR(255) NOT NULL UNIQUE ,
                                      email VARCHAR(255) NOT NULL,
                                      platform_tenant_id VARCHAR(255) NOT NULL,
                                      created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
                                      updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
                                      FOREIGN KEY (platform_tenant_id) REFERENCES platform_tenants(id) ON DELETE CASCADE
                                 );

-- Add indexes
CREATE INDEX idx_platform_invitations_email ON platform_invitations(email);
CREATE UNIQUE INDEX idx_platform_tenant_email ON platform_invitations(platform_tenant_id, email);

CREATE TRIGGER update_platform_invitations_updated_at BEFORE UPDATE ON platform_invitations FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
-- Product Table
CREATE TABLE products (
	id VARCHAR(255) PRIMARY KEY,
    platform_tenant_id VARCHAR(255) NOT NULL REFERENCES platform_tenants(id) ON DELETE RESTRICT,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (platform_tenant_id, name)
);

CREATE INDEX idx_products_platform_tenant_id ON products(platform_tenant_id);

CREATE TRIGGER update_products_updated_at BEFORE UPDATE ON products FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Product Permission Definitions (Per Product)
-- Using composite primary key (product_id, name) - no separate id field needed
CREATE TABLE product_permissions (
    product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (product_id, name)
);

CREATE INDEX idx_product_permissions_product_id ON product_permissions(product_id);
CREATE TRIGGER update_product_permissions_updated_at BEFORE UPDATE ON product_permissions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Product Role Definitions (Per Product)
CREATE TABLE product_roles (
	id VARCHAR(255) PRIMARY KEY,
    product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (product_id, name)
);

CREATE INDEX idx_product_roles_product_id ON product_roles(product_id);

CREATE TRIGGER update_product_roles_updated_at BEFORE UPDATE ON product_roles FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();


-- Organization Table
CREATE TABLE organizations (
    id VARCHAR(255) PRIMARY KEY,
    product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_organizations_product_id ON organizations(product_id);

CREATE TRIGGER update_organizations_updated_at BEFORE UPDATE ON organizations FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Workspace Table
CREATE TABLE workspaces (
	id VARCHAR(255) PRIMARY KEY,
    organization_id VARCHAR(255) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, name)
);

CREATE INDEX idx_workspaces_organization_id ON workspaces(organization_id);

CREATE TRIGGER update_workspaces_updated_at BEFORE UPDATE ON workspaces FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Product User Table (End-users, directory only)
CREATE TABLE product_users (
	id VARCHAR(255) PRIMARY KEY,
    product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    name VARCHAR(100),
    status VARCHAR(50) NOT NULL, -- Default removed
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (product_id, email)
);

CREATE INDEX idx_product_users_product_id ON product_users(product_id);
CREATE INDEX idx_product_users_email ON product_users(email);
CREATE INDEX idx_product_users_status ON product_users(status);

CREATE TRIGGER update_product_users_updated_at BEFORE UPDATE ON product_users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Organization Membership Table
CREATE TABLE organization_memberships (
	organization_id VARCHAR(255) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    product_user_id VARCHAR(255) NOT NULL REFERENCES product_users(id) ON DELETE CASCADE,
    product_role_id VARCHAR(255) NOT NULL REFERENCES product_roles(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organization_id, product_user_id)
);

CREATE INDEX idx_organization_memberships_product_user_id ON organization_memberships(product_user_id);
CREATE INDEX idx_organization_memberships_product_role_id ON organization_memberships(product_role_id);
CREATE INDEX idx_organization_memberships_org_role ON organization_memberships(organization_id, product_role_id);

CREATE TRIGGER update_organization_memberships_updated_at BEFORE UPDATE ON organization_memberships FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Workspace Membership Table
CREATE TABLE workspace_memberships (
	workspace_id VARCHAR(255) NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    product_user_id VARCHAR(255) NOT NULL REFERENCES product_users(id) ON DELETE CASCADE,
    product_role_id VARCHAR(255) NOT NULL REFERENCES product_roles(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, product_user_id)
);

CREATE INDEX idx_workspace_memberships_product_user_id ON workspace_memberships(product_user_id);
CREATE INDEX idx_workspace_memberships_product_role_id ON workspace_memberships(product_role_id);
CREATE INDEX idx_workspace_memberships_ws_role ON workspace_memberships(workspace_id, product_role_id);

CREATE TRIGGER update_workspace_memberships_updated_at BEFORE UPDATE ON workspace_memberships FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();


-- Create the platform_users table
-- This table unifies productuser information with their platform tenant membership
CREATE TABLE platform_users (
    id VARCHAR(255) PRIMARY KEY,
    external_id VARCHAR(255) UNIQUE,
    email VARCHAR(255) NOT NULL,
    name VARCHAR(100),
    hashed_password TEXT NOT NULL,
    platform_tenant_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Foreign key constraints
    FOREIGN KEY (platform_tenant_id) REFERENCES platform_tenants(id) ON DELETE RESTRICT,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE
);
-- Create indexes for performance
-- Unique constraint on (platform_tenant_id, email) - one productuser per email per tenant
CREATE UNIQUE INDEX idx_platform_users_tenant_email ON platform_users(platform_tenant_id, email);
-- Index on platform_tenant_id for faster tenant-based queries
CREATE INDEX idx_platform_users_tenant_id ON platform_users(platform_tenant_id);
-- Index on email for faster email lookups
CREATE INDEX idx_platform_users_email ON platform_users(email);
-- Index on role for filtering by role
CREATE INDEX idx_platform_users_role ON platform_users(role);

-- Product API Keys Table
CREATE TABLE product_api_keys (
                                  id VARCHAR(255) PRIMARY KEY,
                                  product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
                                  name VARCHAR(100) NOT NULL,
                                  description TEXT,
                                  hashed_value TEXT NOT NULL,
                                  obfuscated_value VARCHAR(100) NOT NULL,
                                  status VARCHAR(50) NOT NULL,
                                  last_used_at TIMESTAMPTZ,
                                  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                  UNIQUE (product_id, name)
);

CREATE INDEX idx_product_api_keys_product_id ON product_api_keys(product_id);
CREATE INDEX idx_product_api_keys_status ON product_api_keys(status);
CREATE INDEX idx_product_api_keys_hashed_value ON product_api_keys(hashed_value);

CREATE TRIGGER update_product_api_keys_updated_at BEFORE UPDATE ON product_api_keys FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE product_api_key_permissions (
                                             api_key_id VARCHAR(255) NOT NULL REFERENCES product_api_keys(id) ON DELETE CASCADE,
                                             product_id VARCHAR(255) NOT NULL,
                                             permission_name VARCHAR(100) NOT NULL,
                                             created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                                             UNIQUE (api_key_id, product_id, permission_name),
                                             FOREIGN KEY (product_id, permission_name) REFERENCES product_permissions(product_id, name) ON DELETE CASCADE,
                                             PRIMARY KEY (product_id, api_key_id, permission_name)
);
CREATE INDEX idx_product_api_key_permissions_api_key_id ON product_api_key_permissions(api_key_id);

-- Create trigger for updated_at column
CREATE TRIGGER update_platform_users_updated_at
    BEFORE UPDATE ON platform_users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

