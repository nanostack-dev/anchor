SET LOCAL lock_timeout = '5s';

ALTER TABLE product_roles DROP COLUMN type;
ALTER TABLE product_roles DROP COLUMN organization_id;
