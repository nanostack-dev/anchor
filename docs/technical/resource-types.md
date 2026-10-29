# Resource types

A Product declares a tree of **resource types** under the built-in `organization` type, and every role and permission is **scoped** to one of them ([ADR-0022](../adr/0022-resource-types-scope-roles-and-permissions.md) A1–A5, C1–C4, D1–D2). The model compiles the tree and the scopes, and [access evaluation](access-evaluation.md#resources) takes any declared type; every check on an organization or a workspace answers as before.

## The tree

| Rule | Where it is enforced | Refusal |
| --- | --- | --- |
| Every Product has exactly one `organization` type, the root. It is created with the Product and can't be renamed, reparented, deleted or declared again. | `internal/resourcetype/service` | `409 BUILT_IN_RESOURCE_TYPE` |
| A slug matches `^[a-z][a-z0-9_]{0,48}$`, is unique in its Product and never changes. | `internal/domain/resourcetype` | `400 RESOURCE_TYPE_SLUG_INVALID`, `409 RESOURCE_TYPE_SLUG_DUPLICATE` |
| Every other type lists one to ten parent types it may sit under. It never lists itself; `nests_in_itself` lets an instance sit under its own type. | `validateItem`, `Tree.Validate` | `400 RESOURCE_TYPE_PARENT_REQUIRED`, `400 VALIDATION_ERROR` past ten, `400 RESOURCE_TYPE_SELF_PARENT`, `400 RESOURCE_TYPE_NOT_FOUND` (also when a published tree leaves out a type another listed type sits under) |
| No two types are each other's ancestors. | `Tree.Validate` | `400 RESOURCE_TYPE_CYCLE` |
| The tree is at most 5 levels deep, counting `organization`, through each type's deepest parent. | `Tree.Validate`, `resourcetype.MaxDepth` | `400 RESOURCE_TYPE_TOO_DEEP` |
| A Product declares at most 50 types besides `organization`. | `resourcetype.MaxOwnTypes` | `409 RESOURCE_TYPE_LIMIT_REACHED` on create; `400 VALIDATION_ERROR` for a published tree of more than 50 |
| A type that another type sits under, that a role or permission is scoped to, or that registered [resources](resources.md) use, can't be deleted. `blocked_by.resources` appears only when resources block. | `CountUsage`; a foreign key violation also answers the 409, with zero counts, because it aborts the transaction | `409 RESOURCE_TYPE_IN_USE` with `metadata.blocked_by` |
| A parent type can't be removed, and `nests_in_itself` can't be turned off, while a registered resource of the type sits under one of that type. | `checkResourceParents`, on the update and the tree `PUT` | `409 RESOURCE_TYPE_IN_USE` with `metadata.parent_type` and `metadata.blocked_by.resources` |
| A change that would leave a role bundling a permission no longer at or below its scope is refused. | `checkReshape` | `409 RESOURCE_TYPE_IN_USE` with `metadata.role_ids` |

`PUT /v1/products/{product_id}/resource-types` replaces every type except `organization` in one transaction, matched by slug: it is what the admin UI publishes a draft tree with. It writes only the types that changed, so publishing the same tree twice changes nothing. Every tree write reads the Product's `organization` row with `transactor.ForNoKeyUpdate`, and role and permission writes read it with `transactor.ForShare`, so no tree write can move a scope boundary while a role's permission scopes are being checked. Rows that reference the type still insert during a tree write. Each type created, updated or deleted emits `product.resource_type.created|updated|deleted` with `resource_type_id` and `resource_type`.

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
- **Inspector.** Beside the canvas (below it on a phone): name, slug, allowed parent types (only those that keep the tree acyclic and at most five deep; the last parent can't be removed), nests in itself, and the counts of roles and permissions scoped to the type (role and permission searches filtered by `resource_types`). In read mode, *View roles* and *View permissions* open both pages filtered to the type (`?resource_type=<slug>`); edit mode hides them so a draft is never left by accident. *Delete* is disabled with its reasons while types sit under the type or roles or permissions are scoped to it; Organization is locked and has no Delete.
- **Custom roles switch.** Every type's inspector shows its `allows_custom_roles` switch (D9): read-only text in read mode, a switch in edit mode. The tree `PUT` doesn't carry the switch, so it is part of the draft (the summary counts it) and *Publish* first saves each changed switch with `PATCH /v1/products/{product_id}/resource-types/{resource_type_id}`, then sends the tree if the tree changed too. A refused switch keeps the draft and shows Anchor's message. The two calls are not one transaction: when the switch is saved and the tree is then refused, a *Custom roles saved* notice has already said so, the draft keeps only the tree changes, and *Discard* doesn't undo the switch. Every published type's switch can change. A type the draft adds has its switch disabled until it is published, because the tree `PUT` doesn't carry the switch and the type has no ID to `PATCH` before then.
- **Keyboard.** Arrows move to the closest parent, the leftmost child or the neighbour on the same level; Enter renames; `N` adds a child; Delete asks to delete. Keyboard changes never animate.
- **Motion.** A node added with the pointer fades and scales in (150 ms); layout changes interpolate node positions for 200 ms in JavaScript so edges follow each frame. Nodes never get a CSS transform transition. With reduced motion, layout changes are instant and the fade stays.
- **Focus.** Entering edit mode focuses the selected type (or Organization); publishing and discarding focus *Edit resource types*; a delete focuses the parent. The editor is keyed by Product, so switching Product drops the draft, and it is read-only while a publish is in flight.
- **Refusals.** A refused publish keeps the draft and explains each error code in plain words until the draft changes; for `RESOURCE_TYPE_IN_USE` with `metadata.role_ids` it names the managed roles to fix and counts the custom roles of Organizations apart, since each Organization changes its own.

### Roles and permissions pages

DR-F4 scopes the catalog pages. Code: `anchor-ui/src/components/product/roles/`, `anchor-ui/src/components/product/permissions/` and `resource-type-scope.ts`.

- **Tree order.** Everywhere a list of types appears (pickers, filters, permission groups) it reads Organization first, then each level of the tree from the top, in the order Anchor returns the types (`treeOrder`). A type's level is its deepest path from Organization.
- **Creating a role** is a route, `/products/resources/roles/new`, in four steps: *Scope*, *Details*, *Permissions*, *Review*. The scope step starts empty and can't continue until a resource type is chosen; nothing is preselected, Organization included. It shows which types the role can hold and which are not offered. The permissions step loads only the permissions of the scope and the types below it (`resource_types` filter, left out for Organization, which covers every type; every page is read by cursor) and groups them by type in tree order, so a permission outside the scope can't be chosen. A scope with no permission says so and links to a new permission pre-scoped to it; the role may still be created without permissions. Editing a role skips the scope step and shows the scope read-only; it offers the same filtered list, and warns that saving drops any permission the role holds outside its scope.
- **Creating a permission** is a route too, `/products/resources/new-permission` (outside `/permissions/` because a permission may be named `new`). The resource type comes first and starts empty unless the page was opened for a type; choosing one pre-fills the name with `{slug}:` (C2) while the name is empty or still that prefix.
- **The roles table** lists the roles the Product manages with their resource type and permission count, and filters on resource types.
- **The permissions table** shows each permission's resource type and the managed roles that hold it: one roles search per page, filtered on the page's permission names (`filter.permissions`) and read to its last cursor page, never one call per row. It filters on resource types, and *Create Permission* keeps a single type filter.
- **URL filters.** Both tables keep their resource type filter in `?resource_type=a,b` (valid slugs, at most 50, the search filter's limit), so the canvas links and reloads open the same view.
- **An Organization's roles.** The Organization page (`/organizations/license/$organizationId`) has *License* and *Roles* sections. *Roles* lists the roles its members may hold from `searchOrganizationRoles`, read-only, each marked *Managed* or *Custom*, with its resource type and permission count. Customers change them from the Product's own settings through the Organization roles API, so the admin UI has no controls for them.

<a id="backfill"></a>

## History

Migration `000054` adds the tables and a nullable `resource_type_id` on `product_roles` and `product_resource_permissions`. Public IDs are KSUIDs, which SQL cannot mint, and a rolling deploy runs the previous release beside the new one, so the release that adds resource types backfilled in code: on every start, a startup job gave each Product its `organization` type and scoped its unscoped roles and permissions to it (ADR-0022 G1).

Once that release runs in production, migration `000058` closes G1. It scopes any role or permission the backfill missed to its Product's `organization` type, then sets `resource_type_id` `NOT NULL`; the backfill and every read that took a null scope for `organization` are gone. A row whose Product has no `organization` type stops the migration. Production starts with `autoFixDirty`, which marks a failed migration as applied, so the resource type module refuses to start while either column is still nullable (`CheckSchema`). To recover, run `migrate force 57`, fix the data, then redeploy.

A role the release before resource types gave an organization permission outside its scope keeps it: renaming a type stays possible, and a change to parent types or nesting, on one type or through the tree `PUT`, is refused with `role_ids` until that role is fixed, since it checks every role of the Product. Creating a Product seeds its `organization` type in the same transaction. A tree read or write still seeds it when a Product lacks it, which also gives the catalog lock its row.

## Tests

`resource_type_*_test.go` in `apps/anchor/cmd/it/ct` cover every rule above with one test per case, plus the cursor search case in `TestCursorSearches` and the duplicate-slug race in `TestConcurrentSameNameWritesReturnConflict`. `TestResourceTypeSchemaCheck` in `apps/anchor/cmd/it/service` covers the startup schema check.

The admin UI has unit tests for the draft model, layout and error messages (`resource-type-tree.test.ts`) and for tree order and scopes (`resource-type-scope.test.ts`), stories with worst cases and interactions (`ResourceTypesEditor.stories.tsx`, `ScopedPermissionPicker.stories.tsx`, `ProductRoleEditor.stories.tsx`, the roles, permissions and Organization roles table stories), and browser scenarios in `anchor-ui/e2e/features/access/resource-types.e2e.ts` and `scoped-roles.e2e.ts`.
