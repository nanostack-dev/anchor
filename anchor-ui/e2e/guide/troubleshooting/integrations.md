# Integrations, events and email

Run this domain with the same managed local runtime as the other domains:

```sh
E2E_REUSE_SERVER=1 pnpm exec playwright test --config playwright.app.config.ts \
  --project chromium --no-deps e2e/features/integrations \
  --reporter=list --output=test-results/integrations
```

Use `E2E_REUSE_SERVER=1` only when this worktree's managed frontend is already
running. Otherwise use the regular app-suite command, which starts it.

## Local integration fixtures

SMTP setup starts asynchronous connection verification. An enabled integration
can still be `CONFIGURING`; the visible `Live` badge represents the enabled
switch, not a successful SMTP handshake. `configureSMTP` polls the real API for
`ACTIVE` before a send prerequisite is considered ready. The runtime's Mailpit
accepts SMTP AUTH PLAIN over plaintext, matching the backend CT configuration.

Keep Clerk's optional API key empty. Saving an API key immediately starts real
Clerk reconciliation against the vendor API. The local browser test covers the
actual configuration UI and signed incoming webhooks with a generated webhook
secret. It verifies user creation and audit activity. Hosted reconciliation and
vendor account provisioning need separate integration credentials and are not
claimed by these local tests.

Legacy integration routes must wait for product selection and then redirect to
the product route. Rendering the product component from the legacy route caused
a strict route-parameter lookup to fail before selection settled. The initial
redirect repair exposed a second defect: the installed TanStack `Navigate`
component uses a layout effect and compares the complete props object by
identity. New inline JSX props during navigation repeatedly updated the router,
raising React's maximum-depth error even though the final URL assertion passed.
Keep the separate legacy-route scenario and the shared uncaught-error assertion.
Use a navigation effect keyed by loading state and product ID; memoizing only
the nested `params` object does not stabilize the complete `Navigate` props.

For secret-preservation checks, save a generated local secret once, then submit
an unrelated update with the secret field blank. Verify the behavior using that
same secret afterward; an obfuscated marker alone does not prove preservation.

## Mailpit isolation

Mailpit's inbox is shared across workers. Never clear the inbox, assert its
global total, or inspect the latest message without a recipient filter. Each
test generates a unique recipient and polls for the matching message. Product
deletion cleans Anchor's records; Mailpit messages belong to the disposable
runtime and disappear at teardown.

## Draft persistence and event delivery

Email draft edits debounce before persistence. Wait for the real draft API to
contain the intended body and variable schema before publishing or reloading.
Do not add a fixed sleep to make the race disappear. Examples save separately
from the draft; wait for their own endpoint before checking a reload.

Monaco's native EditContext textbox is covered by its rendered text. Click the
accessible `HTML editor` region to focus it, then send keyboard commands to its
textbox. With the suite's Windows `Desktop Chrome` user agent, use `Control+A`,
`Backspace`, then `keyboard.insertText`. Clicking the textbox directly is
intercepted; merely focusing it and inserting over a selection left old content
in the real draft. Polling the draft body proved that clearing first replaced
the whole document. Keep the keyboard path instead of evaluating Monaco's model
or forcing a click through the overlay.

The builder uses `LocalEditor`, which loads Monaco and only its HTML/editor
workers from lazy local chunks. The React wrapper's default loader downloads
editor assets from a CDN. Configure the local Monaco API before mounting that
wrapper, as shown by the [Monaco React Vite integration](https://github.com/suren-atoyan/monaco-react#use-monaco-editor-as-an-npm-package)
and [Monaco ESM integration](https://github.com/microsoft/monaco-editor/blob/main/docs/integrate-esm.md).
The browser scenario records requests and requires every editor asset to come
from the loopback app; package setup must not reintroduce external asset loads.
Keep `monaco-editor` an exact direct dependency and use `.js` in exported ESM
subpaths for its strict package exports. `LocalEditor.stories.tsx` retains empty,
single-line and realistic long invoice HTML fixtures for component verification.

The realistic doctype and attribute fixture exposed stock light-theme text below
the required contrast ratio. Even Monaco's built-in high-contrast light theme
left the doctype gray at 3.94:1. `LocalEditor` inherits the usual light theme and
sets only those HTML tokens to the application's semantic `--foreground` color,
converted to the hex format Monaco requires. Keep the accessibility checks and
the real doctype/attributes in the story; disabling checks or removing markup
would hide the defect. The same fixture covers a 320px container and doubled
editor text size.

Publishing originally kept the `Draft only` badge visible after the mutation
completed. The generated TanStack keys contain an object with `_id`, so string
keys such as `["getEmailTemplate"]` do not invalidate them. Mutations must use
the generated query-key factories; keep the immediate `Published` assertion
before reload so a stale cache cannot pass as persistence coverage.

Clerk signs the exact incoming bytes, including whitespace and property order.
The original handler reconstructed the decoded JSON before validating it;
correctly signed unsorted JSON then failed with
`INTEGRATION_WEBHOOK_VALIDATION_FAILED`. Go fixtures used sorted `json.Marshal`
output, hiding that mismatch. Preserve the wire body before generated decoding,
and keep the browser scenario's original `JSON.stringify` order in its signed
request. Do not sort test JSON to accommodate the defect.

Raw-body capture is bounded to 1 MiB before decoding; an oversized webhook
follows the existing `400` invalid-body response. Middleware unit tests require
unchanged wire bytes, untouched non-webhook requests and rejection before the
handler on overflow. Keep that bound when changing the capture path.

The event test owns a loopback HTTP receiver, selects only `organization.created`
in the visible catalog, saves the endpoint in the UI, and creates an organization
through the supported API to trigger delivery. It checks the received envelope
and Standard Webhooks signature using the one-time secret returned by the UI
save request. Always close the receiver in `finally`.

Send History is a read-only delivered feature: its status filters are tested.
Single and bulk deletion apply to the email template list, which exposes those
actions. Do not describe API-seeded sends as coverage of an absent send-creation
page or add nonexistent history deletion actions.

## Email controls and regressions

The builder journey checks the actual clipboard after **Copy HTML**, waits for a
real POST triggered by **Refresh Preview**, and then checks publishing, reload,
test sending and delivered Mailpit output. Clipboard permissions belong to this
test's browser context and must not be granted globally.

The typed-form journey changes the schemas through the visible NUMBER, BOOL,
OBJECT and LIST controls, edits object properties and list item properties, and
removes a property, a variable, a list row and the saved example. It verifies the
real draft and example endpoints before reload. Repeated nested form inputs have
the same accessible labels; positional locators are limited to these displayed
repeated inputs. Examples display detected range variables before other
variables, while the Variables panel keeps schema order. Do not assume those
two orders are identical.

Keep the numeric formatting and false conditional in the preview assertion.
Sending the form strings `"12.5"` and `"false"` directly to Go templates rendered
`%!f(string=12)` and took the true branch. Schema-aware payload conversion must
produce a number and Boolean, while OBJECT and LIST inputs must produce JSON
objects and arrays. Also keep `printf "%.2f" .count` in the template: scanning
quoted text as a variable incorrectly added `2f` to the Examples form. Variable
detection must skip quoted template literals.

The pagination journey creates eleven templates and eleven real SMTP sends,
then selects **Show 10** and visits both pages with **Next** and **Previous**.
Eleven is sufficient to test the endpoint boundary without a large fixture.
List responses must report the total matching rows, not `len(items)` from the
current page; otherwise the table disables Next. The send-history filter must
map both status and template ID from API query parameters into the scoped
repository query. The browser's Failed facet must hide the successful delivery
and Clear all must restore it. Keep this UI assertion even when API filter tests
also exist.

The eight scenarios in this folder passed together in the Chromium app run on
2026-10-05 with two workers and zero retries. That verifies the local integration
configuration, signed Clerk ingestion and event delivery, SMTP email journey,
typed forms, removals, filters and pagination described above. Hosted vendor
reconciliation remains outside these fixtures. A domain pass is not a pass for
the entire app; use the current whole-suite report before making that claim.
