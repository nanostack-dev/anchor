# Cursor-paginated searches

Every persisted collection in the Anchor API is a `POST .../search` that pages with an opaque keyset cursor. The rule and the cursor contract are in [engineering best practices](../engineering-best-practices.md#collections-are-cursor-paginated-searches); this page records how Anchor applies them.

## Contract

- `PaginationRequest.cursor` continues a search after the last item of a previous page. `PagedListResponse.next_cursor` is absent on the last page. `total` counts every match of the filter.
- An invalid cursor, a cursor issued for another sort, and a cursor sent with a non-zero `offset` answer `400 INVALID_CURSOR`. Anchor's error middleware (`internal/middleware/error_middleware.go`) wraps the framework `StrictErrorHandler`, which maps the two `search` sentinels, so no handler maps them. A service that collapses repository errors into `fault.ErrUnexpected` passes them through first (`searchx.IsCursorError`).
- `sort_direction` is ascending when a `sort_by` is named without one. Without a `sort_by`, the operation's default order applies, including its direction. Every operation description states its default.
- Offset paging stays for page-number tables in the UI. A client that iterates uses the cursor.

## Searches

Each ordering ends with the tiebreak, and the tiebreak follows the direction of the last requested field. Every search below returns `next_cursor`.

| Operation | Default order | Tiebreak | Sort fields | Index behind the default order |
| --- | --- | --- | --- | --- |
| `searchProducts` | `created_at` ASC | `products.id` | name, created_at, updated_at | tenant-scoped, small |
| `searchProductResourcePermissions` | `created_at` ASC | `name` (composite key, product fixed) | created_at, updated_at, name | none: bounded per product |
| `searchProductPermissions` | `created_at` ASC | `name` (composite key, product fixed) | created_at, updated_at, name, id and resource (both order by name) | none: the built-in catalog is bounded per product |
| `searchPlatformInvitations` | `created_at` ASC | `id` | email, created_at, updated_at | tenant-scoped, small |
| `searchPlatformUsers` | `created_at` ASC | `id` | email, created_at, updated_at, role | tenant-scoped, small |
| `searchProductRoles` | `created_at` ASC | `id` | id, name, created_at, updated_at | product-scoped, small |
| `searchProductAPIKeys` | `created_at` ASC | `id` | id, name, created_at, updated_at, status, last_used_at (nullable) | `(product_id, created_at, id)` |
| `searchProductUsers` | `created_at` ASC | `id` | email, name (nullable), status, created_at, updated_at | `(product_id, created_at, id)` |
| `searchOrganizationAPIKeys` | `created_at` ASC | `id` | id, name, created_at, updated_at, status, last_used_at (nullable) | `(organization_id, created_at, id)` |
| `searchProductOrganizations` | `created_at` ASC | `id` | name, created_at, updated_at | `(product_id, created_at, id)` |
| `searchOrganizationWorkspaces` | `created_at` ASC | `id` | name, created_at, updated_at | `(organization_id, created_at, id)` |
| `searchOrganizationMembers` | `joined_at` DESC | `product_user_id` (composite key, organization fixed) | joined_at, email | `(organization_id, created_at, product_user_id)` |
| `searchOrganizationInvitations`, `searchProductOrganizationInvitations` | `created_at` ASC | `id` | created_at, email, expires_at | `(organization_id, created_at, id)`, `(product_id, created_at, id)` |
| `searchOrganizationLicenses` | `organization_name` ASC | `organizations.id` | organization_name, instantiated_at (nullable through the left join) | none for a name order: the scope index finds the product's organizations and the page sorts them |
| `searchIntegrationAuditLogs` | `created_at` DESC | `id` | created_at, action, severity | `(integration_instance_id, created_at, id)` |
| `searchUserOrganizations` | `joined_at` ASC | `organization_id` (composite key, user fixed) | joined_at, name | none: one user belongs to few organizations |
| `searchEmailTemplates` | `created_at` DESC | `id` | created_at, updated_at, name, slug | `(product_id, created_at, id)` |
| `searchEmailSends` | `created_at` DESC | `id` | created_at, status | `(product_id, created_at, id)` |
| `searchLicenseTemplates` | `name` ASC | `id` | name, status, created_at, updated_at | product-scoped, small |
| `searchOrganizationLicenseHistory` | `changed_at` DESC | `id` | changed_at | `(organization_id, changed_at DESC, id DESC)` |

The last six are new. They replace the `GET` lists below, which stay available and are marked `deprecated: true` with the search named in their description.

| Deprecated `GET` | Replaced by | What it keeps |
| --- | --- | --- |
| `listIntegrationAuditLogs` | `searchIntegrationAuditLogs` | the newest 100 entries |
| `listUserOrganizations` | `searchUserOrganizations` | every membership, ordered by join date |
| `listEmailTemplates` | `searchEmailTemplates` | `limit` and `offset`, and `count` equal to `total` |
| `listEmailSends` | `searchEmailSends` | `limit`, `offset`, `template_id` and `status` |
| `listLicenseTemplates` | `searchLicenseTemplates` | every template, ordered by name |
| `getOrganizationLicenseHistory` | `searchOrganizationLicenseHistory` | `limit` and `offset` |

Not searches, with the reason stated in their operation descriptions: `listIntegrationInstances` (one instance per provider, so bounded by the number of providers), `getOrganizationUsageSeries` (an aggregate time series computed per request, paged by `limit` and `offset` over buckets, never a `next_cursor`), `getProductEventsCatalog` (a static catalog compiled into the binary), and the arrays embedded in a single resource (`getEmailTemplateExamples`, `getLicenseSchema`, `getOrganizationLicenseDiff`).

## Rules the repositories follow

- A search uses `transactor.Page(...).Keyset(transactor.Keys(...))`, never `OrderBy` or `SortColumns`. Each sort field maps to a key whose constructor matches the column type and nullability in the migrations: `StringKey` for `VARCHAR`, `TimeKey` for `TIMESTAMPTZ`, and a `Nullable*` key for a nullable column or a column read through a left join. A text column holding enum-like values (`status`, `role`) is a `StringKey`.
- The row type scans one SQL row per item. A search that shows children (role permissions, API-key permissions, the permissions behind `include=role_permissions`) pages the parents flat and loads the children for the page in a second statement.
- Sort only on bounded columns. Every sortable column is `VARCHAR(n)` or a timestamp.
- `search.Request.WithSort(nil, nil)` injects `created_at` ASC. A resource whose default is not that sets its default in its API mapper with `searchx.Sort`.
- NULLs sort last when ascending and first when descending, the Postgres placement the seek predicate follows. This reaches three searches: `last_used_at` on API keys, `name` on product users, and `instantiated_at` on organization licenses. The organization license search once placed unlicensed organizations last in both directions with a `CASE` expression. A keyset cannot express that, so descending now lists them first.
- Internal loops that walked pages by offset follow `next_cursor`: the license adjustment backfill reads the history this way, and so does the permission sync at startup.

## Tests

- `cmd/it/ct/cursor_search_test.go` (`TestCursorSearches`) walks every search three rows at a time across rows that tie on the sort column, in the default order, in the opposite direction and in each interesting sort field. It asserts every row once in the order SQL gives, a last page without `next_cursor`, and `400 INVALID_CURSOR` for a malformed cursor, a cursor with an offset and a cursor issued for another sort. A new search adds one case there.
- `cmd/it/ct/new_searches_test.go` covers the filters and the scope of the six new searches. `cmd/it/ct/deprecated_lists_test.go` keeps the deprecated lists honest until they are removed.
- `cmd/it/repository/cursor_pagination_repository_test.go` walks a composite primary key, a nullable key and a plain table at the repository layer.
- `clients/go/anchorsdk/pagination_test.go` covers `Cursor` and `All` against a stub server.

## Clients

- The Go SDK's search builders have `Cursor(string)`, and `All(ctx)` follows `next_cursor` as an `iter.Seq2`. See the [SDK design notes](../../clients/go/anchorsdk/DESIGN.md).
- anchor-ui tables page by offset for their page numbers and pass the sort they show. The license history and the integration audit panel read their pages by cursor, and the lookups of every license template follow `next_cursor` to the last page.
