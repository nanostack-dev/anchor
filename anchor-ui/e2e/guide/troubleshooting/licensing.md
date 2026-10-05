# Licensing browser tests

The maintained scenarios live in `e2e/features/licensing/`: `schema.e2e.ts` owns schema
declaration and editing; `templates.e2e.ts` owns template forms, archived read-only
views and follower propagation; `organizations.e2e.ts` owns customer usage, values,
history and navigation guards; `migrations.e2e.ts` owns selection, cross-page cohorts
and outcomes. `helpers.ts` contains only API prerequisites and repeated domain
operations. Import `test` and `expect` from `e2e/support/fixtures` so each scenario
receives an isolated product and browser state.

Run a focused investigation with the persistent local stack already ready:

```sh
E2E_REUSE_SERVER=1 pnpm exec playwright test --config playwright.app.config.ts --project chromium --no-deps e2e/features/licensing --reporter=list --output=test-results/licensing
```

Use the normal full-app command when the stack is not already running. The `--no-deps`
shortcut assumes bootstrap completed on that exact runtime. A local process unable to
reach loopback can make Playwright attempt a second web server despite reuse; fix host
execution permissions, rather than treating the subsequent Docker permission error as an
application failure.

## Scope and efficient prerequisites

The UI exposes Visual and Text schema modes and the five field types LIMIT, NUMBER,
BOOLEAN, ENUM and STRING. Text is the license DSL, not JSON. LIMIT requires either GAUGE
or WINDOWED_COUNTER. Browser tests cover every type in one schema and template journey;
parser combinatorics and rule permutations stay in the existing fast unit tests.

Creation of organizations and reporting runtime usage require a product API key. A
platform session can declare schemas and templates but `POST
/products/{id}/organizations` rejects that token with HTTP 401. Call `await
world.productAPI()` for runtime prerequisites; it lazily creates a scoped key from the
built-in permission catalog. Keep `world.api` for platform management. Do not copy keys
into tests, read developer credentials or create shared state in a deployed environment.

Use real API prerequisites for independent data, and use the real UI for the mutation
under test. Template creation and editing, schema creation and editing, customer
adjustment and migrations all run through the browser. The UI currently offers no
template archive action: the archive endpoint supplies an archived prerequisite, and
browser assertions cover its status, filter, direct edit URL, read-only view and
exclusion from migration targets. Instantiation and usage reporting are also runtime API
operations.

The string pattern validator matches the entire string. A prefix expression such as
`^https://` rejects `https://example.test`; use `^https://.*$` for URL prerequisites.
This failure is schema behavior, not a request timing problem. Boolean template inputs
start visually unchecked while their backing value is unset; explicitly interact with
the switch to give the form a boolean value before submitting.

Missing-resource tests must still use a valid public identifier. `missing-template`
fails path validation with HTTP 400, so it exercises the generic request error rather
than “Template not found”. The template scenario uses the `ltpl_` prefix followed by 27
zeroes: it is syntactically valid and has no record in the disposable database.

## Locators and readiness

The breadcrumb repeats the active tab name as a link. Scope Usage, Changes and Values to
`getByRole("navigation", { name: "License sections" })`, then query the named link. The
header and history also both contain “Instantiated”; scope the historical event to a
list item. Template Status exists both as a sort button in the table header and a filter
trigger; the latter has `aria-expanded`, so query the named button with `expanded:
false` before opening it.
The same source template name appears in both instantiation and a subsequent migration;
scope it to the list item containing “Moved to another tier”. Usage-card number and
unit spans can concatenate in text content, so numeric checks allow whitespace between
the value and “of” while still checking the exact value and limit. Their usage bar also
exposes a complete numeric `aria-label`.

Schema field panels retain collapsed inputs during their animation. Keep controls scoped
to the accessible field group, and verify the active panel rather than selecting an
arbitrary first input or adding a delay. Hidden field panels must remain outside the
accessibility tree while retaining `inert` keyboard protection. The tests should fail if
collapsed controls become exposed again. The editor expands one field at a time;
Storybook interactions that inspect two usage shapes must assert the first visible
control, expand the second field and assert its control. The final submitted payload
still proves both shapes survive conversion and saving. Do not query hidden controls to
make an outdated interaction assertion pass.

A repeated warm benchmark caught `chooseOption` timing out while selecting Boolean:
another portal's Boolean text intercepted the pointer, then the target option became
invisible. This establishes a portal hit-test race; it does not establish whether
exit timing, popup alignment or a library defect caused that portal overlap. The
unchanged scenario also passed eight focused repetitions, so a single green rerun
does not demonstrate reliability.

The licensing helper now waits for no accessible listbox, opens the named combobox
with Enter, verifies `aria-expanded="true"` and one accessible listbox, then activates
the exact named option inside that listbox with Enter. It verifies the trigger's
selected label, `aria-expanded="false"` and no accessible listbox before returning.
Keep these state checks when changing the helper. A probe found that a closed select
can retain a listbox outside the accessibility tree; requiring
`getByRole("listbox", { includeHidden: true })` to have zero DOM elements incorrectly
blocked an otherwise completed selection. Synchronize the operator-visible open and
closed states without force clicks, fixed delays, retries or a larger timeout. The
[Base UI Select documentation](https://base-ui.com/react/components/select) explains
its selected-item popup alignment and keyboard text navigation; the actual Enter
activation and resulting value are verified through the local UI.

The repaired helper passed eight repetitions of the unchanged visual schema journey
with one worker, then all 17 licensing scenarios with four workers, using Node 24.
These focused checks preserve every schema/type/rule and persistence assertion;
they are reliability samples, not a replacement for the full-suite performance matrix.

A creation notification can overlap the template editor's bottom-right Cancel
button during a rapid create-to-edit journey. The failure log showed the actual
“License template created” dialog intercepting the click; that dialog exposes a
“Close toast” button. Assert the specific notification, activate its close button
with Enter and assert it disappears before continuing. Keyboard activation also
avoids chasing the close control while its entrance animation moves it; a pointer
click can wait for stability until the toast expires and then time out on the
removed control. Do not force the covered form
button, remove the notification through DOM evaluation, wait a fixed toast duration
or increase the action timeout. This keeps the same controls usable and the test fast.

Wait for the relevant visible result or a specific API response registered before
clicking. Customer adjustments assert the exact PATCH payload, unchanged values and
adjusted field set; they do not merely look for a toast. Keep both client omission
validation and a real server rule rejection: setting `max_flows` above its declared
maximum returns HTTP 400, keeps the draft visible with its field error and leaves the
persisted license unchanged. Repair the same draft and save successfully to verify
error recovery. Template propagation is durable
background work, so one bounded `expect.poll` observes the real license until followed
values update while customer overrides stay intact. Do not replace this with a sleep or
increase every test timeout.

Usage ranges are buttons inside the Time range group, exposing `aria-pressed`. Changing
24h, 7 days, 30 days and 90 days checks the real series endpoint and pressed state.
LIMIT cards exercise within, at, exceeded and never-reported states; a windowed report
supplies `from` and `to` around the present, while gauges supply only their current
quantity. History aggregation may retain different bucket granularities, so assert the
range contract and actual response rather than pixel positions or hard-coded chart
timestamps. Selecting a limit also sets the `field` query parameter. Assert that URL
and reload while a non-default field is selected to prove a shared link preserves
which history is displayed.

## Migration and history regressions

The customer Values page previously treated any draft key as dirty, even when its
value equaled the saved license. Changing `max_flows` from 10 to 42 and back to 10
hid the adjustment bar but blocked Usage navigation with a warning that “0 fields”
had changed. The repair derives dirty state from the same actual field differences
used by the adjustment payload, before the unconditional navigation-blocker hook.
The regression first verifies Stay and Discard for a genuine edit, then reverts a
new edit and requires Usage navigation without the warning. This sequence passed
against the rebuilt local app; preserve both the genuine-edit and reverted-edit
assertions when refactoring draft handling.

“All in current page” and “All matching query” are distinct operations. The cohort test
creates 21 customers because the organization license list shows 20 per page. It
searches a unique cohort, goes Next to the single remaining row and Previous back to
20, exercises page selection and clear selection, chooses every
matching customer and verifies all 21 persisted licenses plus the excluded customer's
unchanged tier. Keep this smallest boundary-crossing dataset; a much larger cohort adds
cost without proving another behavior.

Migration outcome labels are “Set”, “Already there” and “Failed”. Their headings include
a visually separated number which can be concatenated in the accessible name; match the
exact outcome and number with optional whitespace. Assert per-customer details and
persisted values alongside the aggregate count. To exercise a real partial failure,
remove one selected organization through the local API after opening the review dialog:
the run must report that customer as failed and still grant the surviving customer's
license. Do not synthesize this response with network interception.

The history pagination regression creates 51 distinct adjustments after instantiation,
yielding 52 records. The old controller always requested offset zero with a limit of 50
and never supplied the view's `onLoadMore` callback, leaving older events inaccessible.
The scenario requires a “Showing 50 of 52” indicator, loads older changes and confirms
the original historical instantiation is reachable. Preserve this regression when
changing query pagination or cache invalidation; 51 writes are the smallest clear case
that separates more than one history page from the normal single-page journey.

## Adding a capability

Read the corresponding real page and OpenAPI declaration first. Add a focused browser
scenario to the owning file and a `// Covers:` comment using the real route key from
`src/routes/routePaths.ts`; the shared drift check tracks route additions. Usage,
Changes and Values are children of ORGANIZATION_LICENSE_DETAIL and each requires an
explicit scenario even though they share that key. Keep mutable prerequisites within the
test's product. Let the fixture cascade cleanup dispose of them. Record newly reproduced
failures and their verified solution here; distinguish product fixes, auth mistakes and
locator mistakes before weakening an assertion.
