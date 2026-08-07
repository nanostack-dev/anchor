-- =============================================
-- Migration 000027: Organization License
-- =============================================
-- One Organization's own copy of a license template's values. An Organization
-- has at most one.
--
-- The values are copied, not referenced. Editing a template afterwards cannot
-- change a live customer, and this row is the historical statement of what that
-- customer was sold. It is also why per-organization deviation needs no
-- override layer: a bespoke limit for one customer is an edit to this row.
-- See docs/adr/0004-license-schema-template-and-copy.md.
--
-- No CHECK constraints and no business triggers. Whether a value satisfies its
-- license field's declared rules is a service-layer concern, validated by
-- internal/license/rules on every write.

-- The license carries product_id as well as organization_id, so every statement
-- is scoped the same way the rest of the licensing subsystem is. This unique
-- constraint is what lets a composite foreign key keep the two from disagreeing.
ALTER TABLE organizations ADD CONSTRAINT uq_organizations_id_product UNIQUE (id, product_id);

CREATE TABLE organization_licenses (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: lic_
    platform_tenant_id VARCHAR(255) NOT NULL,
    product_id VARCHAR(255) NOT NULL,
    organization_id VARCHAR(255) NOT NULL UNIQUE,
    -- Provenance: which template this Organization was stamped from, and when.
    -- Deliberately not a foreign key. A deleted template does not make it untrue
    -- that this customer was sold that tier on that date, and the values here
    -- stopped depending on the template the moment they were copied.
    template_id VARCHAR(255) NOT NULL,
    -- Same single-document choice license_templates made, for the same reason:
    -- a license is read whole, adjusted whole, and diffed whole against its
    -- template. Keys are license field names; values are whatever the declared
    -- field type admits — a number, a boolean, or a string.
    values_json JSONB NOT NULL DEFAULT '{}',
    -- When the copy was taken. Equal to created_at today, and separate from it
    -- because re-instantiating an Organization onto a different template moves
    -- this one and leaves created_at alone.
    instantiated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_organization_licenses_organization_product
        FOREIGN KEY (organization_id, product_id)
        REFERENCES organizations(id, product_id)
        ON DELETE CASCADE,
    CONSTRAINT fk_organization_licenses_product_tenant
        FOREIGN KEY (product_id, platform_tenant_id)
        REFERENCES products(id, platform_tenant_id)
        ON DELETE CASCADE
);

-- No separate index on organization_id: the UNIQUE constraint above creates one,
-- and a license is always addressed by its Organization.
CREATE INDEX idx_organization_licenses_platform_tenant_id ON organization_licenses(platform_tenant_id);

CREATE TRIGGER update_organization_licenses_updated_at BEFORE UPDATE ON organization_licenses FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
