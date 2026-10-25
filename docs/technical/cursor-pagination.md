# Cursor-paginated searches

Every persisted collection in the Anchor API is a `POST .../search` that pages with an opaque keyset cursor. The rule and the cursor contract are in [engineering best practices](../engineering-best-practices.md#collections-are-cursor-paginated-searches); this page records how Anchor applies them.

## Contract

- `PaginationRequest.cursor` continues a search after the last item of a previous page. `PagedListResponse.next_cursor` is absent on the last page. `total` counts every match of the filter.
- An invalid cursor, an oversized one (over 8192 bytes), a cursor issued for another sort, and a cursor sent with a non-zero `offset` answer `400 INVALID_CURSOR`. `searchx.ValidateRequest` checks the pagination first (`search.Pagination.Validate`), so the struct tag on the cursor length never answers a generic `VALIDATION_ERROR` in its place and the refusal holds in an empty scope too.
- A sort field a repository does not map fails with `search.ErrUnknownSort`, which answers `400 INVALID_SORT`. Over HTTP an unknown `sort_by` is refused earlier, by request validation (`400 VALIDATION_ERROR`, see below), so `INVALID_SORT` is the second line of defence for a field that validation allows but a repository forgot to map.
- Anchor's error middleware (`internal/middleware/error_middleware.go`) wraps the framework `StrictErrorHandler`, which maps the three `search` sentinels, so no handler maps them. A service that collapses repository errors into `fault.ErrUnexpected` passes them through first (`searchx.IsClientError`), as do all the search services in `internal/service`.
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
| `searchOrganizationRoles` | `created_at` ASC | `public_id` | created_at, key, name | organization-scoped, one row per template plus custom roles |
| `searchProductResourceTypes` | `created_at` ASC | `id` | created_at, updated_at, slug, name | `(product_id, created_at, id)`, built with the table in `000054` |
| `searchResources` | `created_at` ASC | `external_id`, then `id` (the product's identifier is unique only per type) | created_at, name | `(organization_id, created_at, external_id, id)`, built with the table in `000072` |
| `searchProductAPIKeys` | `created_at` ASC | `id` | id, name, created_at, updated_at, status, last_used_at (nullable) | `(product_id, created_at, id)` |
| `searchProductUsers` | `created_at` ASC | `id` | email, name (nullable), status, created_at, updated_at | `(product_id, created_at, id)` |
| `searchOrganizationAPIKeys` | `created_at` ASC | `id` | id, name, created_at, updated_at, status, last_used_at (nullable) | `(organization_id, created_at, id)` |
| `searchProductOrganizations` | `created_at` ASC | `id` | name, created_at, updated_at | `(product_id, created_at, id)` |
| `searchOrganizationWorkspaces` | `created_at` ASC | `id` | name, created_at, updated_at | `(organization_id, created_at, id)` |
| `searchOrganizationMembers` | `joined_at` DESC | `product_user_id` (composite key, organization fixed) | joined_at, email | `(organization_id, created_at, product_user_id)` |
| `searchOrganizationInvitations`, `searchProductOrganizationInvitations` | `created_at` ASC | `id` | created_at, email, expires_at | `(organization_id, created_at, id)`, `(product_id, created_at, id)` |
| `searchOrganizationLicenses` | `organization_name` ASC | `organizations.id` | organization_name, instantiated_at (nullable through the left join, NULLs last in both directions) | none for a name order: the scope index finds the product's organizations and the page sorts them |
| `searchIntegrationAuditLogs` | `created_at` DESC | `id` | created_at, action, severity | `(integration_instance_id, created_at, id)` |
| `searchUserOrganizations` | `joined_at` ASC | `organization_id` (composite key, user fixed) | joined_at, name | none: one user belongs to few organizations |
| `searchEmailTemplates` | `created_at` DESC | `id` | created_at, updated_at, name, slug | `(product_id, created_at, id)` |
| `searchEmailSends` | `created_at` DESC | `id` | created_at, status | `(product_id, created_at, id)` |
| `searchLicenseTemplates` | `name` ASC | `id` | name, status, created_at, updated_at (full text matches name and description) | product-scoped, small |
| `searchOrganizationLicenseHistory` | `changed_at` DESC | `id` | changed_at | `(organization_id, changed_at DESC, id DESC)` |

The last six are new. They replace the `GET` lists below, which stay available and are marked `deprecated: true` with the search named in their description.

| Deprecated `GET` | Replaced by | What it keeps |
| --- | --- | --- |
| `listIntegrationAuditLogs` | `searchIntegrationAuditLogs` | the newest 100 entries |
| `listUserOrganizations` | `searchUserOrganizations` | every membership, ordered by join date |
| `listEmailTemplates` | `searchEmailTemplates` | any `limit` (a page over 1000 is assembled from consecutive searches), `offset`, and `count` equal to `total` |
| `listEmailSends` | `searchEmailSends` | any `limit`, `offset`, `template_id` and `status` |
| `listLicenseTemplates` | `searchLicenseTemplates` | every template, ordered by name |
| `getOrganizationLicenseHistory` | `searchOrganizationLicenseHistory` | `limit` and `offset` |

Not searches, with the reason stated in their operation descriptions: `listIntegrationInstances` (one instance per provider, so bounded by the number of providers), `getOrganizationUsageSeries` (an aggregate time series computed per request, paged by `limit` and `offset` over buckets, never a `next_cursor`), `getProductEventsCatalog` (a static catalog compiled into the binary), and the arrays embedded in a single resource (`getEmailTemplateExamples`, `getLicenseSchema`, `getOrganizationLicenseDiff`).

## Rules the repositories follow

- A search builds its ordering with `transactor.StrictKeys(...)`, returns its error, and hands the result to `transactor.Page(...).Keyset(order)`, never `OrderBy` or `SortColumns`. `StrictKeys` refuses a requested sort field the key map lacks (`search.ErrUnknownSort`) instead of dropping it as `transactor.Keys` does; `Keys` stays for orderings that are not searches. Each sort field maps to a key whose constructor matches the column type and nullability in the migrations: `StringKey` for `VARCHAR`, `TimeKey` for `TIMESTAMPTZ`, and a `Nullable*` key for a nullable column or a column read through a left join. A text column holding enum-like values (`status`, `role`) is a `StringKey`.
- The row type scans one SQL row per item. A search that shows children (role permissions, API-key permissions, the permissions behind `include=role_permissions`) pages the parents flat and loads the children for the page in a second statement.
- The body of a search is not checked against the OpenAPI enums, so each search input has a `Validate()` that the service calls first: struct tags (`oneof`, `dive`) refuse a filter value outside its enum, and `searchx.ValidateRequest` refuses a `sort_by` the resource does not map. Both answer 400 `VALIDATION_ERROR`. A product user status filter accepts the contract's lowercase values and the uppercase ones the Clerk ingest and the create default store; a platform role compares without regard to case. A new sort field is added to the repository's key map and to that list.
- Sort only on bounded columns. Every sortable column is `VARCHAR(n)` or a timestamp.
- `search.Request.WithSort(nil, nil)` injects `created_at` ASC. A resource whose default is not that sets its default in its API mapper with `searchx.Sort`, so the default always names a mapped field: `StrictKeys` would refuse an injected `created_at` that the key map lacks.
- A nullable key sorts NULLs last when ascending and first when descending, the Postgres placement, unless the key pins it with `NullsLast()` or `NullsFirst()`. `last_used_at` on API keys and `name` on product users keep the default. `instantiated_at` on organization licenses is `NullsLast()`, so organizations holding no license come after every licensed one in both directions, and the cursor follows that placement.
- Internal loops that walked pages by offset follow `next_cursor`: the license adjustment backfill reads the history this way, and so does the permission sync at startup.

## Index migrations

Migrations 000043 to 000053 add the indexes in the table above, one file per index. Each file holds exactly one `CREATE INDEX CONCURRENTLY IF NOT EXISTS` statement and its down file one `DROP INDEX CONCURRENTLY IF EXISTS`. golang-migrate's postgres driver sends a file as a single query on its own connection and opens no transaction, so a lone statement builds without blocking writes, while a file with several statements runs in an implicit transaction where `CONCURRENTLY` is refused. Keep it one statement per file. A concurrent build that fails leaves an invalid index and the migration dirty: drop the index by name, then rerun. `TestCursorSearchIndexMigrationsAreSingleConcurrentStatements` and `TestCursorSearchIndexesAreValid` guard the shape and the result.

## Tests

- `cmd/it/ct/cursor_search_test.go` (`TestCursorSearches`) runs the framework's `searchtest` conformance kit (`pkg/testkit/searchtest`, see its README) against every search, one suite each, three rows per page across rows that tie on the sort column. A suite walks the default order, the default field without a direction and every sort field in both directions; the expected ids come from SQL over the fixture, never from the search. Its refusal scopes are the default order, one populated scope per sort field (so the forged cursors reach every key type), and two empty scopes (a filter that matches nothing, with and without a sort) that stand for the short-circuit a search may take. In every scope the kit sends a malformed, an oversized, a forged and a foreign cursor, a cursor with an offset and an unknown `sort_by`, and expects 400 (`INVALID_CURSOR`; `VALIDATION_ERROR` for the sort, which request validation refuses first). `searchtest.FrameworkCursorMutations` forges the cursors, because every cursor is issued by this service. The case also asserts what the kit does not: a page that fills the limit exactly has no `next_cursor`, offset pages keep the order, and a cursor sent without a limit takes the default page size. A new search adds one case there with its fixture, every sort field and a no-match filter; do not write a second walker.
- `internal/searchx/repositories_test.go` builds every search repository with no database and asks it for a sort it does not map, which must fail with `search.ErrUnknownSort`. `internal/service/search_errors_test.go` stubs the repository of each service that used to collapse errors and checks that all three sentinels come out unchanged. Validation makes both unreachable over HTTP, so the unit tests pin the layers behind it.
- `cmd/it/ct/new_searches_test.go` covers the filters and the scope of the six new searches. `cmd/it/ct/deprecated_lists_test.go` keeps the deprecated lists honest until they are removed.
- `cmd/it/repository/cursor_pagination_repository_test.go` walks a composite primary key, a nullable key and a plain table at the repository layer.
- `clients/go/anchorsdk/pagination_test.go` covers `Cursor` and `All` against a stub server.

## Clients

- The Go SDK's search builders have `Cursor(string)`, and `All(ctx)` follows `next_cursor` as an `iter.Seq2`. See the [SDK design notes](../../clients/go/anchorsdk/DESIGN.md).
- anchor-ui tables page by offset for their page numbers and pass the sort they show. The license history and the integration audit panel read their pages by cursor, and the lookups of every license template follow `next_cursor` to the last page.
