CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_resources_resource_type_external_id
    ON resources (resource_type_id, external_id);
