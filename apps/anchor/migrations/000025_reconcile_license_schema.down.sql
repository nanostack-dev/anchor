-- Best-effort reverse of the forward reconcile. The dropped `license_signing_keys`
-- table is intentionally NOT recreated: signed license tokens were removed from
-- the product, so there is no schema to restore it to. Only the column rename is
-- reversed, and only when the forward direction actually applied it.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'licenses' AND column_name = 'refresh_interval_seconds'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'licenses' AND column_name = 'token_ttl_seconds'
    ) THEN
        ALTER TABLE licenses RENAME COLUMN refresh_interval_seconds TO token_ttl_seconds;
    END IF;
END $$;
