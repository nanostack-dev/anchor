# Organization roles

Roles belong to Organizations ([ADR-0022](../adr/0022-resource-types-scope-roles-and-permissions.md) D8). A Product defines **role templates** under `/v1/products/{product_id}/roles`, and every Organization holds its own **copy** of each template. Members and invitations hold the copy. Access decisions read the copy's permissions, which equal the template's, so every check answers as before.

This change ships in two releases. A rolling deploy runs the previous release beside the next one, so the release that first writes copies must run beside a release that already reads them:

1. **This release** adds the columns, keys the templates, and reads and writes every copy correctly. It makes no copy.
2. **The next release** makes the copies and moves memberships onto them.

## Storage and IDs

Templates and Organization roles share `product_roles`. A template has no `organization_id`; a copy has one and points back with `template_id`. Sharing the table keeps every foreign key that names a role (`organization_memberships`, `organization_invitations`, `workspace_memberships`) valid during a rolling deploy.

A copy is addressed by its **template's ID** inside its Organization: `public_id`, a virtual column, is `COALESCE(template_id, id)`. Responses (memberships, the access decision context) show it, and requests may send it, so the role IDs released clients hold keep working.

| Rows the next release writes | How this release handles them |
| --- | --- |
| A copy | Template routes, searches and resource type usage leave it out (`organization_id IS NULL`). |
| A membership on a copy | Its role shows the template's ID. A member search by the template's ID finds it. |
| A template ID named in a request | It resolves to the Organization's copy when there is one, or else to the template. |
| A change to a template | A rename, a description change, and every permission assigned, unassigned or replaced reach its copies in the same transaction (`SyncCopies`). |
| A delete of a template | A template whose copy a member holds, or that a pending invitation names, answers `409 ROLE_IN_USE` as before. Otherwise its copies are deleted with it. |

Every template write locks the template's row and its copies' rows first, in ID order (`LockTemplate`), after the catalog lock when it needs one. A copy made by the next release reads the template under a share lock, so it sees the permissions from before a change or after it, never a mix. A membership written on a copy waits for a delete of the template, which counts the members of the template and its copies inside the same transaction. `SyncCopies` rewrites only the copies that differ.

## Keys

A template has a `key`, unique in its Product regardless of case (`409 ROLE_KEY_DUPLICATE`). Omitted on create, it is the name in lower case. It never changes, and every copy keeps it. A create whose name another template holds answers `409 ROLE_NAME_DUPLICATE` as before, even when its key clashes too.

On every start, under the `organization_roles:key-backfill` lock, `BackfillKeys` gives a key to every template the previous release wrote without one, one transaction per Product. The key is the name in lower case, or the template's ID when another template already holds that key.

## Rollback

Migrations `000061` and `000062` replace the product-wide name guard with a guard on template names. A copy shares its template's name, so their down migrations succeed only after memberships move back to the templates and the copies are deleted.

## Tests

`TestRoleTemplateKeyBackfill` and `TestOrganizationCopiesWrittenByTheNextRelease` in `apps/anchor/cmd/it/service` write the rows the next release writes and check each row in the table above. `TestConcurrentSameNameWritesReturnConflict` in `apps/anchor/cmd/it/ct` has the duplicate-key race.
