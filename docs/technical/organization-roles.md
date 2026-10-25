# Organization roles

Roles belong to Organizations ([ADR-0022](../adr/0022-resource-types-scope-roles-and-permissions.md) D8). A Product defines **role templates** under `/v1/products/{product_id}/roles`, and every Organization holds its own **copy** of each template. Members and invitations hold the copy. Access decisions read the copy's permissions, which equal the template's, so every check answers as before.

This change shipped in two releases, because a rolling deploy runs the previous release beside the next one. The first release added the columns and read and wrote copies correctly without making any. The second makes the copies and moves memberships onto them.

## Storage and IDs

Templates and Organization roles share `product_roles`. A template has no `organization_id`; a copy has one and points back with `template_id`. Sharing the table keeps every foreign key that names a role (`organization_memberships`, `organization_invitations`, `workspace_memberships`) valid during a rolling deploy.

A copy is addressed by its **template's ID** inside its Organization: `public_id`, a virtual column, is `COALESCE(template_id, id)`. Responses (memberships, invitations, the access decision context, Organization roles) show it, and requests may send it, so the role IDs released clients hold keep working. Memberships store the copy's own row ID. Invitations store the public ID and resolve it again on accept. Member searches filter `role_ids` on the public ID. A role ID of another Organization names no role: `404 ROLE_NOT_FOUND` on a path, `400 ROLE_NOT_FOUND_IN_REQUEST` in a body.

Template routes, searches and resource type usage leave copies out (`organization_id IS NULL`).

[Role assignments on resources](resources.md#role-assignments) hold an Organization's role the same way: a template's ID in `role_ids` resolves to the copy (`Resolve`), and `resource_role_assignments.role_id` stores the copy's row ID, or a custom role's. The role must be scoped to the resource's type.

## Copies

A copy is made when it is first needed:

| Moment | Where |
| --- | --- |
| An Organization is created | `OrganizationService.Create` and `CreateWithMember` call `EnsureCopies`. The founding member joins on the template and moves onto the copy in the same transaction. |
| A template ID is resolved in an Organization (member add or update, invitation create or update) | `organizationrole.Service.Resolve`, in the write's transaction |
| An Organization's roles are read | `Get`, `Search` and `Update` call `EnsureCopies`, so a read under `organization_role:read` may write the copies it lacks |
| Every start, for every Organization, under the `organization_roles:copy-backfill` lock | `RegisterCopyBackfill` |

The ADR asks for a background job when a template is added. Copies made on first need give the same observable result: an Organization that never uses the new template has nothing to copy, and listing its roles makes the copy first.

`EnsureCopies` copies in batches of 500 until nothing is missing, then moves the Organization's memberships that still hold a template onto the copy.

A template change reaches its copies in the same transaction: a rename or a description change, and every permission assigned, unassigned or replaced (`SyncCopies`, which rewrites only the copies that differ). The copy keeps the template's key. Deleting a template no member holds deletes its copies; a template whose copy a member holds, through a membership or a role assignment on a resource, or that a pending invitation names, answers `409 ROLE_IN_USE` as before. The in-use counts are `CountMembershipAssignments` for a template and its copies, and `CountHolders` for an Organization's own role; both include `resource_role_assignments`.

Locks keep copies consistent with their templates:

- Every template write takes the catalog lock first when it needs one, then locks the template's row and its copies' rows in ID order (`LockTemplate`). A write that moves the Product's default locks the current default template with it, in the same order (`LockTemplates`).
- A copy being made reads its templates under a share lock, in ID order, so it holds the permissions from before a template change or after it, never a mix.
- A write that names a copy holds its template under a share lock (`Resolve`). A member added while the template is deleted then answers `400 ROLE_NOT_FOUND_IN_REQUEST` instead of joining a role that the delete removes.

## Startup backfill

`BackfillCopies` runs beside the start, not before it, because the first run copies every template into every Organization. It runs every step for one Product in its own transaction, so an interrupted run leaves no copy without its permissions. A Product that fails is logged and the next one runs.

1. Every template the previous release wrote without a key gets one: its name in lower case, or its ID when another template already holds that key.
2. Every Organization gets a copy of every template, a batch of 500 per transaction, until none is missing.
3. Every membership that still holds a template moves onto its Organization's copy.
4. Every copy is brought in step with its template, which repairs a template change the previous release made during the deploy.

## Keys and the default role

- A template has a `key`, unique in its Product regardless of case (`409 ROLE_KEY_DUPLICATE`). Omitted on create, it is the name in lower case. It never changes, and every copy keeps it. A create whose name another template holds answers `409 ROLE_NAME_DUPLICATE` as before, even when its key clashes too.
- A Product may mark one `organization`-scoped template as its default (`is_default`); marking another moves the default (D5) and emits `product.role.updated` for the template that lost it. A template update writes `is_default` only when it names it. A default scoped to another type answers `400 DEFAULT_ROLE_NOT_ORGANIZATION_SCOPED`, and a concurrent change `409 DEFAULT_ROLE_CONFLICT`.
- An Organization may choose any of its `organization`-scoped roles as its own default with `PATCH /v1/products/{product_id}/organizations/{organization_id}/roles/{role_id}` (D10.3). It emits `organization.role.updated` for each role whose default changed, including the role that was the default before.
- An Organization role's `is_default` shows the effective default: the Organization's own choice, or else its copy of the Product's default template. A PATCH compares with that effective default: `true` on the role that already is the default changes nothing, so an Organization that never chose keeps following the Product's default. `false` on the Organization's own choice returns it to the template's copy; on that copy itself it changes nothing.
- Adding a member, creating an Organization with a founding member, or creating an invitation without `role_id` uses the Organization's default. With none, the call answers `400 ROLE_REQUIRED`.

Routes under `/organizations/{organization_id}/roles` take the `organization_role:read`, `:create`, `:update` and `:delete` API key scopes. Role assignments on resources use the list-shaped `role_ids` and `roles` contract (ADR-0022 D3); memberships keep their single `role_id`, and `role_keys` is not accepted yet.

## Custom roles and customized copies

A resource type's `custom_roles` switch lets each Organization shape its own roles (ADR-0022 D9, D10). It is off, so an existing Product changes nothing. For now only `organization` accepts it (phase 1); any other type answers `400 CUSTOM_ROLES_NOT_AVAILABLE`.

| Request | Switch off | Switch on |
| --- | --- | --- |
| `POST .../organizations/{organization_id}/roles` | `400 CUSTOM_ROLES_DISABLED` | `201` custom role, `organization.role.created` |
| `PATCH` name, description or permissions of a copy | `400 CUSTOM_ROLES_DISABLED` | the copy is `customized` and keeps its bundle when the template changes |
| `PATCH` name, description or permissions of a custom role | `400 CUSTOM_ROLES_DISABLED` | `200`, `organization.role.updated` |
| `PATCH` `is_default` only | allowed | allowed |
| `POST .../roles/{role_id}/reset` | allowed | allowed |
| `DELETE` of a custom role | allowed | allowed |
| `DELETE` of a template a member holds, on the Organization or on a resource | `409 ROLE_IN_USE` | its copies that are held, invited or customized become custom roles |

- A custom role names its `resource_type`; Anchor never picks one (`400 RESOURCE_TYPE_REQUIRED`). Its permissions follow the same scope rule as a template's (D1).
- An Organization holds at most 50 custom roles (`409 CUSTOM_ROLE_LIMIT_REACHED`), counted under a lock on the Organization's row. Copies that become custom roles when their template is deleted count too, and may take it past 50; nobody loses access over a limit.
- One key names one role in an Organization. A custom role's key may not equal another role of the Organization or any role template of the Product, since each template's copy holds its key. A template's key may not equal any Organization's custom role. Both sides hold an advisory lock on the key while they check, so the two can't race (`409 ROLE_KEY_DUPLICATE`).
- A customized copy keeps its own name, description and permissions when its template changes; `SyncCopies` skips it. A reset brings back the template's and clears `customized`. A copy can't be deleted (`409 TEMPLATE_COPY_NOT_DELETABLE`), and only a copy can be reset (`409 NOT_A_TEMPLATE_COPY`).
- A custom role a member holds, on the Organization or on a resource, or that a pending invitation names, can't be deleted (`409 ROLE_IN_USE`). A delete locks the role's row before it counts, and a write that names a custom role holds it under a key share lock, so a member added during the delete answers `400 ROLE_NOT_FOUND_IN_REQUEST` instead of joining a role the delete removes. Deleting the Organization's chosen default returns it to the template's copy and emits `organization.role.updated` for that copy.
- A change or reset of a role locks its row first, so it works from the role as a concurrent template change left it.
- A template whose key one of an Organization's custom roles holds is not copied into that Organization; a write naming it there answers `400 ROLE_NOT_FOUND_IN_REQUEST`. Anchor refuses such a key, so this happens only with a custom role made while a release that didn't check custom keys ran.
- Deleting a template with the switch on first gives every Organization with a member or a pending invitation on it its copy. Then the copies that are held (by a membership or a resource role assignment), invited or customized become custom roles, with their own row ID as their ID; pending invitations move onto them, and role assignments keep pointing at the same row. A copy that was its Organization's default through the Product's default template stays its default. Each emits `organization.role.created`; the members and invitations that now show the new ID emit nothing, so a consumer re-reads the Organization's members after that event. The other copies are deleted with the template.
- The detach follows the `organization` type's switch, whatever the template's scope. A detached copy scoped to another type can't be changed until that type gets its own switch (phase 2), but it can be reset, deleted and assigned.
- Turning the switch off keeps every custom role and customized copy working; it only refuses new ones and new changes. A template deleted while the switch is off removes its copies as before, customized ones included, unless a member holds one (`409 ROLE_IN_USE`).
- The tree reshape check (resource types) reads the bundles of custom roles and customized copies too.

## Admin UI

An Organization's page in the admin UI has a read-only *Roles* section that lists its roles marked template, customized or custom ([Resource types](resource-types.md#roles-and-permissions-pages)). The admin UI never changes them: the Product calls the routes above from its own settings page.

## Rollback

Migrations `000061` and `000062` replace the product-wide name guard with a guard on template names. A copy shares its template's name, so their down migrations succeed only after memberships move back to the templates and the copies are deleted.

## Tests

`organization_custom_role_test.go` covers each switch state, the custom role limit, keys, customize and reset, deletes and template deletes with the switch on. `organization_role_test.go` in `apps/anchor/cmd/it/ct` covers the copies, propagation of grants and revokes to every Organization through the access evaluation, the race between a copy and a template change, a member added while its template is deleted, keys, both defaults with every switch state, their races and a default moved while an Organization copies, invitations, founding members, another Organization's role IDs, the search filter limits and deletion. `TestCursorSearches` has the `searchOrganizationRoles` case and `TestConcurrentSameNameWritesReturnConflict` the duplicate-key race. `TestRoleTemplateKeyBackfill`, `TestCopyBackfill`, `TestEnsureCopiesBeyondOneBatch` and `TestOrganizationCopiesWrittenByTheNextRelease` in `apps/anchor/cmd/it/service` cover the backfill, the copy batches and the template routes over copies.
