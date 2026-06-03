ALTER TABLE workspace_memberships
    DROP CONSTRAINT workspace_memberships_product_role_id_fkey,
    ADD CONSTRAINT workspace_memberships_product_role_id_fkey
        FOREIGN KEY (product_role_id) REFERENCES product_roles(id) ON DELETE RESTRICT;

ALTER TABLE organization_memberships
    DROP CONSTRAINT organization_memberships_product_role_id_fkey,
    ADD CONSTRAINT organization_memberships_product_role_id_fkey
        FOREIGN KEY (product_role_id) REFERENCES product_roles(id) ON DELETE RESTRICT;
