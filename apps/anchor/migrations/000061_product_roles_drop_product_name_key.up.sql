-- An Organization's copy shares its template's name; idx_product_roles_template_lower_name
-- now guards template names alone.
ALTER TABLE product_roles DROP CONSTRAINT product_roles_product_id_name_key;
