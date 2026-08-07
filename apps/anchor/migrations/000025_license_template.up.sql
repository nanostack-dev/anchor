-- =============================================
-- Migration 000025: License Template
-- =============================================
-- A named set of values satisfying a Product's license schema. "Free" and "Pro"
-- become reusable objects rather than values retyped per customer.
--
-- Templates are mutable and unversioned: no draft/published/archived lifecycle
-- and no version column. A template is consulted once, at instantiation, unlike
-- an email template which resolves at send time and is therefore a live
-- dependency. See docs/adr/0004-license-schema-template-and-copy.md.
--
-- No CHECK constraints and no business triggers. Whether a value satisfies its
-- field's declared rules is a service-layer concern, validated by
-- internal/license/rules on every write.

-- License Templates
-- The values live in a single JSONB document rather than in a child table,
-- which is the opposite of the choice license_schema_fields made. The reason
-- that table used rows was that the usage path resolves one reported key to one
-- declared field and needs an indexed lookup. No such path exists here: a
-- template is read whole at instantiation, replaced whole on edit, and diffed
-- whole against an organization's license. A child table would buy an index
-- nothing queries and turn every write into a delete-and-reinsert.
--
-- Keys are license field names; values are whatever the declared field type
-- admits — a number, a boolean, or a string.
CREATE TABLE license_templates (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: ltpl_
    platform_tenant_id VARCHAR(255) NOT NULL,
    product_id VARCHAR(255) NOT NULL,
    name VARCHAR(120) NOT NULL, -- operator-facing, unique within the product
    description TEXT NOT NULL DEFAULT '',
    values_json JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (product_id, name),
    CONSTRAINT fk_license_templates_product_tenant
        FOREIGN KEY (product_id, platform_tenant_id)
        REFERENCES products(id, platform_tenant_id)
        ON DELETE CASCADE
);

-- No separate index on product_id: the UNIQUE (product_id, name) index leads
-- with it, so a lookup by product already has one.
CREATE INDEX idx_license_templates_platform_tenant_id ON license_templates(platform_tenant_id);

CREATE TRIGGER update_license_templates_updated_at BEFORE UPDATE ON license_templates FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
