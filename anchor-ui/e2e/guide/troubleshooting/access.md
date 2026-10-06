# Access and tenancy browser tests

The scenarios in `features/access` own one disposable product per test. Tests
create unrelated prerequisites through the authenticated public API, then drive
the shipped browser controls. Role and permission mutations are asserted in
both the UI and the persisted API resource.

## Implemented behavior and fixture boundaries

- `access.catalog` covers the built-in Anchor permission catalog, its search,
  facets, pagination and recovery from no matches. These permissions are
  software-managed; no create/edit/delete controls should appear.
- `access.permission-*` covers resource permission creation, view/edit/cancel,
  persisted description, protected name, search/facets/sort/pagination,
  single/bulk deletion and the affected role warning. Fixture API calls create
  a role when deletion impact is the behavior under test.
- `access.role-*` covers the role wizard, resource permission selection,
  view/edit/cancel, persisted grant replacement, list controls and single/bulk
  deletion. An assigned role must retain its row after a `409` deletion error;
  bulk deletion must report the failed row while deleting an unrelated role.
- Product users and organizations are read-only browser lists. Their API
  creation is preparation, not browser coverage of a creation flow.
- Organization members, organization API keys and workspaces are read-only
  organization-scoped views. Their fixture APIs create the selected
  organization and related data. Organization invitations expose single/bulk
  deletion; invitation creation/acceptance remain API features.
- `/workspace-memberships` is an implemented placeholder heading. The test
  records that boundary instead of inventing membership controls.

## Worker login fails with an invalid URL

Symptom: every scenario fails before its first step at `page.goto('/login')`
inside the worker fixture, reporting `Cannot navigate to invalid URL`.

Cause: `browser.newContext()` creates a context directly and does not inherit
the configured `use.baseURL`. The ordinary test `page` fixture does inherit it.

Repair: pass the configured app origin when creating the worker login context,
or give the shared login helper an absolute URL. Keep one configured app origin
so worker setup and test pages cannot point to different applications.

Verification: run the access folder with the managed app config. The scenarios
must reach their own browser actions; retries cannot repair an invalid URL.

```sh
E2E_REUSE_SERVER=1 pnpm exec playwright test --config playwright.app.config.ts \
  --project chromium --no-deps e2e/features/access \
  --reporter=list --output=test-results/access
```

The reuse flag is for an already healthy, locally owned app runtime. A normal
fresh suite run owns runtime startup and teardown through `package.json`.

## Tenant fixture writes return 401 with a platform login token

Symptom: organization, product user, organization membership, workspace or
organization API key preparation returns `401` despite a successful UI login.

Cause: these write operations authenticate a product management key in the
public OpenAPI contract. A platform JWT can read many admin views, but it does
not authorize those tenant writes.

Repair: use `await world.productAPI()` for tenant preparation. The shared fixture
creates a unique local management key with supported catalog scopes and
disposes its request context during teardown. Use `world.api` for platform and
product catalog operations. Follow each operation's contract instead of
reusing one credential everywhere; never borrow a saved development key.

Verification: `access.product-users`, `access.organizations`,
`access.organization-members`, `access.organization-invitations`,
`access.organization-keys` and `access.workspaces` must prepare real resources
and then reach their browser assertions.

## A deleted row vanishes while its resource still exists

Symptom: immediately after confirming permission or role deletion, the row
count is zero but a direct API GET returns the previous `200` resource.

Cause: a fetching table can temporarily replace rows with a loading state.
Absence of the row alone does not prove that the DELETE has committed. In the
permission and role lifecycle scenarios, the initial direct GET raced the
pending DELETE.

Repair: register `page.waitForResponse` before the confirmation click, matching
the exact resource URL and `DELETE` method. Assert its `204`, then assert row
removal and the scoped GET `404`. Use `expect.poll` for eventual persisted
state. The bulk helper waits for the explicit completed success/failure summary.

Verification: `access.permission-lifecycle` and `access.role-lifecycle` passed
after this repair without a timeout increase or fixed sleep.

## Wizard transitions produce two Continue buttons

Symptom: a role wizard locator for `Continue` fails strict mode during the step
animation because both outgoing and incoming panels are mounted.

Repair: scope interactions to named `tabpanel` roles: `1. Basic Info`,
`2. Permissions` and `3. Review`. Scope grant text to the permissions panel;
the selected summary can repeat the permission name alongside its selectable
row. Keep review assertions in the review panel.

Verification: `access.role-lifecycle` creates a role, replaces its grants,
reloads the persisted detail and deletes it through the shipped UI.

## Sorting and facets have the same accessible label

The list toolbar facet and the Name column sort button both have the name
`Name`. `helpers.facet` chooses the first exact role/name match because the
toolbar precedes the table. Sorting actions instead scope to the named
`columnheader`. If the table layout changes, update that domain helper and
verify both operations rather than adding CSS selectors.

Read-only assertions must name mutation actions accurately: an unanchored
`/Create/` also matches the sorting button `Created At`. Anchor action prefixes
or use complete names so a valid sorting control does not cause a false failure.

## Search inputs change while server results stay unchanged

Reproduced failures:

- `access.role-bulk` searches `Delete` but the unrelated role remains.
  `product_role_repository.go` received `FullTextSearch` but never applied it
  to its product-scoped SQL predicate.
- `access.permission-list` searches one unique permission but retains the
  original `12 total`. `product_resource_permission_repository.go` likewise
  ignored `FullTextSearch`. Fixing that predicate alone was insufficient:
  `apps/anchor/internal/api/product_resource_permission_api.go`, in
  `mapToSearchProductResourcePermissionInput`, also dropped the public
  `full_text_search` field. Its request builder must forward
  `WithFullTextSearch(req.FullTextSearch)` before the repository can apply it.
- `access.organization-members` searches one member but another remains. The
  browser sent legacy `limit`, `offset` and `query` fields instead of the
  generated contract's `pagination` and `full_text_search`. Two members cannot
  catch the related pagination bug, so this scenario owns twelve members and
  checks the second page's two rows before searching by email and name.

Repair these at their contract boundary. Keep the product/org scope predicate
when adding repository full-text matching, forward the value through the API
request mapper, and send the generated request shape from the browser. Follow
the field through browser request, API mapper and repository; a correct layer
cannot repair a value dropped earlier. Rebuild the real local backend/frontend
before reruns; a source edit does not change an already built runtime.

Verification: run `access.role-list`, `access.role-bulk`,
`access.permission-list` and `access.organization-members` against the rebuilt
runtime. A visible input value alone is never a search assertion: verify
matching rows, excluded rows or the result total.

## Deleting the last role grant returns null permissions

Symptom: `access.permission-impact` sees the role's last grant removed but its
API response contains `permissions: null`; the public response contract
requires an array.

Repair: the role response mapper must serialize an empty collection as `[]`.
Keep the assertion exact rather than coercing `null` to an empty array in the
test; clients rely on the declared shape.

Verification: `access.permission-impact` deletes the granted permission in the
browser, checks the affected-role warning and polls the persisted role until
its permissions equal `[]`.
