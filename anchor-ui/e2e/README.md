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

The app preview uses `http://127.0.0.1:3015`; Playwright UI uses the canonical
root URL `http://127.0.0.1:9351`. Open that root URL after restarting the launcher;
an old bookmarked websocket URL can refer to the previous process.
The UI launcher starts and verifies one managed preview
before opening Playwright, so Reload can reconnect without starting it again.
It accepts an existing preview only when its live backend belongs to this
worktree and its run ID, API URL and frontend fingerprint match. It runs the
idempotent browser bootstrap before opening the UI, whose feature selection
does not automatically run project dependencies. Select and rerun cases in the
UI. Stop the UI command with Ctrl-C to tear down the preview and runtime it
started. An explicitly prestarted preview remains its original owner's
responsibility. A live backend without a ready preview is refused; start its
managed preview explicitly or stop that backend first. After an interrupted
process, `node scripts/e2e-runtime.mjs stop` cleans up only this worktree's
disposable services. The runtime refuses foreign
processes and records ownership under ignored `e2e/runtime/.local/`.
After a managed run, `node scripts/e2e-runtime.mjs verify-stopped` checks that
its metadata, startup lock and containers are gone. PR CI enforces this too.
A hard-killed run, or a worktree removed while its runtime was up, leaves
containers that no metadata tracks. Every runtime start, `stop`, and
`pnpm test:e2e:clean` remove them: this worktree's untracked runs, and runs
whose owning worktree directory no longer exists. Live runtimes of other
worktrees are left alone.

For a visible demonstration, keep prerequisite setup fast and pace only the
tests you launch in the UI:

```sh
pnpm test:e2e:ui --headed --workers 1 --slow-mo 250
```

`--headed` shows Chromium during execution, `--workers` sets interactive
parallelism and `--slow-mo` paces browser actions in milliseconds. Numeric
values must be positive integers. The default retains normal browser speed
and configured worker count. `pnpm test:e2e:ui --help` and
`pnpm test:e2e:ui --print-config --headed --workers 1 --slow-mo 250` inspect
options and the launch plan without opening ports or starting services.

Before handing over the UI, verify the browser state:

1. Open `http://127.0.0.1:9351` and clear text, status and changed-test filters.
2. In Testing Options, select both `bootstrap` and `chromium` projects.
   Playwright remembers filters in the browser; the launcher cannot reset them.
3. Confirm all feature folders and scenarios are visible against the current
   discovered inventory. Bootstrap alone shows only the prerequisite login case.
4. Leave the run idle with Run all enabled when the user will launch it.
   Keep the launcher process running; close it with Ctrl-C after the demonstration.

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

## Managed test preflight

`pnpm test:e2e:preflight` checks prerequisites for a fresh managed run;
`pnpm test:e2e:preflight --reuse` verifies an already running owned preview.
Add `--ui` to check the interactive UI port too. Run these from the frontend
directory. The command emits a JSON `READY` or `BLOCKED` report with a stable
reason, corrective action and elapsed time; blocked checks exit nonzero.
`coverage: prerequisites_only` means no application scenarios have run.

The gate checks Node 24, a compatible pnpm version from `package.json`, the
backend Go toolchain from `go.mod`, loadable Playwright/Vite dependencies,
Docker/Compose, and preview port ownership. A blank headless Chromium is briefly
launched to verify the browser installation and its system dependencies. Each
tool/browser probe has a five-second limit. Prepare an unavailable Go toolchain
with `go version` in the backend module before retrying. Dependencies and browsers
are installed explicitly through the setup commands above.

A fresh check requires no running application and refuses occupied preview ports,
existing runtime metadata or a pending startup. Reuse additionally requires the
owned backend to be active, backend/toolchain and frontend fingerprints to match,
and the preview identity and readiness endpoint to agree with that runtime.
Failures leave services and unrelated listeners untouched. Stop a runtime through
its owning manager; use the existing owned cleanup and `verify-stopped` procedure
for an interrupted launch. Rebuild the managed preview after source changes.

Application and responsive-review commands enforce the gate before Playwright
execution. Complete verification and affected-run commands require fresh checks;
the interactive launcher verifies either a fresh start or an existing owned
preview, then retains its normal ownership checks. Direct
`node scripts/serve-e2e-full.mjs` startup is gated too. Set
`E2E_REUSE_SERVER=1` for app/review iteration only when that preview is already
ready; reuse no longer falls back silently to a fresh launch. Help, print-config,
selection planning and Playwright `--list` remain available without a Docker
runtime. `test:e2e:status` remains an informational diagnostic report, not a
readiness gate. Storybook, motion, deployed smoke and legacy test commands retain
their separate prerequisites. Full final verification and its zero-retry/completion rules remain
required.

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
