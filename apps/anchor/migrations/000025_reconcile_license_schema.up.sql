-- Reconcile the license schema for any database that applied an INTERMEDIATE
-- form of migration 000023.
--
-- 000023 was edited in place after it had already been applied to a persistent
-- (preview) database: the `licenses.token_ttl_seconds` column was renamed to
-- `refresh_interval_seconds` and the `license_signing_keys` table was removed.
-- golang-migrate tracks migrations by version number, so an edited 000023 is
-- never re-run — the stranded database keeps the old column and table while the
-- binary's generated models expect the new shape, and every `licenses` query
-- fails with `column licenses.refresh_interval_seconds does not exist`.
--
-- This migration rolls the schema forward to the intended 000023 shape. It is a
-- no-op on a database that applied the final 000023 (fresh installs, CI, main),
-- because the column is already renamed and the table already absent.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'licenses' AND column_name = 'token_ttl_seconds'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'licenses' AND column_name = 'refresh_interval_seconds'
    ) THEN
        ALTER TABLE licenses RENAME COLUMN token_ttl_seconds TO refresh_interval_seconds;
    END IF;
END $$;

DROP TABLE IF EXISTS license_signing_keys;
