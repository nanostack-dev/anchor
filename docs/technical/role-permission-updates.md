# Product-role permission updates

`PUT /v1/products/{product_id}/roles/{role_id}` requires the role name and treats its optional `permissions` property as a replacement:

| Request | Effect |
| --- | --- |
| Property omitted | Keep the existing grants. |
| `"permissions": []` | Remove every grant. |
| Nonempty permission array | Replace the current grants with the validated names. |

The generated Go SDK represents the optional array as `*[]string`. Leave the pointer nil to retain grants; pass a pointer to `[]string{}` to clear them. The schema is the owner of this distinction; regenerate the server, UI client and Go SDK when changing it.

Role creation and the separate assign/remove endpoints retain their existing contracts. The current UI role wizard requires at least one permission; the API supports an empty set, and the role detail view displays that persisted state.

`TestProductRole_UpdatePermissionReplacementStates` verifies the three HTTP operations against persisted state. `TestProductRoleUpdatePermissionJSON` verifies SDK serialization, and `access.role-empty-update` checks the persisted empty state after a browser reload.

## Name resolution and input ownership

Role create, update and permission assignment resolve resource-permission names through the product-scoped resource catalog. The private resolver returns a copied slice with canonical spelling. It preserves requested order, duplicates, IDs and scope, and never rewrites the caller's slice, including on a missing-name error. The existing case-insensitive canonicalizer chooses the last catalog spelling for a duplicate case-folded key; role resolution retains that policy.

The role cap, catalog lookup and missing-name errors stay in the role service. API-key services reuse the pure name algorithm while retaining their own catalog and validation boundaries.

## Membership permission projections

Membership reads use `organization.WithoutRolePermissions` when they do not request role permissions. The existing boolean interface and explicit include inputs retain the same meaning.

Both organization-member and user-organization API views use one permission projection policy: omit the property when permissions were not requested; emit `[]` when requested but empty. API-key and membership mappings retain their explicit tenant-specific domain/database conversions and pass existing converter methods directly to collection/query helpers.
