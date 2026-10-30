-- Workspaces are resources of a product's own resource type (ADR-0022 A6).
-- Every workspace already has its resource, so the built-in tables go.
DROP TABLE IF EXISTS workspace_resources;
DROP TABLE IF EXISTS workspace_memberships;
DROP TABLE IF EXISTS workspaces;

UPDATE product_event_endpoint_configs
SET events_json = events_json - 'workspace.created' - 'workspace.updated' - 'workspace.deleted'
WHERE events_json ?| ARRAY['workspace.created', 'workspace.updated', 'workspace.deleted'];
