-- A Product lets Organizations write custom roles on a resource type (ADR-0022
-- D9). No default: the previous release inserts types without the column, and
-- the code reads NULL as off.
SET LOCAL lock_timeout = '5s';

ALTER TABLE product_resource_types ADD COLUMN allows_custom_roles BOOLEAN;
