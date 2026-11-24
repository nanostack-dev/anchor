-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS product_user_tombstones;
-- squawk-ignore ban-drop-column
ALTER TABLE product_users DROP COLUMN IF EXISTS last_synced_event_at;
