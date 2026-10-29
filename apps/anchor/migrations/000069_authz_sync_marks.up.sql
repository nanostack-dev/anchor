-- The sync between Anchor's tables and each Product's OpenFGA store (ADR-0022
-- E5). A mark says the store may lag the tables for one key of one
-- projection, every key of a projection when scope_key is '*', or the whole
-- Product when projection is ''. A write leaves it in its own transaction, so
-- no committed change goes unsynced; a sync deletes it only if revision is
-- still the one it read, so a write landing during a sync is never lost.
-- Revisions come from one sequence, so a mark deleted and made again never
-- reuses the revision a sync read before.
SET LOCAL lock_timeout = '5s';

CREATE SEQUENCE authz_sync_mark_revisions;

CREATE TABLE authz_sync_marks (
    product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    projection VARCHAR(64) NOT NULL,
    scope_key VARCHAR(255) NOT NULL,
    revision BIGINT NOT NULL DEFAULT nextval('authz_sync_mark_revisions'),
    marked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (product_id, projection, scope_key)
);

CREATE INDEX idx_authz_sync_marks_marked_at ON authz_sync_marks (marked_at);

-- The model a store's tuples follow, and the hash of the catalog it was
-- compiled from; reconciled_at is when the store was last rebuilt in full.
ALTER TABLE authz_stores ADD COLUMN model_id VARCHAR(64);
ALTER TABLE authz_stores ADD COLUMN model_hash VARCHAR(64);
ALTER TABLE authz_stores ADD COLUMN reconciled_at TIMESTAMPTZ;
