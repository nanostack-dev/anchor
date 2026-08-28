-- Echopoint moved its organization variables and environments off the flows
-- permission family onto environment:*. An API key carries its own permission
-- rows, so a key that could reach those routes yesterday loses them today
-- unless it is granted the replacements.
--
-- The join to product_resource_permissions scopes this to products that define
-- environment:*, which is Echopoint's dev and prod products and nothing else.
INSERT INTO organization_api_key_permissions (api_key_id, organization_id, product_id, permission_name)
SELECT held.api_key_id, held.organization_id, held.product_id, replacement.name
FROM organization_api_key_permissions held
         JOIN (
    VALUES ('flows:read', 'environment:read'),
           ('flows:update', 'environment:create'),
           ('flows:update', 'environment:update'),
           ('flows:delete', 'environment:delete')
) AS mapping (flows_name, environment_name) ON held.permission_name = mapping.flows_name
         JOIN product_resource_permissions replacement
              ON replacement.product_id = held.product_id
                  AND replacement.name = mapping.environment_name
ON CONFLICT DO NOTHING;
