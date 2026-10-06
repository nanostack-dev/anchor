# Browser tests

Playwright drives the built Anchor UI in Chromium. Use roles, labels and visible
text, and wait for observable results with `expect` rather than fixed sleeps.

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

Add one user journey at a time. Start with admin invitations, product management,
then product-scoped organizations, memberships, API keys, workspaces, roles,
licensing and email. Reads can join `e2e/smoke/`; writes belong in the local suite
with unique fixtures and cleanup scoped to those fixtures. Run cross-browser
journeys once Chromium coverage is stable. Storybook tests remain the faster
place for component states and validation combinations.
