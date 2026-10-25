-- Custom roles (ADR-0022 D9, D10). A resource type's custom_roles switch lets
-- Organizations write their own roles scoped to it; it is off, so an existing
-- Product changes nothing. A customized copy of a role template keeps its own
-- bundle when the template changes.
ALTER TABLE product_resource_types ADD COLUMN custom_roles BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE product_roles ADD COLUMN customized BOOLEAN NOT NULL DEFAULT FALSE;
