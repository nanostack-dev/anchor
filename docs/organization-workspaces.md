# Organization Workspaces

Anchor supports organization-scoped workspaces as the third level of the Product -> Organization -> Workspace hierarchy. Workspaces are managed through product and organization scoped API paths so every operation carries the owning product and organization context.

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

## Authorization

Product API key callers need the matching workspace scope:

- `workspace:create` for create
- `workspace:read` for get and search
- `workspace:update` for update
- `workspace:delete` for delete

Platform bearer callers can read workspaces through get and search endpoints. They cannot create, update, or delete workspaces.

## Isolation

Repository reads, searches, updates, and deletes scope workspace rows through the owning organization and product. A workspace requested through the wrong organization or product path is treated as not found.

## Platform Admin UI

The platform admin frontend exposes a read-only Workspaces page. Admins select an organization, then view workspace rows in a datatable with search, filters, sorting, and pagination. The page intentionally does not expose create, edit, delete, or bulk mutation controls.
