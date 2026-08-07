ALTER TABLE organization_licenses DROP CONSTRAINT fk_organization_licenses_template;

ALTER TABLE license_templates DROP CONSTRAINT uq_license_templates_id_product;

DROP INDEX idx_license_templates_product_status;

DROP INDEX uq_license_templates_product_name_active;
ALTER TABLE license_templates ADD CONSTRAINT license_templates_product_id_name_key UNIQUE (product_id, name);

ALTER TABLE license_templates DROP COLUMN status;
