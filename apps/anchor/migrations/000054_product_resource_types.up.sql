-- Resource types scope roles and permissions (ADR-0022 A1-A5, G1). The
-- organization type of each product and the scope of existing roles and
-- permissions are backfilled at startup (internal/resourcetype), because public
-- IDs are KSUIDs, which SQL cannot mint. resource_type_id stays nullable until
-- the ADR-0022 G6 cleanup: a release without resource types still writes rows
-- during a rolling deploy.
CREATE TABLE product_resource_types (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: rtype_
    product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    slug VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    nests_in_itself BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT product_resource_types_product_id_slug_key UNIQUE (product_id, slug)
);

CREATE INDEX idx_product_resource_types_product_created_id
    ON product_resource_types (product_id, created_at, id);

CREATE TRIGGER update_product_resource_types_updated_at
    BEFORE UPDATE ON product_resource_types
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE product_resource_type_parents (
    resource_type_id VARCHAR(255) NOT NULL REFERENCES product_resource_types(id) ON DELETE CASCADE,
    parent_type_id VARCHAR(255) NOT NULL REFERENCES product_resource_types(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (resource_type_id, parent_type_id)
);

CREATE INDEX idx_product_resource_type_parents_parent_type_id
    ON product_resource_type_parents (parent_type_id);

ALTER TABLE product_resource_permissions ADD COLUMN resource_type_id VARCHAR(255);
ALTER TABLE product_resource_permissions
    ADD CONSTRAINT product_resource_permissions_resource_type_id_fkey
        FOREIGN KEY (resource_type_id) REFERENCES product_resource_types(id) NOT VALID;

ALTER TABLE product_roles ADD COLUMN resource_type_id VARCHAR(255);
ALTER TABLE product_roles
    ADD CONSTRAINT product_roles_resource_type_id_fkey
        FOREIGN KEY (resource_type_id) REFERENCES product_resource_types(id) NOT VALID;
