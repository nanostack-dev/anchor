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
