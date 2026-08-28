-- Removes only what the up migration granted: an environment permission on a
-- key that still holds the flows permission it was derived from. A key granted
-- environment:* on its own is left alone.
DELETE
FROM organization_api_key_permissions granted
WHERE granted.permission_name IN
      ('environment:create', 'environment:read', 'environment:update', 'environment:delete')
  AND EXISTS (SELECT 1
              FROM organization_api_key_permissions held
                       JOIN (
                  VALUES ('flows:read', 'environment:read'),
                         ('flows:update', 'environment:create'),
                         ('flows:update', 'environment:update'),
                         ('flows:delete', 'environment:delete')
              ) AS mapping (flows_name, environment_name)
                            ON held.permission_name = mapping.flows_name
              WHERE held.api_key_id = granted.api_key_id
                AND held.organization_id = granted.organization_id
                AND held.product_id = granted.product_id
                AND mapping.environment_name = granted.permission_name);
