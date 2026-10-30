# Organization Workspaces

> [!WARNING]
> **Deprecated** ([ADR-0022](adr/0022-resource-types-scope-roles-and-permissions.md) A6). Workspaces are now resources of the Product's `workspace` [resource type](technical/resource-types.md#the-workspace-type). The routes below keep answering exactly as before until no Product calls them; then they are removed with the `workspaces` table, the `workspace.*` events and `workspace_memberships` (A6 phase 4, milestone M7). They get no new features.

Anchor supports organization-scoped workspaces as the third level of the Product -> Organization -> Workspace hierarchy. Workspaces are managed through product and organization scoped API paths so every operation carries the owning product and organization context.

## What replaces them

| Today | Use instead |
| --- | --- |
| `POST …/organizations/{organization_id}/workspaces` | `PUT …/organizations/{organization_id}/resources/workspace/{id}` with the Product's own ID, a `name` and `"parent": { "type": "organization", "id": "<organization_id>" }` |
| `GET` and `PUT …/workspaces/{workspace_id}` | `GET` and `PUT …/resources/workspace/{workspace_id}` |
| `DELETE …/workspaces/{workspace_id}` | Keep this route while the workspace routes exist. `DELETE …/resources/workspace/{workspace_id}` refuses a workspace's resource with `409 RESOURCE_IS_WORKSPACE`, because only this route deletes the workspace too. |
| `POST …/workspaces/search` | `POST …/resources/search` with `"filter": { "resource_types": ["workspace"] }` |
| A role on a workspace | `PUT …/resources/workspace/{workspace_id}/members/{product_user_id}` with a role scoped to `workspace` ([role assignments](technical/resources.md#role-assignments)) |
| `workspace.*` events | `resource.role_assignment.*` for roles; resource events stay inside Anchor |

Every existing workspace is already a resource with the **same ID** and its name, under its Organization: the startup migration registered it, and every write through these routes keeps it in step ([resources](technical/resources.md#workspaces)). A Product can therefore switch a workspace to the resource routes without moving data. A workspace's description stays on the workspace: resources carry a name only.

Every response of these routes carries two headers:

```http
Deprecation: @1792886400
Link: </v1/products/prod_123/organizations/org_123/resources/workspace/ws_2xWk...>; rel="successor-version"
```

`Deprecation` is the date of the deprecation, 2026-10-25, as [RFC 9745](https://www.rfc-editor.org/rfc/rfc9745) defines it. `Link` names the replacing route: the workspace's resource for one workspace, the resource search for the collection and its search. Errors, including 401 and 404, carry them too. The OpenAPI contract marks the five operations `deprecated: true`, so the Go SDK and the UI client flag their callers.

An organization has zero or more workspaces. They are ordinary peers: none has a
default designation, and users can rename or delete every workspace.

## Organization creation and related reads

Organization creation accepts an optional `workspace` payload using the ordinary
workspace fields. For example, `{"name":"Acme","workspace":{"name":"Engineering"}}`
creates both in one transaction, including their events and any requested license
and founding membership. Omitting `workspace` creates no workspaces.

The Go SDK exposes `Organizations().Create("Acme").Workspace("Engineering")`.
A founding-member retry returns the existing organization without creating or
recreating workspaces, even if its initial workspace was renamed or deleted.

Organization Get and Search accept `include=workspace,license`. The workspace
include returns all linked workspaces for the caller's product, including an empty
`workspaces: []` when none exist. Without the include the field is omitted. Each
included resource is read once per organization page.

Consumers decide their own onboarding and migration policy. Anchor does not sweep
organizations at startup and does not assign resources to a preferred workspace.

## API Behavior

- Create: `POST /v1/products/{product_id}/organizations/{organization_id}/workspaces`
- Search: `POST /v1/products/{product_id}/organizations/{organization_id}/workspaces/search`
- Get: `GET /v1/products/{product_id}/organizations/{organization_id}/workspaces/{workspace_id}`
- Update: `PUT /v1/products/{product_id}/organizations/{organization_id}/workspaces/{workspace_id}`
- Delete: `DELETE /v1/products/{product_id}/organizations/{organization_id}/workspaces/{workspace_id}`

Workspace names are unique within an organization. The same workspace name can be reused in a different organization, including another organization under the same product.

## Examples

Create a workspace:

```http
POST /v1/products/prod_123/organizations/org_123/workspaces
Content-Type: application/json

{
  "name": "Engineering",
  "description": "Workspace for engineering teams"
}
```

Successful create response:

```json
{
  "id": "ws_2xWk...",
  "organization_id": "org_123",
  "name": "Engineering",
  "description": "Workspace for engineering teams",
  "created_at": "2026-05-09T12:00:00Z",
  "updated_at": "2026-05-09T12:00:00Z"
}
```

Search workspaces in an organization:

```http
POST /v1/products/prod_123/organizations/org_123/workspaces/search
Content-Type: application/json

{
  "pagination": {
    "limit": 25,
    "offset": 0
  },
  "sort_by": "created_at",
  "sort_direction": "desc",
  "full_text_search": "engineer"
}
```

Successful search response:

```json
{
  "items": [
    {
      "id": "ws_2xWk...",
      "organization_id": "org_123",
      "name": "Engineering",
      "description": "Workspace for engineering teams",
      "created_at": "2026-05-09T12:00:00Z",
      "updated_at": "2026-05-09T12:00:00Z"
    }
  ],
  "count": 1,
  "total": 1
}
```

The response carries a `next_cursor` while more workspaces follow. Send it back as `pagination.cursor`, with the same filter and sort, to read the next page, and stop when it is absent. `offset` serves page-number tables only and cannot be combined with a cursor. Without a `sort_by`, workspaces come oldest first, then by `id`. See [cursor-paginated searches](technical/cursor-pagination.md).

## Authorization

Product API key callers need the matching workspace scope:

- `workspace:create` for create
- `workspace:read` for get and search
- `workspace:update` for update
- `workspace:delete` for delete

Platform bearer callers can read workspaces through get and search endpoints. They cannot create, update, or delete workspaces.

## Isolation

Repository reads, searches, updates, and deletes scope workspace rows through the owning organization and product. A workspace requested through the wrong organization or product path is treated as not found.

## Access evaluation

`"type": "workspace"` is decided through the workspace's resource: the Organization role reaches it through its parent, and roles assigned on it grant too. An Organization-scoped permission is still decided on the Organization, and an unknown or foreign workspace is still `WORKSPACE_NOT_IN_ORGANIZATION` ([access evaluation](technical/access-evaluation.md#workspaces)).

## Platform Admin UI

The platform admin frontend exposes a read-only Workspaces page. Admins select an organization, then view workspace rows in a datatable with search, filters, sorting, and pagination. The page intentionally does not expose create, edit, delete, or bulk mutation controls.
