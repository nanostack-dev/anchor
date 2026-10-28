ALTER TABLE authz_stores DROP COLUMN reconciled_at;
ALTER TABLE authz_stores DROP COLUMN model_hash;
ALTER TABLE authz_stores DROP COLUMN model_id;
DROP TABLE IF EXISTS authz_sync_marks;
DROP SEQUENCE IF EXISTS authz_sync_mark_revisions;
