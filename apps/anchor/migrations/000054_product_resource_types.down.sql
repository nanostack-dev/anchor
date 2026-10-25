ALTER TABLE product_roles DROP COLUMN resource_type_id;
ALTER TABLE product_resource_permissions DROP COLUMN resource_type_id;
DROP TABLE product_resource_type_parents;
DROP TABLE product_resource_types;
