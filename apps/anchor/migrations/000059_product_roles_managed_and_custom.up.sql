-- A Product role is MANAGED (no organization_id: the Product manages it and
-- every Organization of the Product references it by the same ID) or CUSTOM
-- (an Organization's own role). The code owns the values of type and keeps it
-- in step with organization_id. The previous release keeps writing roles
-- without a type during the deploy: the code reads a NULL type as MANAGED,
-- and the startup backfill fills it.
SET LOCAL lock_timeout = '5s';

ALTER TABLE product_roles ADD COLUMN organization_id VARCHAR(255);
ALTER TABLE product_roles ADD COLUMN type VARCHAR(10);

ALTER TABLE product_roles
    ADD CONSTRAINT product_roles_organization_id_fkey
        FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE NOT VALID;

-- lint: allow-backfill every existing role is one the Product manages; a few rows per product
UPDATE product_roles SET type = 'MANAGED' WHERE type IS NULL;
