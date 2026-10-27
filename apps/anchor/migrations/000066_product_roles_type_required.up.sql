-- Every release now writes a role type (ADR-0022). A role the release before
-- types wrote during the 000059 deploy has none, and it is one the Product
-- manages. A failure here is forced past by autoFixDirty, so the resource type
-- module refuses to start while the column is still nullable. To recover: run
-- `migrate force 65`, then redeploy.
SET LOCAL lock_timeout = '5s';

-- lint: allow-backfill fills only the roles the previous release wrote during the 000059 deploy; a few rows per product
UPDATE product_roles SET type = 'MANAGED' WHERE type IS NULL;

-- The table is small and the lock times out after 5s; NOT VALID would gain
-- nothing in a file that runs as one transaction.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE product_roles ALTER COLUMN type SET NOT NULL;
