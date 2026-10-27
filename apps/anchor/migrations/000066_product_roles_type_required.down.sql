SET LOCAL lock_timeout = '5s';

ALTER TABLE product_roles ALTER COLUMN type DROP NOT NULL;
