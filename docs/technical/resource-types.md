# Resource types

A Product declares a tree of **resource types** under the built-in `organization` type, and every role and permission is **scoped** to one of them ([ADR-0022](../adr/0022-resource-types-scope-roles-and-permissions.md) A1–A5, C1–C4, D1–D2). The tree and the scopes are stored now; access decisions do not read them yet, so every existing check answers as before.

## The tree

| Rule | Where it is enforced | Refusal |
| --- | --- | --- |
| Every Product has exactly one `organization` type, the root. It is created with the Product and can't be renamed, reparented, deleted or declared again. | `internal/resourcetype/service` | `409 BUILT_IN_RESOURCE_TYPE` |
| A slug matches `^[a-z][a-z0-9_]{0,48}$`, is unique in its Product and never changes. | `internal/domain/resourcetype` | `400 RESOURCE_TYPE_SLUG_INVALID`, `409 RESOURCE_TYPE_SLUG_DUPLICATE` |
| Every other type lists one to ten parent types it may sit under. It never lists itself; `nests_in_itself` lets an instance sit under its own type. | `validateItem`, `Tree.Validate` | `400 RESOURCE_TYPE_PARENT_REQUIRED`, `400 VALIDATION_ERROR` past ten, `400 RESOURCE_TYPE_SELF_PARENT`, `400 RESOURCE_TYPE_NOT_FOUND` (also when a published tree leaves out a type another listed type sits under) |
| No two types are each other's ancestors. | `Tree.Validate` | `400 RESOURCE_TYPE_CYCLE` |
| The tree is at most 5 levels deep, counting `organization`, through each type's deepest parent. | `Tree.Validate`, `resourcetype.MaxDepth` | `400 RESOURCE_TYPE_TOO_DEEP` |
| A Product declares at most 50 types besides `organization`. | `resourcetype.MaxOwnTypes` | `409 RESOURCE_TYPE_LIMIT_REACHED` on create; `400 VALIDATION_ERROR` for a published tree of more than 50 |
| A type that another type sits under, or that a role or permission is scoped to, can't be deleted. An Organization's copies of a role template are not counted: they follow their template. | `CountUsage`; a foreign key violation also answers the 409, with zero counts, because it aborts the transaction | `409 RESOURCE_TYPE_IN_USE` with `metadata.blocked_by` |
| A change that would leave a role bundling a permission no longer at or below its scope is refused. | `checkReshape` | `409 RESOURCE_TYPE_IN_USE` with `metadata.role_ids` |

`PUT /v1/products/{product_id}/resource-types` replaces every type except `organization` in one transaction, matched by slug: it is what the admin UI publishes a draft tree with. It writes only the types that changed, so publishing the same tree twice changes nothing. Every tree write holds the catalog lock, a `FOR NO KEY UPDATE` on the Product's `organization` row; role and permission writes hold it `FOR SHARE`, so no tree write can move a scope boundary while a bundle is being checked. Each type created, updated or deleted emits `product.resource_type.created|updated|deleted` with `resource_type_id` and `resource_type`.

## Custom roles switch

Each type has `custom_roles`, off by default. On, it lets Organizations create custom roles scoped to the type and customize their copies of its role templates ([Organization roles](organization-roles.md)). The built-in `organization` type accepts a change of this switch and nothing else (`409 BUILT_IN_RESOURCE_TYPE`). The Product's own types answer `400 CUSTOM_ROLES_NOT_AVAILABLE` to `true` until ADR-0022 D9 phase 2. A change emits `product.resource_type.updated`.

## Scopes on roles and permissions

- Creating a permission or a role requires `resource_type`. Anchor never picks one: a missing scope is `400 RESOURCE_TYPE_REQUIRED`, an undeclared one `400 RESOURCE_TYPE_NOT_FOUND`.
- A scope never changes. An update naming another one is `409 PERMISSION_SCOPE_IMMUTABLE` or `409 ROLE_SCOPE_IMMUTABLE`; naming the same one is accepted.
- A role bundles only permissions scoped to its own type or to a type below it, on create, on a permission replacement and on a single assignment. Anything else is `400 PERMISSION_OUTSIDE_ROLE_SCOPE`.
- Role and permission searches filter on `resource_types`.
- `scope_modifier` is deprecated: it is still accepted and returned, nothing evaluates it, and the cleanup (ADR-0022 G6) removes it.

## Admin UI

**Product › Roles & Permissions › Resource types** (`/products/resources/types`) draws the tree on a canvas ([ADR-0022](../adr/0022-resource-types-scope-roles-and-permissions.md) DR-F1–DR-F3). Code: `anchor-ui/src/components/product/resource-types/`.

- **Layout.** `@dagrejs/dagre` lays the tree out top down on every change; nothing about positions is stored. Nodes can't be dragged, an edge goes from each parent to its child, and a self-nesting type shows a "nests in itself" badge instead of a loop. Draft types are laid out in the order Anchor returns them after publish (creation order, then slug), so publishing keeps every node in place. The first view fits the tree but zooms out no further than 50 %, so a wide tree stays readable; *Fit to view* fits it whole.
- **Read, then edit.** The page opens read-only. *Edit resource types* starts a local draft; the summary line counts types added, renamed, changed and deleted. *Publish* sends the whole draft with the `PUT` tree call and *Discard* restores the published tree. A draft with an empty name or an invalid or duplicate slug can't be published.
- **Adding.** In edit mode, hovering, focusing or selecting a node shows a `+` below it. It inserts a child with a focused name field: Enter keeps it, Escape drops it. The slug follows the name until it is edited, and stays editable only until the type is published. A new type can't take the slug of a type the same draft deletes: Anchor matches by slug and would keep the old type. `+` is disabled at 50 types and below level 5.
- **Inspector.** Beside the canvas (below it on a phone): name, slug, allowed parent types (only those that keep the tree acyclic and at most five deep; the last parent can't be removed), nests in itself, and the counts of roles and permissions scoped to the type (role and permission searches filtered by `resource_types`). *Delete* is disabled with its reasons while types sit under the type or roles or permissions are scoped to it; Organization is locked and has no Delete.
- **Keyboard.** Arrows move to the closest parent, the leftmost child or the neighbour on the same level; Enter renames; `N` adds a child; Delete asks to delete. Keyboard changes never animate.
- **Motion.** A node added with the pointer fades and scales in (150 ms); layout changes interpolate node positions for 200 ms in JavaScript so edges follow each frame. Nodes never get a CSS transform transition. With reduced motion, layout changes are instant and the fade stays.
- **Focus.** Entering edit mode focuses the selected type (or Organization); publishing and discarding focus *Edit resource types*; a delete focuses the parent. The editor is keyed by Product, so switching Product drops the draft, and it is read-only while a publish is in flight.
- **Refusals.** A refused publish keeps the draft and explains each error code in plain words until the draft changes; for `RESOURCE_TYPE_IN_USE` with `metadata.role_ids` it names the roles to fix.

## Backfill

Migration `000054` adds the tables and a nullable `resource_type_id` on `product_roles` and `product_resource_permissions`. Public IDs are KSUIDs, which SQL cannot mint, so the backfill is code: on every start, under the `resource_types:organization-backfill` advisory lock, `RegisterOrganizationBackfill` gives each Product its `organization` type and scopes its unscoped roles and permissions to it (ADR-0022 G1). It is idempotent.

The column stays nullable until the cleanup because a rolling deploy runs the previous release beside this one, and that release still writes unscoped rows. It also checks no bundle against scopes, so for that window an organization permission could be assigned to a role of another type; the next tree change that depends on it is then refused with `role_ids`, which names the role to fix. Reads treat a null scope as `organization`, which is what the next start's backfill writes. A Product created by the previous release gets its `organization` type the first time a write or read needs the tree.

## Tests

`resource_type_*_test.go` in `apps/anchor/cmd/it/ct` cover every rule above with one test per case, plus the cursor search case in `TestCursorSearches` and the duplicate-slug race in `TestConcurrentSameNameWritesReturnConflict`. `TestOrganizationBackfill` in `apps/anchor/cmd/it/service` covers the backfill, which no route reaches.

The admin UI has unit tests for the draft model, layout and error messages (`resource-type-tree.test.ts`), stories with worst cases and interactions (`ResourceTypesEditor.stories.tsx`), and browser scenarios in `anchor-ui/e2e/features/access/resource-types.e2e.ts`.
