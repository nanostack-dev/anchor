-- Drop triggers
DROP TRIGGER IF EXISTS update_integration_events_updated_at ON integration_events;
DROP TRIGGER IF EXISTS update_integration_instances_updated_at ON integration_instances;

-- Drop tables in dependency order (children first)
DROP TABLE IF EXISTS integration_audit_logs CASCADE;
DROP TABLE IF EXISTS integration_events CASCADE;
DROP TABLE IF EXISTS integration_instances CASCADE;

-- Remove composite unique constraint added for tenant-consistent FKs
ALTER TABLE products DROP CONSTRAINT IF EXISTS uq_products_id_tenant;

-- Remove external identity column from product_users
DROP INDEX IF EXISTS idx_product_users_external_lookup;
ALTER TABLE product_users
    DROP COLUMN IF EXISTS external_id;
