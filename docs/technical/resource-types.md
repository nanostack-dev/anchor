# Resource types

A Product declares a tree of **resource types** under the built-in `organization` type, and every role and permission is **scoped** to one of them ([ADR-0022](../adr/0022-resource-types-scope-roles-and-permissions.md) A1–A5, C1–C4, D1–D2). The tree and the scopes are stored now; access decisions do not read them yet, so every existing check answers as before.

## The tree

| Rule | Where it is enforced | Refusal |
| --- | --- | --- |
| Every Product has exactly one `organization` type, the root. It is created with the Product and can't be renamed, reparented or deleted. | `internal/resourcetype/service` | `409 BUILT_IN_RESOURCE_TYPE` |
| A slug matches `^[a-z][a-z0-9_]{0,48}$`, is unique in its Product and never changes. | `internal/domain/resourcetype` | `400 RESOURCE_TYPE_SLUG_INVALID`, `409 RESOURCE_TYPE_SLUG_DUPLICATE` |
| Every other type lists one to ten parent types it may sit under. It never lists itself; `nests_in_itself` lets an instance sit under its own type. | `Tree.Validate` | `400 RESOURCE_TYPE_SELF_PARENT`, `400 RESOURCE_TYPE_NOT_FOUND` |
| No two types are each other's ancestors. | `Tree.Validate` | `400 RESOURCE_TYPE_CYCLE` |
| The tree is at most 5 levels deep, counting `organization`, through each type's deepest parent. | `Tree.Validate`, `resourcetype.MaxDepth` | `400 RESOURCE_TYPE_TOO_DEEP` |
| A Product declares at most 50 types besides `organization`. | `resourcetype.MaxOwnTypes` | `409 RESOURCE_TYPE_LIMIT_REACHED` |
| A type that another type sits under, or that a role or permission is scoped to, can't be deleted. | `CountUsage`, and the foreign keys as the race backstop | `409 RESOURCE_TYPE_IN_USE` with `metadata.blocked_by` |
| A change that would leave a role bundling a permission no longer at or below its scope is refused. | `checkReshape` | `409 RESOURCE_TYPE_IN_USE` with `metadata.role_ids` |

`PUT /v1/products/{product_id}/resource-types` replaces every type except `organization` in one transaction, matched by slug: it is what the admin UI publishes a draft tree with. Every tree write holds the catalog lock, a `FOR UPDATE` on the Product's `organization` row; role and permission writes hold it `FOR SHARE`, so no tree write can move a scope boundary while a bundle is being checked. Each type created, updated or deleted emits `product.resource_type.created|updated|deleted` with `resource_type_id` and `resource_type`.

## Scopes on roles and permissions

- Creating a permission or a role requires `resource_type`. Anchor never picks one: a missing scope is `400 RESOURCE_TYPE_REQUIRED`, an undeclared one `400 RESOURCE_TYPE_NOT_FOUND`.
- A scope never changes. An update naming another one is `409 PERMISSION_SCOPE_IMMUTABLE` or `409 ROLE_SCOPE_IMMUTABLE`; naming the same one is accepted.
- A role bundles only permissions scoped to its own type or to a type below it, on create, on a permission replacement and on a single assignment. Anything else is `400 PERMISSION_OUTSIDE_ROLE_SCOPE`.
- Role and permission searches filter on `resource_types`.
- `scope_modifier` is deprecated: it is still accepted and returned, nothing evaluates it, and the cleanup (ADR-0022 G6) removes it.

## Backfill

Migration `000054` adds the tables and a nullable `resource_type_id` on `product_roles` and `product_resource_permissions`. Public IDs are KSUIDs, which SQL cannot mint, so the backfill is code: on every start, under the `resource_types:organization-backfill` advisory lock, `RegisterOrganizationBackfill` gives each Product its `organization` type and scopes its unscoped roles and permissions to it (ADR-0022 G1). It is idempotent.

The column stays nullable until the cleanup because a rolling deploy runs the previous release beside this one, and that release still writes unscoped rows. Reads treat a null scope as `organization`, which is what the next start's backfill writes. A Product created by the previous release gets its `organization` type the first time a write or read needs the tree.

## Tests

`resource_type_*_test.go` in `apps/anchor/cmd/it/ct` cover every rule above with one test per case, plus the cursor search case in `TestCursorSearches` and the duplicate-slug race in `TestConcurrentSameNameWritesReturnConflict`. `TestOrganizationBackfill` in `apps/anchor/cmd/it/service` covers the backfill, which no route reaches.
