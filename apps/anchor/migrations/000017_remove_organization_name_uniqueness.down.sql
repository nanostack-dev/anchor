ALTER TABLE organizations
    ADD CONSTRAINT organizations_product_id_name_key UNIQUE (product_id, name);
