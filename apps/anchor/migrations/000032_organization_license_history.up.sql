-- =============================================
-- Migration 000032: Organization License Change History
-- =============================================
-- Every change to one Organization's license. Rows are immutable: no
-- updated_at and no update trigger, following integration_audit_logs. A
-- correction is a later entry, never an edit of an earlier one.
--
-- No CHECK constraints and no business triggers. Which change types exist, and
-- which columns each one fills, are service-layer concerns.

CREATE TABLE organization_license_changes (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: lchg_
    platform_tenant_id VARCHAR(255) NOT NULL,
    product_id VARCHAR(255) NOT NULL,
    organization_id VARCHAR(255) NOT NULL,
    -- Which license record was changed. Deliberately not a foreign key, for the
    -- same reason organization_licenses.template_id is not one: the entry is a
    -- record of what happened, and stays true whatever becomes of the row it
    -- names. The Organization below is what the cascade hangs from.
    license_id VARCHAR(255) NOT NULL,
    change_type VARCHAR(50) NOT NULL,
    -- Set when a template was stamped onto the Organization: which one.
    template_id VARCHAR(255),
    -- Set when one license field was adjusted: which one.
    field VARCHAR(255),
    old_value_json JSONB,
    new_value_json JSONB,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_organization_license_changes_organization_product
        FOREIGN KEY (organization_id, product_id)
        REFERENCES organizations(id, product_id)
        ON DELETE CASCADE,
    CONSTRAINT fk_organization_license_changes_product_tenant
        FOREIGN KEY (product_id, platform_tenant_id)
        REFERENCES products(id, platform_tenant_id)
        ON DELETE CASCADE
);

-- One Organization's history, newest first, which is the only way it is read.
-- The identifier breaks the tie because the entries of one adjustment share a
-- single changed_at.
CREATE INDEX idx_organization_license_changes_organization
    ON organization_license_changes(organization_id, changed_at DESC, id DESC);

CREATE INDEX idx_organization_license_changes_platform_tenant_id
    ON organization_license_changes(platform_tenant_id);
