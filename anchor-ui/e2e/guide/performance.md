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

Finish source/test edits and close other test browsers before comparing. Rebuild
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

Record cold backend/build startup separately from browser execution. Compare
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

Measurement results will be recorded here once the complete scenario set passes.
