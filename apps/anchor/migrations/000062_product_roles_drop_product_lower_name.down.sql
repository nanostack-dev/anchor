-- The product-wide name guard comes back only while no Organization holds a
-- copy of a role template, since a copy shares its template's name. Before
-- rolling back past this migration, move memberships back to the templates and
-- delete the copies (product_roles rows with an organization_id).
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_product_roles_product_lower_name_unique
    ON product_roles (product_id, lower(name));
