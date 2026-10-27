SET LOCAL lock_timeout = '5s';

ALTER TABLE product_resource_types DROP COLUMN allows_custom_roles;
