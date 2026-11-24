SET LOCAL lock_timeout = '5s';

-- Clerk updated_at (ms) of the last Clerk event applied; NULL = never synced by
-- Clerk (API-created, invited, or not yet reconciled since this migration).
ALTER TABLE product_users ADD COLUMN last_synced_event_at TIMESTAMPTZ;

-- Records a Clerk deletion so an out-of-order user.created / user.updated
-- webhook or stale reconcile cannot resurrect a deleted user (ADR-0026).
-- Clerk user IDs are never reused.
CREATE TABLE product_user_tombstones (
    product_id  VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    external_id VARCHAR(255) NOT NULL,
    deleted_at  TIMESTAMPTZ  NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (product_id, external_id)
);
