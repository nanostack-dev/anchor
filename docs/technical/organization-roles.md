# Organization roles

Roles belong to Organizations ([ADR-0022](../adr/0022-resource-types-scope-roles-and-permissions.md) D8). A Product defines **role templates** under `/v1/products/{product_id}/roles`, and every Organization holds its own **copy** of each template. Members and invitations hold the copy. Access decisions read the copy's permissions, which equal the template's, so every check answers as before.

This change shipped in two releases, because a rolling deploy runs the previous release beside the next one. The first release added the columns and read and wrote copies correctly without making any. The second makes the copies and moves memberships onto them.

## Storage and IDs

Templates and Organization roles share `product_roles`. A template has no `organization_id`; a copy has one and points back with `template_id`. Sharing the table keeps every foreign key that names a role (`organization_memberships`, `organization_invitations`, `workspace_memberships`) valid during a rolling deploy.

A copy is addressed by its **template's ID** inside its Organization: `public_id`, a virtual column, is `COALESCE(template_id, id)`. Responses (memberships, invitations, the access decision context, Organization roles) show it, and requests may send it, so the role IDs released clients hold keep working. Memberships store the copy's own row ID. Invitations store the public ID and resolve it again on accept. Member searches filter `role_ids` on the public ID. A role ID of another Organization names no role: `404 ROLE_NOT_FOUND` on a path, `400 ROLE_NOT_FOUND_IN_REQUEST` in a body.

Template routes, searches and resource type usage leave copies out (`organization_id IS NULL`).

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

A template change reaches its copies in the same transaction: a rename or a description change, and every permission assigned, unassigned or replaced (`SyncCopies`, which rewrites only the copies that differ). The copy keeps the template's key. Deleting a template no member holds deletes its copies; a template whose copy a member holds, or that a pending invitation names, answers `409 ROLE_IN_USE` as before.

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

Routes under `/organizations/{organization_id}/roles` take the `organization_role:read` and `organization_role:update` API key scopes. The list-shaped `role_ids`, `role_keys` and `roles` contract arrives with role assignments (ADR-0022 D3, PR 12).

## Rollback

Migrations `000061` and `000062` replace the product-wide name guard with a guard on template names. A copy shares its template's name, so their down migrations succeed only after memberships move back to the templates and the copies are deleted.

## Tests

`organization_role_test.go` in `apps/anchor/cmd/it/ct` covers the copies, propagation of grants and revokes to every Organization through the access evaluation, the race between a copy and a template change, a member added while its template is deleted, keys, both defaults with every switch state, their races and a default moved while an Organization copies, invitations, founding members, another Organization's role IDs, the search filter limits and deletion. `TestCursorSearches` has the `searchOrganizationRoles` case and `TestConcurrentSameNameWritesReturnConflict` the duplicate-key race. `TestRoleTemplateKeyBackfill`, `TestCopyBackfill`, `TestEnsureCopiesBeyondOneBatch` and `TestOrganizationCopiesWrittenByTheNextRelease` in `apps/anchor/cmd/it/service` cover the backfill, the copy batches and the template routes over copies.
