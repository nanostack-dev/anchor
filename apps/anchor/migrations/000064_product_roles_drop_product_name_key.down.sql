-- The product-wide name guard comes back only while no two roles of a Product
-- share a name: delete the custom roles (product_roles rows with an
-- organization_id) before rolling back past this migration.
SET LOCAL lock_timeout = '5s';

ALTER TABLE product_roles ADD CONSTRAINT product_roles_product_id_name_key UNIQUE (product_id, name);
