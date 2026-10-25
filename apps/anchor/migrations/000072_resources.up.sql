-- The resource registry (ADR-0022 B1-B4): references to a Product's own
-- resources, named by the Product's identifier inside an Organization and a
-- resource type, each under one parent. A NULL parent_resource_id means the
-- Organization itself is the parent. The chain is walked with recursive
-- queries, so no depth is stored.
CREATE TABLE resources (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: res_
    product_id VARCHAR(255) NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    organization_id VARCHAR(255) NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    -- Cascades only so a Product's delete, which removes its types one
    -- statement at a time, never trips on its resources. Deleting a single
    -- type is refused while resources use it (internal/resourcetype).
    resource_type_id VARCHAR(255) NOT NULL REFERENCES product_resource_types(id) ON DELETE CASCADE,
    external_id VARCHAR(200) NOT NULL,
    name VARCHAR(200) NOT NULL,
    -- Cascades for the same reason: an Organization's or a Product's delete
    -- removes resources in an order that ignores their parents. A single
    -- delete is refused while children exist, unless it cascades
    -- (internal/resource).
    parent_resource_id VARCHAR(255) REFERENCES resources(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT resources_organization_type_external_id_key UNIQUE (organization_id, resource_type_id, external_id)
);

CREATE INDEX idx_resources_organization_created_external_id ON resources (organization_id, created_at, external_id, id);
CREATE INDEX idx_resources_parent_resource_id ON resources (parent_resource_id);
CREATE INDEX idx_resources_resource_type_id ON resources (resource_type_id);
CREATE INDEX idx_resources_product_id ON resources (product_id);

CREATE TRIGGER update_resources_updated_at
    BEFORE UPDATE ON resources
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
