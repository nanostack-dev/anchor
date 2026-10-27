-- The product-wide name guard comes back only while no two roles of a Product
-- share a name: delete the custom roles (product_roles rows with an
-- organization_id) before rolling back past this migration.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_product_roles_product_lower_name_unique
    ON product_roles (product_id, lower(name));
