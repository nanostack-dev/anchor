CREATE UNIQUE INDEX idx_products_tenant_lower_name_unique
    ON products (platform_tenant_id, lower(name));

CREATE UNIQUE INDEX idx_product_permissions_product_lower_name_unique
    ON product_permissions (product_id, lower(name));

CREATE UNIQUE INDEX idx_product_roles_product_lower_name_unique
    ON product_roles (product_id, lower(name));

CREATE UNIQUE INDEX idx_product_resource_permissions_product_lower_name_unique
    ON product_resource_permissions (product_id, lower(name));
