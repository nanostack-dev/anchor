ALTER TABLE product_event_endpoint_configs
    ADD COLUMN delivery_status TEXT NOT NULL DEFAULT 'never_attempted',
    ADD COLUMN consecutive_failed_calls INTEGER NOT NULL DEFAULT 0;
