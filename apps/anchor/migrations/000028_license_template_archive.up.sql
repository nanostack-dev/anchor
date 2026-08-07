-- =============================================
-- Migration 000028: Archive license templates, never delete them
-- =============================================
-- An Organization's license names the template it was instantiated from. That
-- statement of what a customer was sold has to stay resolvable for as long as
-- the license does, so a template is withdrawn by archiving it and its row is
-- never removed. Keeping the row is what lets the provenance below become a
-- real foreign key. See docs/adr/0010-license-templates-are-archived.md.
--
-- No CHECK constraint on status: the permitted values are a service-layer
-- concern, as they are for every other status in this schema.

ALTER TABLE license_templates ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE';

-- The name is unique among a Product's ACTIVE templates only. Archiving "Pro"
-- has to free the name, or a withdrawn tier would block its own replacement for
-- ever. A partial index is the only shape that says that.
ALTER TABLE license_templates DROP CONSTRAINT license_templates_product_id_name_key;
CREATE UNIQUE INDEX uq_license_templates_product_name_active
    ON license_templates(product_id, name) WHERE status = 'ACTIVE';

-- Archived templates are read by identifier — a license points at one, and the
-- diff resolves it — so the listing is the only place status narrows, and it
-- narrows within a product.
CREATE INDEX idx_license_templates_product_status ON license_templates(product_id, status);

-- Referenced by the foreign key below.
ALTER TABLE license_templates ADD CONSTRAINT uq_license_templates_id_product UNIQUE (id, product_id);

-- Provenance, now enforced. No ON DELETE clause is what this migration buys: a
-- template row is never deleted, so nothing has to decide what happens to the
-- licenses that name it.
--
-- This statement fails if any license names a template that was hard-deleted
-- before this migration ran. Failing loudly is correct — the alternative is
-- discarding a customer's record of what they were sold.
ALTER TABLE organization_licenses
    ADD CONSTRAINT fk_organization_licenses_template
    FOREIGN KEY (template_id, product_id)
    REFERENCES license_templates(id, product_id);
