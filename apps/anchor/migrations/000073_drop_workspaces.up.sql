-- Workspaces are resources of a type each Product declares (ADR-0022 A6).
-- Nothing reads these tables since the release before this one, and every
-- workspace had its resource.
SET LOCAL lock_timeout = '5s';

-- The release before this one stopped reading and writing these tables.
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS workspace_resources;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS workspace_memberships;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS workspaces;

-- lint: allow-backfill only endpoints still subscribed to workspace events; a few rows per product
UPDATE product_event_endpoint_configs
SET events_json = events_json - 'workspace.created' - 'workspace.updated' - 'workspace.deleted'
WHERE events_json ?| ARRAY['workspace.created', 'workspace.updated', 'workspace.deleted'];
