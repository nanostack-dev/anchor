SET LOCAL lock_timeout = '5s';

-- One OpenFGA authorization model per (product, release model version) so
-- rolling deploys across modelVersion bumps stop rewriting each other's model
-- pointer every 30 s, and rollbacks still find their version's model (ADR-0025).
-- authz_stores.model_id and model_hash stay populated as a mirror for rollbacks
-- to pre-#401 releases until a follow-up migration drops them.
CREATE TABLE authz_models (
    product_id    VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    model_version VARCHAR(64)  NOT NULL,
    model_hash    VARCHAR(64)  NOT NULL,
    model_id      VARCHAR(64)  NOT NULL,
    written_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (product_id, model_version)
);
