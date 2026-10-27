ALTER TABLE product_resource_permissions ALTER COLUMN resource_type_id DROP NOT NULL;
ALTER TABLE product_roles ALTER COLUMN resource_type_id DROP NOT NULL;
