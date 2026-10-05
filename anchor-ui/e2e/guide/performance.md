# Browser suite performance

Measure the same feature set with one, two and four workers. Keep retries off
while diagnosing; report flaky retries separately from passing cases.

## Repeatable comparison

From `anchor-ui`, start `node scripts/serve-e2e-full.mjs` in one terminal, wait
for its ready message, then run this in another:

```sh
pnpm benchmark:e2e
# Optional smaller comparison after a focused change:
pnpm benchmark:e2e --workers=2,4 --runs=3
```

The default command runs three complete suites at each worker count, in order,
against that owned preview. It verifies identical scenario identities and zero
failed, flaky or skipped cases. Reports include the commit, frontend fingerprint,
scenario/helper fingerprint, toolchain/machine, startup/build time, suite runner/wall times, p95 scenario time
and five slowest cases. Raw logs and JSON stay under ignored
`.ui-craft/performance/<timestamp>/`; copy the summarized evidence here.

Finish app, browser-scenario, helper and configuration edits and close other test
browsers before comparing. Rebuild
the preview after an app edit. Run a fresh managed suite after the comparison to
verify initialization and teardown too; the repeated warm runs do not replace
that check. Stop the manually owned preview with Ctrl-C when done.

CI uses the Go build-cache path restored by
[`actions/setup-go`](https://github.com/actions/setup-go/blob/v6.4.0/src/package-managers.ts)
and the pnpm dependency cache. The runtime caches its backend binary by sources,
embedded YAML and Go toolchain locally; a fresh database still starts each
managed run. Install Chromium only in this job. These optimizations preserve
every browser scenario; they avoid recompiling or downloading unrelated work.

## Keeping coverage while reducing work

Record fresh backend/build startup separately from browser execution. Compare
three repeated runs at each chosen worker count on the same machine/backend.
Preserve test count and assertions. Keep wall time, setup time, test durations,
slowest scenarios and failures with the environment/commit that produced them.

Expensive prerequisites belong to the runtime or a worker fixture. Mutable
resources belong to a test. Use API fixtures for prerequisites and retain
separate browser scenarios for creation, editing and deletion actions. Reuse
authentication only where shared account state cannot invalidate another test.

Actions/assertions wait for specific outcomes. Timeouts are bounded failure
budgets; fixed sleeps and whole-page network-idle waits add cost and uncertainty.
Video is off. Normal headless tracing is off; `E2E_TRACE=1` enables local
failure diagnostics and records every test, so exclude it from comparisons.

Use Chromium for the broad local suite. Cross-browser runs add a browser matrix
to the same scenarios rather than replacing feature coverage. Add sharding only
after measured suite latency outweighs duplicated startup cost.

## Measured baseline — 2026-10-05

The complete 56-scenario suite passed nine consecutive runs: three at each
worker count, zero retries, failures, flaky results or skips (504 executions).
Scenario identities and the bytes of the tests, helpers and configuration stayed
identical throughout. These are browser-run timings against one already started
preview; they exclude dependency installation, runtime startup and teardown.

| Workers | Runner time, three runs | Median runner | Median process wall | Scenario p95, three runs |
| --- | --- | --- | --- | --- |
| 1 | 133.63 / 134.47 / 132.56 s | 133.63 s | 134.27 s | 7.07 / 7.11 / 7.13 s |
| 2 | 69.95 / 70.46 / 70.46 s | 70.46 s | 71.11 s | 7.08 / 7.06 / 6.97 s |
| 4 | 41.20 / 41.13 / 41.13 s | 41.13 s | 41.86 s | 7.29 / 7.20 / 7.23 s |

Four workers reduced median runner time by 69.2% versus one worker and 41.6%
versus two. The local default is capped at four; use fewer on smaller machines.
CI keeps two workers because its runner has a different CPU and startup budget.
More local workers have not been measured. The slightly higher scenario p95 at
four workers is included above rather than hidden by the faster overall suite.

Environment: Apple M2 Max, 12 logical CPUs, 32 GiB RAM, macOS arm64; benchmark
runner and frontend build Node 24.13.0. Commit `69afbcdd1a9a1354f3dc6537317ce8353a4496e7`;
frontend fingerprint `578f2c9dc411301bf55d311a3f6e8598a3886db86fff1e176f69d59fad53a856`;
scenario/helper fingerprint `dbf93f0fcefd287dacded488d676fc354d3a0e93da710e8835e35ff545222aa5`.
Evidence: ignored `.ui-craft/performance/2026-10-05T22-33-07.637Z/summary.json`
and the nine individual JSON reports/logs.
Component-story synchronization edits during this series were outside the
production frontend and browser-input fingerprints; both remained unchanged.

That preview used a new database, pre-pulled container images and a cached Go
binary. Startup was 38.47 s: backend 20.16 s and frontend build 17.87 s, plus
preview setup. The separate managed-run validation also uses Node 24 throughout.
These numbers are not a cache-empty installation benchmark.

The five slowest cases in the median four-worker run were product lifecycle
(8.87 s), role lifecycle (7.23 s), event subscriptions and signed delivery
(7.20 s), template validation and persistent editing (7.19 s), and API-key
wizard/editing (6.32 s). They retain their persistence, cancellation and negative assertions.
Optimize their prerequisites or waits only with an unchanged assertion set and
a repeated comparison.

Earlier verification exposed a schema-menu pointer interception, a notification
covering the Events Discard action, and incomplete managed teardown. The
[licensing](troubleshooting/licensing.md),
[integration](troubleshooting/integrations.md) and
[cleanup](troubleshooting.md#managed-tests-passed-but-left-the-backend-running)
recipes record the observed failures and repairs. Those attempts and the earlier
completed comparison are excluded from this final table. The entire nine-run
comparison was restarted with the final browser inputs; no failed run was
discarded within the reported series. Stopping that preview also passed the
owned-runtime cleanup guard.

## Fresh managed run and CI

After stopping the benchmark preview, a managed run under Node 24.13.0 built the
frontend, started a fresh database and passed all 56 scenarios with the default
four workers, zero retries, failures, flakes or skips. Process wall time was
72.89 s. Startup was 29.04 s (backend 16.96 s, frontend build 11.77 s); container
images and the Go binary were already cached. Automatic teardown passed the
owned-runtime guard: no runtime/startup metadata, startup lock or containers;
the API and preview were stopped. This final check includes the
[cleanup repair](troubleshooting.md#managed-tests-passed-but-left-the-backend-running).
Evidence: ignored `.ui-craft/final-managed-node24.log` and
`.ui-craft/final-managed-node24-results.json`.

An additional interruption check sent SIGTERM after pending startup metadata
appeared and before readiness. The process exited successfully and left no
runtime/startup metadata, startup lock or containers in that owned Compose
project. Its log stays in `.ui-craft/startup-abort.log`.

The top stack's [GitHub E2E run](https://github.com/nanostack-dev/anchor/actions/runs/37383789740)
also passed all 56 scenarios with two workers and no retries, failures, flakes
or skips, and its post-run cleanup guard passed. The JSON report duration was
167.49 s, including managed startup; the complete job took 4 min, including
checkout and dependency setup.
Do not compare that total directly with the warm local runner table above.
