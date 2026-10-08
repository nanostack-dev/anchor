# Stripe organization billing in Anchor

Stripe is a native product integration in Anchor. Configure it in **Integrations
→ Stripe**, manage recurring prices in **Pricing**, and manage each organization's
subscription from its **License → Billing** tab. These pages use the regular
Anchor login, selected product, and authenticated product-scoped API.

Anchor owns license schemas, templates, bespoke adjustments, and license history.
Stripe owns prices, customers, subscriptions, invoices, and hosted payment pages.
Anchor persists billing associations and received events in its database and
reconciles subscription changes through its existing licensing service.

## Start the local sandbox

Prerequisites: Docker with Compose, the repository's Go/Node/pnpm versions,
installed frontend dependencies, and Stripe CLI logged in to a sandbox.

From `anchor-ui`:

```sh
pnpm install --frozen-lockfile
stripe login
pnpm stripe:prototype
```

Open <http://127.0.0.1:3307>. Sign in through Anchor's normal login using the local
owner credentials stored privately in the managed runtime's `owner.json`; the
launcher prints that file's path, without printing the password. Select the demo
product named in the launch output, then open **Integrations → Stripe**.
If the development tools panel covers a control, close it with its **Close
tanstack query devtools** button before continuing.

The launcher starts a managed local Anchor runtime when none exists. It initializes
a local owner, a demo product, Free/Pro/Business license templates, and two
organizations on Free. It then creates or updates that product's native Stripe
integration through the authenticated API, forwards signed sandbox webhooks, and
serves the regular Anchor app. There is no separate billing HTML entry or bridge
server. Browser requests use the same main API client and session as the rest of
Anchor. An empty native catalog is seeded with Pro at USD 29/month and Business
at USD 99/month; an existing catalog is preserved.

The local demo uses the provider's **LOCAL_CLI** authentication method: the main
Anchor backend calls the already authenticated Stripe CLI and checks its pinned
sandbox account. The launcher does not extract OAuth tokens or OS keychain
credentials. It removes `STRIPE_API_KEY` from child environments so an unrelated
environment key cannot override the selected CLI account. Stripe live mode is
refused.

For a configured deployment, choose **API_KEY** in the same native integration
and provide a Stripe test secret or restricted API key, account ID, webhook
signing secret, and Anchor return URL. Secret fields are write-only and encrypted
at rest. LOCAL_CLI requires the CLI on the backend host and is intended for this
local sandbox workflow.

## Preserve data and stop safely

If this worktree already has a running managed Anchor backend, the launcher
borrows it. It reuses the saved product when the runtime ID and Stripe account
match, preserving its templates, organizations, licenses, and native billing
records. It initializes the fallback only when no fallback has been selected.
It does not reset another product or replace an API_KEY configuration with
LOCAL_CLI. An initialized borrowed runtime needs its existing private owner
credentials; a missing owner file is an actionable startup error.

Use **Ctrl-C** in the launch terminal to stop the owned UI and Stripe listener.
The managed Anchor backend stays running, preserving its database and the demo
product for restart. From another terminal:

```sh
pnpm stripe:prototype --stop
```

To remove a disposable runtime created by this launcher, including its local
database, use `pnpm stripe:prototype --stop --teardown`. This refuses a borrowed
runtime or a runtime whose saved identity has changed. The next launch after
teardown creates a fresh product.

After backend source changes, stop the UI/listener and refresh the managed backend
in place before relaunching:

```sh
pnpm stripe:prototype --stop
node scripts/e2e-runtime.mjs refresh
pnpm stripe:prototype
```

Refresh compiles before stopping the verified backend process, keeps its API
port, database, Redis, Mailpit, and secret files, and applies normal migrations
on startup. It refuses changed process ownership. Do not use a fresh-runtime
command to update a demo whose data you want to keep.

The stop command checks the launcher's script, working directory, and process
start time. After an unexpected launcher exit, it stops only recorded child
groups whose process identities and directories still match. Repeated terminal
signals do not interrupt cleanup. It does not kill arbitrary owners of occupied
ports.

Private launch metadata and scrubbed logs live under ignored
`anchor-ui/.ui-craft/stripe-prototype/`, with private files using mode `0600`.
Legacy prototype state stays in that scratch directory for an explicit migration;
the native application does not automatically import old associations. A new
runtime gets a new product, so old Stripe customers cannot silently target fresh
Anchor organizations. Stripe sandbox objects remain in the account after shutdown.

## Manage prices and organization billing

1. In **Pricing**, create a recurring price and choose its Anchor template,
   currency, and monthly or yearly interval. Amounts use the currency's
   **smallest unit**: `2900` means USD 29.00, CAD 29.00, or EUR 29.00. Supported
   amounts are integers from `1` through `99999999`.
2. Open an organization's **License → Billing** tab and start Stripe Checkout.
   An optional trial lasts `0` through `30` days. Use a Stripe sandbox payment
   method on the hosted page. The test card `4242 4242 4242 4242` accepts a future
   expiry and any three-digit CVC; see [Stripe's testing documentation](https://docs.stripe.com/testing).
3. The signed subscription event selects the price's license template. **Sync**
   fetches current Stripe state and repairs a missed or failed reconciliation.
4. Change the subscription price to move between priced templates. Stripe invoices
   prorations and can hold an unpaid change pending; Anchor grants the target
   license only when Stripe applies that change.
5. Open the customer portal to manage payment details and invoices. **Cancel**
   schedules the subscription to end at its period boundary; **Resume** removes
   the scheduled cancellation before it ends.
6. Archive a price to withdraw it from future checkouts. Existing subscriptions
   retain their price. Change amounts by creating a replacement price and moving
   subscriptions explicitly.

Choose an active **fallback template**, initially Free, for ended subscriptions.
Active and trialing subscriptions select their paid price's template. Canceled,
unpaid, paused, and expired incomplete subscriptions select the fallback.
`past_due` and `incomplete` retain the current license while payment is unresolved.
A pending price update does not grant its target license before Stripe applies
it. This is the sandbox's grace policy; live collection and access enforcement
require an explicit product policy.

Template changes use **CARRY_FORWARD**, preserving bespoke license adjustments.
Reconciliation skips organizations already on the target template, so repeated
events do not restamp licenses. Anchor's existing template sync continues to
propagate unadjusted fields and maintain license history.

## API and verification

Native billing endpoints live under
`/v1/products/{product_id}/billing/stripe`; management requests require Anchor
authentication and access to that product. The Stripe listener forwards to the
signed `/v1/products/{product_id}/billing/stripe/webhook` endpoint. The regular
integration lifecycle creates, configures, verifies, disables, and deletes the
STRIPE provider. Billing configuration and state remain scoped to its product.

The main API contract is `apps/anchor/cmd/http/openapi.yaml`; generated server and
client types come from that contract. Follow the repository's standard generation
commands, rather than editing generated files:

```sh
cd apps/anchor
./generate_anchor.sh
cd ../../anchor-ui
pnpm openapi-ts
```

The `internal/stripebilling` module owns native orchestration, durable storage,
and reconciliation. Its `billing` package owns license policies and the generated
Anchor billing DTOs; regenerate those with
`go generate ./internal/stripebilling/billing` from `apps/anchor`. They do not
define a separate HTTP server or browser client.

Stripe API operations use the official `github.com/stripe/stripe-go/v87` client
with typed request parameters and Stripe resources. Each integration gets its
own client and backend configuration; no global API key is changed. API_KEY uses
the SDK's HTTP backend. LOCAL_CLI is a development backend for the same typed
SDK calls, forwarding through the existing CLI sandbox login without extracting
credentials. Account pinning, sandbox guards, bounded requests, cancellation,
idempotency keys, and secret-safe errors apply to both transports.

The managed browser suite covers the native integration and billing pages using
real local Anchor authentication, products, and organization prerequisites, with
Stripe billing responses replaced at the API boundary. Ordinary browser tests
need no Stripe login and make no external Stripe requests. Run the affected
selection and the complete gate through the existing `test:e2e:affected` and
`test:e2e:verify` commands. Review screenshots, videos, and failure traces stay in
ignored local scratch or CI artifacts, separate from private runtime credentials.

When the demo is running, use a separate managed runtime namespace for verification:

```sh
ANCHOR_E2E_RUNTIME_NAMESPACE=native-verification pnpm stripe:prototype:test
ANCHOR_E2E_RUNTIME_NAMESPACE=native-verification pnpm test:e2e:review integrations/stripe-billing.e2e.ts
```

The namespace selects separate local metadata, credentials, binaries, and Docker
project ownership. Verification cleanup cannot stop or reset the default demo.
