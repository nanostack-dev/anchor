# Case-Insensitive Identifiers

Anchor treats the following product-scoped names as case-insensitive for uniqueness and exact lookup:

- Product names within a platform tenant.
- Product role names within a product.
- Built-in product permission names within a product.
- Product resource permission names within a product.

Case-insensitive matching preserves the original stored casing in responses. When callers assign product resource permissions to roles with different casing, Anchor resolves the input to the canonical stored permission name before persisting the role assignment.

Platform user roles are fixed enum values (`OWNER`, `ADMIN`). Role filtering accepts case variants and returns the canonical enum value.
