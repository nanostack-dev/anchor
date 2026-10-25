-- The product-wide name guard comes back only while no Organization holds a
-- copy of a role template, since a copy shares its template's name. Before
-- rolling back past this migration, move memberships back to the templates and
-- delete the copies (product_roles rows with an organization_id).
ALTER TABLE product_roles ADD CONSTRAINT product_roles_product_id_name_key UNIQUE (product_id, name);
