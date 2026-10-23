# Workspace members

An organization member can hold a role in one workspace of their organization, on top of their organization role ([vocabulary](../../CONTEXT.md)). In that workspace the member holds the permissions of both roles. Rows live in `workspace_memberships` and the code in `apps/anchor/internal/workspacemember`.

| Route | Does |
| --- | --- |
| `PUT .../workspaces/{workspace_id}/members/{product_user_id}` | `assignWorkspaceMemberRole`: gives the member a role, or replaces the one they hold |
| `GET` the same path | `getWorkspaceMember` |
| `DELETE` the same path | `removeWorkspaceMember`: the member keeps their organization role |
| `POST .../workspaces/{workspace_id}/members/search` | `searchWorkspaceMembers`, newest assignment first. Filters: `product_user_ids`, `role_ids`. See [cursor-paginated searches](cursor-pagination.md) |

The routes use the `organization_member:read|update|delete` scopes of the organization member routes.

## Rules

- The workspace must belong to the organization of the product in the path, else `404 WORKSPACE_NOT_FOUND`. Every read and write is scoped by product, organization and workspace.
- The role must belong to the product, else `400 ROLE_NOT_FOUND_IN_REQUEST`, as for an organization member role.
- The product user must be a member of the organization, else `409 WORKSPACE_MEMBER_NOT_IN_ORGANIZATION`.
- A get or a remove of a role the user does not hold answers `404 WORKSPACE_MEMBERSHIP_NOT_FOUND`.
- Assigning the role the member holds already writes nothing and emits nothing. `assigned_at` is the first assignment and survives a replacement.

## Concurrency

An assign runs in one transaction. It takes a key-share lock on the workspace, so a concurrent workspace delete cannot orphan the row, and a no-key-update lock on the member's organization membership row. Removing the member from the organization deletes that row first, so it waits for a running assign and then deletes the workspace roles that assign wrote; an assign that starts later finds no member and answers 409. Two assigns for one member serialize on the same lock, so one created event is emitted.

## Events

Each carries `{organization_id, workspace_id, product_user_id, role_id}`:

- `workspace.membership.created`: a first role in the workspace.
- `workspace.membership.updated`: the role changed. `role_id` is the new role.
- `workspace.membership.deleted`: a remove, or an organization member removal (one event per workspace role removed). `role_id` is the role that was held.

Deleting a workspace or a product user removes the rows by foreign key cascade and emits no workspace membership event. Deleting a role that a workspace role still names is refused as role in use.

On a workspace, [access evaluation](access-evaluation.md) grants a member the permissions of their organization role and of their workspace role together, and reports the second as `context.workspace_role`.
