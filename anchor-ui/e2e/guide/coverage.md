# Anchor browser coverage inventory

The managed app suite covers the keys in `src/routes/routePaths.ts`, plus the
organization-license child pages registered separately. It drives the real local
Anchor browser and API in desktop Chromium. This is a behavior inventory, not a claim of 100% code,
branch, device, browser or API coverage.

<!-- e2e-coverage:start -->
Generated from `playwright.app.config.ts` discovery. Refresh with
`pnpm update:e2e:coverage`; CI checks this block with `pnpm check:e2e:coverage`.

The managed app suite declares **63 scenarios in 15 spec files**.

| Spec | Scenarios |
| --- | ---: |
| [access/permissions.e2e.ts](../features/access/permissions.e2e.ts) | 5 |
| [access/roles.e2e.ts](../features/access/roles.e2e.ts) | 5 |
| [access/tenancy.e2e.ts](../features/access/tenancy.e2e.ts) | 6 |
| [access/workflows.e2e.ts](../features/access/workflows.e2e.ts) | 6 |
| [auth/bootstrap.e2e.ts](../features/auth/bootstrap.e2e.ts) | 1 |
| [auth/session.e2e.ts](../features/auth/session.e2e.ts) | 4 |
| [integrations/email.e2e.ts](../features/integrations/email.e2e.ts) | 5 |
| [integrations/integrations.e2e.ts](../features/integrations/integrations.e2e.ts) | 4 |
| [licensing/migrations.e2e.ts](../features/licensing/migrations.e2e.ts) | 3 |
| [licensing/organizations.e2e.ts](../features/licensing/organizations.e2e.ts) | 6 |
| [licensing/schema.e2e.ts](../features/licensing/schema.e2e.ts) | 4 |
| [licensing/templates.e2e.ts](../features/licensing/templates.e2e.ts) | 4 |
| [platform/administration.e2e.ts](../features/platform/administration.e2e.ts) | 3 |
| [platform/api-keys.e2e.ts](../features/platform/api-keys.e2e.ts) | 4 |
| [platform/products.e2e.ts](../features/platform/products.e2e.ts) | 3 |
<!-- e2e-coverage:end -->

Count refresh and checks use Playwright's full JSON discovery inventory without
starting Docker, the app or a browser. The generated block counts declared
scenarios; it does not establish that they passed. Maintain the behavior and
boundary descriptions below when changing a scenario. Historical run counts and
timings stay in [performance](performance.md).

The tables describe assertions present in the scenarios. A route annotation
detects inventory drift; it does not prove that its controls were exercised or
that a run passed. Before reporting verified coverage, run `pnpm test:e2e:verify`
and inspect `test-results/app/results.json` for every scenario. A stale-runtime
failure, failed assertion, skipped scenario or bootstrap reused against an
initialized database does not establish fresh-install coverage. See
[performance](performance.md) for run evidence and [troubleshooting](troubleshooting.md)
for reproduced defects and repairs.

Each ordinary scenario owns a disposable product. Worker platform admins share
only their authenticated login; browser contexts and product data are isolated.
API calls prepare unrelated data and check persistence. They do not count as a
browser action. The tenant write helper creates a local product management key;
it never uses saved development credentials.

## Session, platform and product administration

Specs: [bootstrap](../features/auth/bootstrap.e2e.ts),
[session](../features/auth/session.e2e.ts),
[administration](../features/platform/administration.e2e.ts),
[products](../features/platform/products.e2e.ts).

| Route key and path | Browser actions asserted | Preparation or boundary | Spec |
| --- | --- | --- | --- |
| `INIT` — `/init` | First owner/tenant signup, launch, authenticated app and initialized health. | Requires the fresh disposable database; warm runs log in instead. | bootstrap |
| `LOGIN` — `/login` | Guest route/search preservation, required/email validation, real rejected credentials, internal redirect, reload, cookie refresh, logout and external redirect containment. | API registers an isolated invited admin for the session journey. | session |
| `REGISTER` — `/register` | Invitation email locked, mismatched passwords rejected, signup, automatic login and reload persistence. | Invitation is created through the platform UI; guest context follows its copied link. | administration |
| `INDEX` — `/` | Selected-product summary and Products navigation. | API product, selected through real header controls. Other dashboard links are not individually clicked. | products, session |
| `PLATFORM_USERS` — `/platform/users` | Search, email sort, pagination, owner delete disabled, single and selected bulk admin deletion. | API invitations/registration create disposable admins. No user create/edit UI is claimed. | administration |
| `PLATFORM_INVITATIONS` — `/platform/users/invitations` | Email validation, create, copy signup link, search, email sort, pagination and single/bulk deletion. | Twelve API invitations prepare list behavior; browser-created invitation drives signup. | administration |
| `PRODUCTS` — `/products` | Create validation/cancel/reset, search, Name facet/clear recovery, name sorting, pagination, selection/reload and single/bulk deletion that leaves outsiders. | API products prepare the list; the lifecycle creates its product through the UI. | products |
| `PRODUCT_EDIT` — `/products/$productId/edit` | Name/description, Config tab, organization-key prefix, protection, reload and unprotect before deletion. | API checks persisted state; protected delete control must remain disabled. | products |
| `SETTINGS_USER` — `/settings/user` | Heading and “Settings are coming soon”. | Implemented placeholder; no settings save controls exist. | products |
| `SETTINGS_APP` — `/settings/app` | Heading and “Settings are coming soon”. | Implemented placeholder; no settings save controls exist. | products |

## Management keys, permissions and roles

Specs: [management keys](../features/platform/api-keys.e2e.ts),
[permissions](../features/access/permissions.e2e.ts),
[roles](../features/access/roles.e2e.ts),
[tenancy](../features/access/tenancy.e2e.ts).

| Route key and path | Browser actions asserted | Preparation or boundary | Spec |
| --- | --- | --- | --- |
| `PRODUCT_API_KEYS` — `/products/product-api-keys` | Search, Name/Status facets and clear recovery, name sort, pagination, edit navigation, single/bulk deletion, persisted empty list and absence of clear key after reload. | Immutable keys are API prerequisites; mutable key is created through the UI. | management keys |
| `PRODUCT_API_KEY_NEW` — `/products/product-api-keys/new` | Required name, mutable switch, permission search/select-all-visible/Selected filter, review/back, create, reveal and copy clear key once. | Built-in management permissions come from the real catalog. | management keys |
| `PRODUCT_API_KEY_EDIT` — `/products/product-api-keys/$apiKeyId/edit` | Mutable name/grant update, immutable metadata update with grants locked, missing-key recovery. | API persistence assertions and an API-deleted missing fixture. | management keys |
| `PRODUCT_USERS` — `/products/users` | Email search, Status facet, pagination, empty recovery and no user mutation buttons. Clerk ingestion also appears here. | Product users are API fixtures or signed Clerk ingestion. Read-only admin list. | tenancy, integrations |
| `PRODUCT_PERMISSIONS` — `/products/permissions` | Built-in catalog search, Name facet, pagination, clear empty search and no create/delete controls. | Software-managed Anchor permission catalog, distinct from resource permissions. | permissions |
| `PRODUCT_RESOURCES_PERMISSIONS` — `/products/resources/permissions` | Create, Name facet, sort, pagination, search/empty recovery, single/bulk delete and affected-role warning. | API permissions prepare list/bulk data; a role holds a grant for deletion impact. | permissions |
| `PRODUCT_RESOURCE_PERMISSION_DETAIL` — `/products/resources/permissions/$permissionName` | View, immutable name, description edit/cancel/save/reload and missing-resource recovery. | API checks persisted description and committed deletion. | permissions, roles |
| `PRODUCT_ROLES` — `/products/resources/roles` | Create wizard, permission selection, Name facet, sorting, pagination, search, single/bulk delete, assigned-role error and mixed bulk outcomes. | API permissions/membership prepare grants and an assigned role. Failed deletion retains its row. | roles |
| `PRODUCT_ROLE_DETAIL` — `/products/resources/roles/$roleId` | View, edit/cancel, renamed role and grant replacement, reload and missing-role recovery. | API checks persisted name/grants; permission deletion must leave `permissions: []`. | roles, permissions |

## Organization and workspace views

Spec: [tenancy](../features/access/tenancy.e2e.ts). These are admin views of
tenant data. API creation or membership assignment is prerequisite setup, not
coverage of a browser create/assign flow.

| Route key and path | Browser actions asserted | Preparation or boundary | Spec |
| --- | --- | --- | --- |
| `ORGANIZATIONS` — `/organizations` | Search, Name facet, name sort, pagination and no selection/mutation controls. | Twelve API organizations; read-only overview. | tenancy |
| `ORGANIZATIONS_APIS_KEYS` — `/organizations/api-keys` | Organization picker, scoped search/Status facet, obfuscated key and grants, no clear key or mutation controls, empty recovery. | Two organizations and API-created organization keys; read-only list. | tenancy |
| `ORGANIZATION_MEMBERSHIPS` — `/organization-memberships` | Organization picker, twelve-member pagination, email/name search, role display and empty recovery. Invitations tab: Status facet, cancel, single/bulk delete and organization isolation. | API users, roles, memberships and invitations; invitation create/accept are not browser controls here. | tenancy |
| `WORKSPACES` — `/workspaces` | Organization picker, search/Name facet, description, scope switch and empty recovery. | API organizations/workspaces; read-only browser view. | tenancy |
| `WORKSPACE_MEMBERSHIPS` — `/workspace-memberships` | Direct navigation and placeholder heading. | Hidden from sidebar; no membership management UI is implemented. | tenancy |

## Workflows

Spec: [workflows](../features/access/workflows.e2e.ts). Organizations are
prepared through the tenancy API; workflows are built, tested, saved, edited
and deleted through the browser. Runs are started by API organization creates
and read back through the Runs tab and the API.

| Route key and path | Browser actions asserted | Preparation or boundary | Spec |
| --- | --- | --- | --- |
| `PRODUCT_WORKFLOWS` — `/products/workflows` | Empty state, recipe card opens a prefilled builder, saved workflow listed with its last run status, custom-event links between workflows, deleted workflow gone. | API organizations and one API workflow emitting a custom event. | workflows |
| `PRODUCT_WORKFLOW_NEW` — `/products/workflows/new` | Canvas trigger, condition and step nodes opening their inspector and "Back to flow", recipe prefill, dry run against a real organization with simulated writes and nothing written (each step node shows "Simulated"), required name and trigger, server validation pinned to the step and parameter it concerns, trigger select (catalog and custom events), "Started after" hint, add steps from the action menu, event-field prefill shown as a field pill, a step output dragged from "Data you can use" onto a parameter, click-to-insert into the input used last, typed custom event data (a picked field types its row), `WORKFLOW_EVENT_FIELD_CONFLICT` pinned on the event data, the catalog typing a custom event from its senders, step condition, disable, create; drag an action from the "Steps" palette onto the "+" before step 1 (it becomes step 1 and opens its inspector), reorder by dragging a step's handle below the next step, and move it back with ArrowUp on the focused handle (focus stays on the handle, "Step moved to position 1" is announced). | Role and organization ids are free text; no live member is added. The palette starts open only on a canvas wide enough to float it beside the steps (desktop); on phone and tablet the scenario opens it with its toggle. Not driven in a browser journey: a palette drop on the pane or on the add node, a palette click appending a step (`WorkflowCanvas` `DragAndDrop` story) and the drop line (visual only). | workflows |
| `PRODUCT_WORKFLOW_DETAIL` — `/products/workflows/$workflowId` | Run triggered by the next organization only (an earlier one is ignored), run status read only once it finishes, Runs tab status and event data, reload persistence of steps, condition field/operator/value, parameter and disabled state, a custom event starting a follow-up workflow (the follow-up shows as an "Open" node on the canvas), the loop warning in the toolbar, on the step that causes it and as a "Loop" edge on the canvas, "Fit the workflow", the leave-without-saving guard (Stay, then Discard and leave through a chained-workflow node), inserting a step from the "+" before a step and finding it in view, the save the server refuses with `WORKFLOW_LOOP`, a "Call your backend" step whose backend writes back with the causation header and the resulting "Loop prevented" run in the Runs tab, delete confirmation. | A loopback Node server stands in for the Product backend (`core.workflow.allow_private_targets` is on only in the test runtimes). Not clicked in a browser: "Run for real" (covered by `TestRunWorkflow_RunsNowAgainstTheGivenEventData`), workflow-level conditions, zoom in and out, moving a step from its inspector or removing it, "Keep going if this step fails", "Chain stopped" runs (component test only) and failed or running runs (stories only). | workflows |

## Licensing

Specs: [schema](../features/licensing/schema.e2e.ts),
[templates](../features/licensing/templates.e2e.ts),
[organization licenses](../features/licensing/organizations.e2e.ts),
[migrations](../features/licensing/migrations.e2e.ts).

| Route key and path | Browser actions asserted | Preparation or boundary | Spec |
| --- | --- | --- | --- |
| `PRODUCT_LICENSE_SCHEMA` — `/products/licensing/schema` | Missing-schema guidance; visual/text create/edit/cancel; String, Number, Boolean, Enum and both Limit usage shapes; rules, conversion, remove field, diagnostics and required/duplicate/range/shape validation. | Schema scenarios create through the UI; other license scenarios declare via API. | schema |
| `PRODUCT_LICENSE_TEMPLATES` — `/products/licensing/templates` | Missing-schema guidance, create/detail links, search, Status facet, name sort, pagination/page size and archived read-only rows. | API tiers prepare list/archive cases. No archive/delete UI is claimed. | schema, templates |
| `PRODUCT_LICENSE_TEMPLATE_NEW` — `/products/licensing/templates/new` | Required name/all schema values, out-of-range rejection and create with every field type. | API schema prerequisite. | templates |
| `PRODUCT_LICENSE_TEMPLATE_DETAIL` — `/products/licensing/templates/$templateId` | View/edit/cancel/save/reload, archived direct edit denied, missing-template recovery and followed value propagation preserving customer adjustments. | API organizations/adjustments prepare propagation; archival itself is API setup. | templates |
| `ORGANIZATION_LICENSE` — `/organizations/license` | Search, Tier facet, matched/custom/unlicensed states, Next/Previous pagination, selected/current-page/all-query selection, migration preview, preserve/discard adjustments and changed/unchanged/failed results. | API organizations, tiers and adjustments; deleting one reviewed customer via API triggers partial failure. | schema, organizations, migrations |
| `ORGANIZATION_LICENSE_DETAIL` — `/organizations/license/$organizationId` | Redirect to Usage, section links, unlicensed/missing states and return link. | API instantiated or unlicensed organization; detail actions appear in the children below. | organizations, templates, migrations |

The detail route registers these child pages outside `ROUTE_PATHS`:

| Child path | Browser actions asserted | API prerequisite or boundary |
| --- | --- | --- |
| `/organizations/license/$organizationId/usage` | Gauge/windowed values; within/at/exceeded/never states; limit selection and field query URL/reload; empty history; 24h/7d/30d/90d requests and pressed range; custom/no-limit states. | Usage is reported through the real API. Browser history/chart controls read it; the UI does not report usage. |
| `/organizations/license/$organizationId/values` | Omission and real server rule validation/error recovery, discard, minimal-diff adjustment, persisted custom values, tier comparison, unsaved navigation Stay/Discard choice and reverted-value navigation. | API schema/tier/license; some adjusted values prepare propagation/migration cases. |
| `/organizations/license/$organizationId/changes` | Instantiation, customer adjustment, followed update and migration entries; old/new tier display; load beyond fifty records. | API adjustments generate the long history; reading/loading history remains a browser action. |

## Provider integrations and events

Spec: [integrations](../features/integrations/integrations.e2e.ts).

| Route key and path | Browser actions asserted | Preparation or boundary | Spec |
| --- | --- | --- | --- |
| `PLATFORM_INTEGRATIONS` — `/platform/integrations` | Legacy redirect to selected product integrations. | Selected API product; no separate global provider configuration. | integrations |
| `INTEGRATION_CLERK` — `/platform/integration-clerk` | Legacy redirect to selected product Clerk setup. | Selected API product. | integrations |
| `PRODUCT_INTEGRATIONS` — `/platform/$productId/integrations` | Redirect destination renders SMTP provider overview. | Provider actions are exercised on their dedicated routes; card navigation is not individually asserted. | integrations |
| `PRODUCT_INTEGRATION_CLERK` — `/platform/$productId/integration-clerk` | Create/configure webhook secret, reset draft, pause/reload/resume, ingested user/activity visible and instance delete. | API posts invalid and correctly signed Clerk webhooks; real local ingestion is asserted. Live vendor API reconciliation is excluded. | integrations |
| `PRODUCT_INTEGRATION_SMTP` — `/platform/$productId/integration-smtp` | Required host, local SMTP create/connect, metadata update/reload, blank password preservation, pause/resume and delete cancel/confirm. | Owned Mailpit SMTP; local PLAIN/NONE configuration. External delivery/TLS vendors are excluded. | integrations |
| `PRODUCT_EVENTS` — `/products/events` | Categories/filter/no-results, checkbox selection, endpoint save/discard/reload, signing secret shown once and endpoint clear. | API creates an organization; the real queue delivers a signed webhook to an owned loopback receiver, whose body/signature are verified. | integrations |

## Email

Spec: [email](../features/integrations/email.e2e.ts).

| Route key and path | Browser actions asserted | Preparation or boundary | Spec |
| --- | --- | --- | --- |
| `EMAIL_TEMPLATES` — `/products/email/templates` | Empty list, New Template navigation, page size/Next/Previous pagination, single/bulk deletion leaving the unselected template. | API templates prepare pagination/deletion cases; builder lifecycle begins with UI creation. | email |
| `EMAIL_TEMPLATE_BUILDER` — `/products/email/templates/$templateId` | Name/subject/Monaco HTML autosave and Copy HTML; inferred STRING variable/required flag; NUMBER/BOOL/OBJECT/LIST schema editing, nested object/list fields and field/variable removal; form/raw examples, list rows, remove/save/reload; delayed initial Examples response blocks creation/save until saved values load, then preserves saved and new examples together; rendered iframe/subject, manual Refresh Preview; publish/reload; Send Test. All editor assets must stay on app origin. | API configures SMTP, prepares the independent typed-variable template and checks persisted drafts/examples. Mailpit receives the real addressed/rendered message; rendered preview verifies number/boolean/nested-value coercion. | email |
| `EMAIL_SENDS` — `/products/email/sends` | Recipient/subject/SENT record, Failed status no-results, Clear all recovery and page size/Next/Previous pagination. | UI test-send creates a real record; API sends prepare the smallest eleven-record pagination boundary through real local SMTP. Mailpit independently confirms delivery in the send journey. Read-only delivery history. | email |

## Remaining behavior boundaries

The suite covers the route families and primary lifecycles above. Ancillary
dashboard/provider-card links still lack individual click assertions. Do not
count their route annotation as proof that these particular controls passed.

The ordinary complete gate uses desktop Chromium. The separate
`playwright.review.config.ts` validates selected changed-feature scenarios on
mobile, tablet and desktop with screenshots/video; each PR records its exact
selection and results. This is device emulation, and does not imply all existing
scenarios have passed on every device or that Firefox/WebKit or real hardware
were tested. Numeric and boolean email examples, nested object fields and a list
of objects are covered, but arbitrary recursive nesting and every schema/rule
combination are not enumerated in browser scenarios. The suite observes genuine
validation failures, rejected credentials, missing records and partial migration
failure; it does not systematically inject HTTP timeouts, dropped connections,
server failures or retry permutations. Keep deterministic parser and contract
combinatorics in the separate fast suites.

External Clerk API reconciliation needs a vendor test tenant and scoped API key.
It is separate from the signed local ingestion tested here. External SMTP
provider transport/deliverability likewise needs its own controlled environment;
the ordinary suite verifies real local SMTP and Mailpit delivery. Placeholder
settings/workspace-membership pages and read-only tenant lists do not have
missing browser CRUD coverage: those actions are absent from the implemented UI.

When adding a route or action, update the relevant row and scenario together,
name the API prerequisites, and run that domain before the full suite. Add the
new failure recipe to the owning troubleshooting file. Preserve the separate
unit, Storybook and backend contract suites; this inventory does not replace
them.
