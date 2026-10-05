# Browser tests

Playwright drives the built Anchor UI in Chromium. Use roles, labels and visible
text, and wait for observable results with `expect` rather than fixed sleeps.

## Whole application

Start with [the browser test guide](guide/README.md). It links the feature matrix,
ten-article research, performance evidence and reproduced failure recipes.

The full suite requires Node 24, pnpm 11.1, Go 1.27 and running Docker with Compose.
It starts a disposable TimescaleDB, Redis, Mailpit and the actual Anchor binary,
builds this worktree once, and creates its own accounts. No dev credentials or
external vendor account is required.

```sh
pnpm install --frozen-lockfile
pnpm exec playwright install chromium
pnpm test:e2e:app
pnpm test:e2e:ui
```

The app preview uses `http://127.0.0.1:3015`; Playwright UI uses
`http://127.0.0.1:9351`. Stop the UI command with Ctrl-C to tear down its owned
runtime. After an interrupted process, `node scripts/e2e-runtime.mjs stop`
cleans up only this worktree's disposable services. The runtime refuses foreign
processes and records ownership under ignored `e2e/runtime/.local/`.
After a managed run, `node scripts/e2e-runtime.mjs verify-stopped` checks that
its metadata, startup lock and containers are gone. PR CI enforces this too.

Run one feature with `pnpm test:e2e:app e2e/features/licensing/schema.e2e.ts`.
For repeated local iterations, keep `node scripts/serve-e2e-full.mjs` running and
use `E2E_REUSE_SERVER=1 pnpm test:e2e:app`. Rebuild it after app/backend changes;
the browser fixtures reject a mismatched frontend fingerprint. CI starts clean.

`pnpm check:e2e` checks the route-to-scenario inventory; update it with
`pnpm check:e2e --write` after reviewing a route change. This is a maintenance
guard, not an executable coverage measurement. The isolated PR workflow runs
the full suite without secrets and uploads results plus failure screenshots.
Reports, traces and generated credentials remain ignored; UI mode records local
diagnostic traces, so keep its server on loopback and recordings out of commits.

## Login and Products smoke

This suite covers the public browser boundary: guest access protection, real
login, Products navigation and search, session persistence after reload, and
logout followed by access protection. It uses an existing dev admin account and
does not create, edit or delete product data. It does not mock the API or inject
authentication state.

From `anchor-ui`, with Node 22.12+ or Node 24:

```sh
pnpm install --frozen-lockfile
pnpm exec playwright install chromium
cp .env.e2e.example .env.e2e.local
# Fill E2E_EMAIL and E2E_PASSWORD with an existing dev admin account.
pnpm test:e2e:smoke
```

`.env.e2e.local` is gitignored. Environment variables take precedence over the
file. Missing credentials fail the authenticated test with an actionable error;
they never silently skip coverage. The guest example can run without credentials:

```sh
pnpm test:e2e:smoke --grep guest
```

The default run builds the current worktree and serves it at
`http://127.0.0.1:3014`. A Vite preview proxy forwards `/health` and `/v1` unchanged to
`E2E_API_URL` (default `https://apidev.tryanchor.dev`), so the branch can use the
real dev backend without changing the backend's CORS policy. The proxy is only
part of the test server. Use a dev account with access to Products; no fixed
product names or counts are required.

To test an already deployed dev UI instead, set its URL explicitly:

```sh
E2E_BASE_URL=https://dev.tryanchor.dev pnpm test:e2e:smoke
```

That mode exercises the deployed frontend; `E2E_API_URL` applies only to the local
test server. Browser contexts are isolated per test. Traces, video and screenshots
are disabled because login recordings can contain credentials and session
tokens. The local HTML report (`pnpm exec playwright show-report`) and
`test-results/` may still contain dev account or product names; both directories
are ignored and must stay local.

The **Browser smoke (dev)** workflow is manual and tests the selected branch's
build against the dev API. Configure repository secrets `ANCHOR_E2E_EMAIL` and
`ANCHOR_E2E_PASSWORD` before running it. It intentionally does not upload login
reports or browser state. Normal PR checks continue to run without dev secrets.

## Mutating local suite

The existing bulk-delete suite stays separate and refuses remote backends:

```sh
ANCHOR_E2E_API_URL=http://127.0.0.1:8080 pnpm test:e2e
```

Run it only against a local disposable database. Its test account is created on
an uninitialized backend, so reusing a backend initialized with another account
requires resetting that disposable database.

## Extending coverage

Use the domain folders under `e2e/features/`, following [the guide](guide/README.md).
Update its coverage matrix and troubleshooting recipes in the same change.
The deployed smoke remains a small read-only health check. Storybook retains
component states and validation combinations; browser tests verify integration
and persistence through actual user journeys.
