# Reproduced browser-test issues

Add a case only after reproducing it. Record the affected scenario, observable
failure, actual cause, smallest repair and exact passing verification. Distinguish
test infrastructure from a product bug; preserve the expected user behavior.

## Empty-list search could pass before the API request

The initial Products smoke changed empty-state text immediately while its search
request waited for a 300 ms debounce. Clearing early canceled the request.
The smoke now matches `POST /v1/products/search` and its body, requires a
successful response, then verifies the returned product or actual empty result.
Example: `e2e/smoke/login.e2e.ts`; verified by the two-test real dev smoke.

## Local frontend cannot directly use the shared dev API

The dev backend does not allow the test frontend's origin through CORS. The
read-only smoke preview forwards real `/health` and `/v1` requests without
changing app production configuration. Full feature coverage uses a disposable
local backend with the exact frontend origin. Keep those two targets separate.

## Vitest consumed Playwright spec files

Anchor's ordinary unit runner discovers `.spec.ts`. Browser scenarios use
`.e2e.ts`, and each Playwright config explicitly selects its owned test folders.
Keep discovery checks in validation when moving or adding suites.

## Changing stack layers or running inside an agent sandbox

Switching to a layer with a different lockfile made pnpm 11 attempt an automatic
dependency reinstall; without a terminal it aborted module replacement. Install
that layer explicitly with `CI=true pnpm install --frozen-lockfile` before using
its package scripts. During the story relocation, the installed Vitest,
Storybook and Playwright versions were verified identical in both layers, so a
focused check could use the installed Vitest CLI directly. Full PR CI still
installs from each layer's frozen lockfile.

A browser runner that stops before collecting tests with `listen EPERM` needs
authorized local-server access from its agent sandbox. This was an execution
boundary; rerunning with that access collected and passed the story. Do not
treat zero collected tests as application verification.

## UI mode omits project dependencies

The bootstrap project is visible in headless reports. Playwright UI mode does
not automatically run it. The UI launcher runs its idempotent browser test
before opening the UI. Worker auth fixtures also initialize the owned local
platform through the public registration API when it is still empty, allowing
a selected feature to run alone. Full cold headless runs verify bootstrap UI.

## UI showed No tests after its preview became ready

The UI's output reported that `http://127.0.0.1:3015/__e2e/ready` was already
used. CLI discovery still listed all 56 scenarios. Playwright UI runs global
setup before collecting tests, and Reload reconnects and repeats that setup;
the occupied ready URL made setup fail before collection. Reloading could
therefore repeat the same failure even while the backend was healthy.

Use `pnpm test:e2e:ui`, whose launcher verifies or starts the owned preview
first, then gives Playwright `E2E_REUSE_SERVER=1`. It checks the live worktree
backend, readiness run ID, API URL and current frontend fingerprint before
running the bootstrap browser test and opening the UI. It refuses foreign or
stale previews and an occupied UI port. Ctrl-C or SIGTERM stops its Playwright
process and only the preview it started; an explicitly prestarted preview stays running.
If an owned backend is running without a ready preview, start its managed
preview explicitly or stop that backend first; the launcher refuses to take
over its lifecycle.
Never stop an unrelated server just to free either loopback port.

To distinguish discovery from setup, run this without launching browsers:

```sh
pnpm exec playwright test --config playwright.app.config.ts --list
```

If discovery succeeds but the UI stays empty, check Output for setup errors
and clear text, project or "Show only changed files" filters. The changed-files
filter can correctly produce zero cases after all e2e changes are committed.
Validate the launcher with a fresh UI start, a Reload, a selected test and
Ctrl-C followed by `node scripts/e2e-runtime.mjs verify-stopped`. When reusing
an explicitly started preview, stop that preview before checking teardown.

## Domain and component incidents

- [Component tests](troubleshooting/components.md)
- [Access control and tenancy](troubleshooting/access.md)
- [Licensing](troubleshooting/licensing.md)
- [Email and integrations](troubleshooting/integrations.md)

## Registration returned success but routed back to initialization

The bootstrap and invitation signup scenarios received a real success response,
but the cached health query still said the tenant was uninitialized. Signup now
updates that health query and synchronously commits auth context before router
navigation. Login uses the same auth-before-navigation order so an internal
redirect remains intact. Verify `auth/bootstrap.e2e.ts`, `auth/session.e2e.ts` and
`platform/administration.e2e.ts` against a fresh managed runtime.

## Cancel retained a product creation draft

`ProductCreateDialog` reset its values in `handleOpenChange(false)`, but its Cancel
button bypassed that function and called the parent callback directly. The
product lifecycle scenario cancels, reopens and requires an empty name. Cancel
now goes through the same reset path as closing the dialog.

## Cleanup used the JWT subject instead of a platform user ID

The authentication identity and public platform user have different IDs. Calling
platform-user DELETE with the token subject failed cleanup. Fixtures now get the
actual user via `GET /v1/me`, delete that ID, and require 204. Product resources
are deleted by their created IDs after unprotecting a test-owned product.

## Reusing a stale server after an app edit

Workers compare the preview's source fingerprint and disposable backend run ID
before login. A run deliberately caught an editor change made during execution;
replaced workers rejected the old build rather than testing the wrong source.
Finish edits, stop the owned preview, rebuild, then rerun. Test/guide-only changes
do not invalidate the app build. Backend caching fingerprints the source and
Go toolchain separately.

## Preview connections delayed shutdown

An open preview websocket could keep HTTP close waiting and leave disposable
services running. Shutdown now closes Vite through its public API and closes
HTTP connections before stopping its owned backend/Compose services. Headless
Playwright allows 30 seconds for that cleanup. Signal handlers register before
runtime startup; an abort during setup also follows the owned cleanup path.
Use `e2e-runtime.mjs verify-stopped` after a managed cold run to verify teardown.

## Selecting two UI tests raced first-owner registration

UI mode omits setup dependencies. Two workers could both observe an empty
platform, then try to initialize it. The worker fixture now claims an atomic
run-local directory lock, rechecks health before registering, and the other
worker polls the initialized health outcome. Both log in with that runtime's
owner afterward. A fresh managed runtime passed two feature scenarios with
`--project=chromium --no-deps` and two workers.

## Setup failure prevented resource cleanup

Worker contexts/accounts and test products now clean up in `finally` blocks.
A failed lazy product-key request cannot prevent product deletion; browser
login failure cannot bypass disposal of already-created request contexts.
Failure assertions remain visible and the whole disposable runtime is still
removed by managed shutdown.

## API keys without grants crashed the table

The API mapped an empty permission slice to JSON `null`, while the contract
requires an array and the table calls `map`. The list regression intentionally
creates zero-grant keys, requires `permissions: []` and exercises pagination and
facets. The response mapper now preserves an empty array for creation and reads.

## Backend cache missed an embedded contract change

The Go binary fingerprint covered Go, SQL and module files but omitted the
embedded `cmd/http/openapi.yaml`. YAML is now included along with the toolchain,
so a contract-only edit cannot reuse an older validator binary. If a new Go
embed is added, update the fingerprint's input list in the same change.

## Missing details waited through retries of a definitive 404

The role, resource-permission and product API-key missing-detail scenarios
passed, but spent up to 15.6 seconds retrying an unchanged missing resource.
Their queries now stop retrying the specific public `*_NOT_FOUND` error code;
other errors retain the existing three-retry budget. A generic ban on retries
would change transient-failure behavior unnecessarily.

Keep the visible recovery action and an assertion of exactly one detail GET.
The rebuilt runtime passed both missing-detail scenarios in about one second
each with those assertions. Compare the complete scenario set before changing
worker count or shared retry behavior.

## CI cannot resolve runner context in job-level env

`actionlint` rejected `GOCACHE: ${{ runner.temp }}/anchor-e2e-go-build` in the
job-level environment. Resolve the cache in the test step with
`GOCACHE="$(go env GOCACHE)"` instead. This uses the build cache restored by
`actions/setup-go`; a new cache under `runner.temp` would compile dependencies
again. Validate workflow changes with `actionlint`, and allow pull requests
targeting stack branches so the final stack layer receives the browser check
before merging.

## Storybook scanned a bulk dialog during teardown

See [the dialog teardown recipe](troubleshooting/components.md#storybook-scanned-a-bulk-dialog-during-teardown).

## Storybook did not observe a schema parse error after untargeted paste

See [the targeted-input recipe](troubleshooting/components.md#storybook-did-not-observe-a-schema-parse-error-after-untargeted-paste).

## Managed tests passed but left the backend running

The first fresh managed run passed all 56 cases, then exited while its owned
API, three containers and `runtime.json` remained. An explicit SIGTERM startup
interruption had already cleaned up correctly. The missing piece was the
Playwright web-server configuration: its
[default shutdown force-kills the process group](https://playwright.dev/docs/test-webserver#configuring-a-web-server),
so the preview's shutdown handler never ran. The detached API and Docker
services are outside that process group.

Configure `gracefulShutdown: { signal: "SIGTERM", timeout: 30_000 }` so the
handler can stop the owned API, take down its Compose volumes and remove run
metadata. The first graceful rerun still left resources: Vite preview installs
its own SIGTERM handler, closes HTTP and calls `process.exit()` before the
asynchronous backend cleanup completes. Close the preview through its public
`server.close()` API at the start of our handler, before awaiting backend work.
This also unregisters Vite's signal callback. Closing only `httpServer` after
backend cleanup does not release that handler. The upper bound still
force-kills a stuck preview. CI runs the
ownership-scoped cleanup verification even when tests fail:

```sh
node scripts/e2e-runtime.mjs verify-stopped
```

That check fails on leftover runtime/startup metadata, a startup lock or Docker
containers whose project prefix belongs to this worktree. It detected the
original leftover metadata before cleanup. To recover, use the existing owned
`stop` command, then repeat the full managed run and cleanup verification. It
does not stop another worktree's services. Benchmark timings above excluded
teardown; this configuration repair does not change their scenario assertions.

## Selective runner cancellation

Playwright 1.60 watches SIGINT for cancellation and web-server cleanup. Forwarding
SIGTERM directly from the new wrapper would terminate the CLI before that path
ran. The wrapper now converts both signals to SIGINT and waits for the child to
close. A real SIGTERM sent after owned startup metadata appeared exited 130;
`node scripts/e2e-runtime.mjs verify-stopped` confirmed no metadata, startup lock
or owned containers remained. Evidence: ignored `.ui-craft/affected-interrupted.log`.
Do not change this to immediate `process.exit()` or forward SIGTERM to the CLI.
