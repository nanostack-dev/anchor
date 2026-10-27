-- Two Organizations may each have a custom role of the same name;
-- idx_product_roles_managed_lower_name now guards the Product's own names.
SET LOCAL lock_timeout = '5s';

ALTER TABLE product_roles DROP CONSTRAINT product_roles_product_id_name_key;
