ALTER TABLE product_event_endpoint_configs
    ADD COLUMN events_json JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE product_event_endpoint_configs
SET events_json = '["organization.created","organization.updated","organization.deleted","organization.membership.created","organization.membership.updated","organization.membership.deleted","workspace.created","workspace.updated","workspace.deleted","organization.api_key.created","organization.api_key.updated","organization.api_key.deleted","product_user.created","product_user.updated","product_user.deleted","organization.license.updated","product.role.created","product.role.updated","product.role.deleted","product.resource_permission.created","product.resource_permission.updated","product.resource_permission.deleted","clerk.user.created","clerk.user.updated","clerk.user.deleted"]'::jsonb;
