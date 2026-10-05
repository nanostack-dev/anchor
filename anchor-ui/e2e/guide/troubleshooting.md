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

## UI mode omits project dependencies

The bootstrap project is visible in headless reports. Playwright UI mode does
not automatically run it. Worker auth fixtures initialize the owned local
platform through the public registration API when it is still empty, allowing
a selected feature to run alone. Full cold headless runs verify bootstrap UI.

## Domain incidents

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
services running. Shutdown now stops its owned backend/Compose services before
closing HTTP connections, including an explicit connection close and bounded
fallback. Signal handlers register before runtime startup; an abort during setup
also follows the owned cleanup path. Use `e2e-runtime.mjs status` after a managed
cold run to verify teardown.

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
