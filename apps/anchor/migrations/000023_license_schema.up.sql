-- =============================================
-- Migration 000023: License Schema
-- =============================================
-- The per-Product declaration of every field a license may carry. This is the
-- first layer of the licensing subsystem; license templates and per-Organization
-- licenses land in later migrations.
--
-- No CHECK constraints and no business triggers. The set of legal field types
-- and the coherence of a rule declaration are service-layer concerns, validated
-- by internal/license/rules before the write. Migration 000014 added CHECK
-- constraints for the email subsystem's status enums; that predates the
-- repository's no-CHECK invariant and is not the pattern to copy.

-- 1. License Schemas
-- One row per Product. The schema is a singleton on the Product rather than a
-- named collection: CONTEXT.md defines a license schema as "the per-Product
-- declaration of every field a license may carry", so a second one for the same
-- Product would have no meaning.
CREATE TABLE license_schemas (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: lsch_
    platform_tenant_id VARCHAR(255) NOT NULL,
    product_id VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (product_id),
    CONSTRAINT fk_license_schemas_product_tenant
        FOREIGN KEY (product_id, platform_tenant_id)
        REFERENCES products(id, platform_tenant_id)
        ON DELETE CASCADE
);

CREATE INDEX idx_license_schemas_product_id ON license_schemas(product_id);
CREATE INDEX idx_license_schemas_platform_tenant_id ON license_schemas(platform_tenant_id);

CREATE TRIGGER update_license_schemas_updated_at BEFORE UPDATE ON license_schemas FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 2. License Schema Fields
-- One row per declared license field. Fields are rows rather than a single
-- JSONB document on the schema because the later usage path resolves a reported
-- key to a field and has to answer "does this key exist, and is it a limit" —
-- an indexed lookup, not a load-and-scan. They are read back ordered by name;
-- the declaration carries no order of its own.
--
-- rules_json holds the structured validation rules, mirroring how
-- email_template_versions.variables_json stores a declared VariableSchema[].
-- Rules are data, never a validator tag string: this contract is public and a
-- tag string would put a Go library's DSL into it.
CREATE TABLE license_schema_fields (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: lfld_
    license_schema_id VARCHAR(255) NOT NULL REFERENCES license_schemas(id) ON DELETE CASCADE,
    name VARCHAR(120) NOT NULL, -- stable identifier used by product code
    -- One of LIMIT / NUMBER / BOOLEAN / ENUM / STRING. Kept out of a CHECK so
    -- adding a type is a service change, not a migration.
    field_type VARCHAR(50) NOT NULL,
    is_required BOOLEAN NOT NULL DEFAULT FALSE,
    description TEXT NOT NULL DEFAULT '',
    rules_json JSONB NOT NULL DEFAULT '{}', -- declared license.FieldRules
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (license_schema_id, name)
);

CREATE INDEX idx_license_schema_fields_schema_id ON license_schema_fields(license_schema_id);

CREATE TRIGGER update_license_schema_fields_updated_at BEFORE UPDATE ON license_schema_fields FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
