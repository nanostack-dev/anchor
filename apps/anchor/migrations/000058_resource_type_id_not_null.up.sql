-- Every role and permission has a scope (ADR-0022 G1). The previous release's
-- startup backfill scoped the rows it saw; this fills any row it missed with
-- the Product's organization type, which every Product has since that release.
-- A failure here is forced past by autoFixDirty, so the resource type module
-- refuses to start while either column is still nullable. To recover: run
-- `migrate force 57`, fix the data, then redeploy.
SET LOCAL lock_timeout = '5s';

-- lint: allow-backfill fills only the rows the previous release's backfill missed; a few rows per product
UPDATE product_resource_permissions p SET resource_type_id = t.id
FROM product_resource_types t
WHERE t.product_id = p.product_id AND t.slug = 'organization' AND p.resource_type_id IS NULL;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM product_resource_permissions WHERE resource_type_id IS NULL) THEN
        RAISE EXCEPTION 'product_resource_permissions has rows whose product has no organization type';
    END IF;
END $$;

-- The table is small and the lock times out after 5s; NOT VALID would gain
-- nothing in a file that runs as one transaction.
-- squawk-ignore adding-not-nullable-field
ALTER TABLE product_resource_permissions ALTER COLUMN resource_type_id SET NOT NULL;

-- lint: allow-backfill fills only the rows the previous release's backfill missed; a few rows per product
UPDATE product_roles r SET resource_type_id = t.id
FROM product_resource_types t
WHERE t.product_id = r.product_id AND t.slug = 'organization' AND r.resource_type_id IS NULL;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM product_roles WHERE resource_type_id IS NULL) THEN
        RAISE EXCEPTION 'product_roles has rows whose product has no organization type';
    END IF;
END $$;

-- squawk-ignore adding-not-nullable-field
ALTER TABLE product_roles ALTER COLUMN resource_type_id SET NOT NULL;
