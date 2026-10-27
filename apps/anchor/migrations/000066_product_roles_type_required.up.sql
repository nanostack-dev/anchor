-- Every release now writes a role type (ADR-0022). A role the release before
-- types wrote during the 000059 deploy has none, and it is one the Product
-- manages.
SET LOCAL lock_timeout = '5s';

UPDATE product_roles SET type = 'MANAGED' WHERE type IS NULL;
ALTER TABLE product_roles ALTER COLUMN type SET NOT NULL;
