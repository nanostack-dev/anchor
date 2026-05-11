-- Add product resource permissions table
CREATE TABLE product_resource_permissions (
    product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    scope_modifier VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    primary key (product_id, name)
);
CREATE INDEX idx_product_resource_permissions_product_id ON product_resource_permissions(product_id);

CREATE TRIGGER update_product_resource_permissions_updated_at
    BEFORE UPDATE ON product_resource_permissions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE product_role_resource_permissions (
    id VARCHAR(255) PRIMARY KEY,
    product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    product_role_id VARCHAR(255) NOT NULL REFERENCES product_roles(id) ON DELETE CASCADE,
    permission_name VARCHAR(200) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(product_id, product_role_id, permission_name),
    FOREIGN KEY (product_id, permission_name) REFERENCES product_resource_permissions(product_id, name)
);

CREATE INDEX idx_role_resource_permissions_role ON product_role_resource_permissions(product_role_id);
CREATE INDEX idx_role_resource_permissions_product ON product_role_resource_permissions(product_id);
CREATE INDEX idx_role_resource_permissions_name ON product_role_resource_permissions(permission_name);
