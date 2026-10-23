-- OpenFGA keeps its own tables in this schema; Anchor runs OpenFGA's migrations
-- into it at startup. Keeping them out of public keeps jet generation and the
-- Anchor schema unchanged.
CREATE SCHEMA IF NOT EXISTS openfga;

-- One OpenFGA store per product. The store ID never changes once created.
CREATE TABLE authz_stores (
    product_id VARCHAR(255) PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    store_id VARCHAR(64) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- A row says the relationship store may lag Postgres for one organization, or
-- for the whole product when organization_id is ''. Writes bump revision in the
-- same transaction as the change; a sync deletes the row only if revision is
-- still the one it read, so a write that lands during a sync is never lost.
CREATE TABLE authz_sync_marks (
    product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    organization_id VARCHAR(255) NOT NULL DEFAULT '',
    revision BIGINT NOT NULL DEFAULT 1,
    marked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (product_id, organization_id)
);
