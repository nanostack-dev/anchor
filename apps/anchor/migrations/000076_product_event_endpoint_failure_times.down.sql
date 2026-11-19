ALTER TABLE product_event_endpoint_configs
    DROP COLUMN IF EXISTS last_failed_at,
    DROP COLUMN IF EXISTS failing_since;
