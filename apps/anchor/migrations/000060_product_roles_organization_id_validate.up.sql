SET LOCAL lock_timeout = '5s';

ALTER TABLE product_roles VALIDATE CONSTRAINT product_roles_organization_id_fkey;
