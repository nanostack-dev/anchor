-- OpenFGA's relationship store (ADR-0021, ADR-0022 E3 and E5). OpenFGA keeps
-- its own tables in the openfga schema; Anchor runs OpenFGA's migrations into
-- it at startup. Keeping them out of public leaves jet generation and the
-- Anchor schema unchanged.
CREATE SCHEMA IF NOT EXISTS openfga;

-- One OpenFGA store per Product. A store's ID never changes once recorded.
CREATE TABLE authz_stores (
    product_id VARCHAR(255) PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    store_id VARCHAR(64) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
