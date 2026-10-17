# Selecting browser suites from changed files

Use these commands from `anchor-ui` with Node 24 and Docker running:

```sh
pnpm test:e2e:plan --base origin/main       # JSON explanation; no browser or server
pnpm test:e2e:affected --base origin/main   # focused feedback
pnpm test:e2e:verify --base origin/main     # complete coverage, affected suites first
pnpm test:e2e:verify --base origin/main --list
pnpm test:e2e:selection                    # real Git fixtures and report-gate tests
```

The base defaults to `E2E_BASE_REF`, then `origin/main`. The comparison uses the
real merge-base with HEAD and includes staged, unstaged and untracked files.
`--head <ref>` compares another committed head while still including local
changes. The planner alone supports `--committed-only`; normal test commands
always include local edits. Fetch the base before comparing branches. CI checks
out complete history and supplies the pull request's actual base SHA, including
the previous branch when testing a stack layer.

## Policy

[`../impact-manifest.json`](../impact-manifest.json) is the reviewed dependency
map. It maps whole feature spec files to auth, platform, access, licensing and
integrations. Source rules include consumers, not just the directory that owns
a change: backend licensing also runs tenancy and events; RBAC backend changes
also run license, integration and key consumers. Multiple matching rules and
multiple changed files form a union. Every executable focused run includes real
authentication and the product lifecycle/dashboard journeys.

Shared authentication, navigation, API clients/contracts, database primitives,
dependencies, runtime and selector changes run every current spec. Unknown
executable paths, Git errors, missing refs and a new or removed spec also run
everything. Renames examine both old and new paths; deleted source still selects
its consumers. Documentation-only or unchanged diffs need no focused run.
`test:e2e:verify` still runs the entire inventory for these diffs.

This map expresses reviewed dependencies; it is not an automatic proof that a
browser never reaches another feature. Missing knowledge expands coverage. Do
not use Playwright `--only-changed` as the app gate: its runner import graph
does not trace the separate Go service or the served frontend. The
[research note](selective-testing-research.md) explains the alternatives.

## One runtime and a complete gate

For a narrow diff, one Playwright invocation executes this project dependency
chain:

```text
bootstrap -> chromium-affected -> chromium-remaining
            parallel scenarios   parallel scenarios
```

The focused command omits the remaining project. The complete command includes
it. A full fallback uses the ordinary app configuration. Projects share one
fresh backend/database and one frontend build; a scenario runs once, with no
duplicate bootstrap. Failure in the affected project fails the invocation and
prevents remaining tests from running. Fix the failure and repeat the complete
command. Workers remain parallel within each phase; phases can slightly extend
total duration, so this is a feedback-order optimization rather than a claim
that a complete run does less work.

Before starting, the complete command lists the ordinary full configuration as
the independent scenario inventory. After success it compares every file and
full nested test title against the JSON result. Missing, unexpected or duplicated
scenarios, skipped tests, global errors, expected failures, flaky results and
retries fail the gate. Counts are discovered rather than hard-coded.
Filtering flags (`--grep`, `--project`, `--shard`, `--last-failed`) are unsupported
by these commands. An interrupted run fails. The runtime refuses to reuse or
stop an existing owner's environment, including a concurrent startup detected
under its startup lock. Close the Playwright UI or manually owned preview first.

Reports stay in ignored `test-results/`: `selection.json` explains the selection,
`app/results.json` contains browser results and `completion.json` proves a full
successful run. A new execution removes an older completion proof. Listing
tests preserves browser result files. CI uploads these files and failure images,
and checks owned runtime cleanup even after failure. A focused green run alone
does not establish complete coverage.

## Maintaining the map

1. When adding/removing a spec, update the owning domain's inventory and
   [coverage matrix](coverage.md). Bootstrap and critical product journeys stay
   in `always`. Inventory drift already falls back to all current specs.
2. Before narrowing a source path, inspect its public handlers, callers,
   subscriptions and UI/API consumers. Record those source paths in `provenance`
   and explain the union in `reason`. Exact filenames and `directory/**` are
   supported; arbitrary glob syntax is rejected.
3. Add a real Git fixture case to `scripts/e2e-impact.node-test.mjs` for a dependency
   rule that introduces a different behavior. Keep shared infrastructure broad.
   Test the public plan/CLI rather than private matching helpers.
4. Run selection checks, the focused command and the complete gate. Record a
   reproduced issue and verified repair in [troubleshooting](troubleshooting.md).
   The coordinator owns map/config/runtime changes during parallel agent work.
5. Keep the full CI gate. Consider reducing CI work only with measured dependency
   evidence and an explicit coverage policy change; do not silently turn this
   required complete job into the focused command.

## Verified behavior — October 5, 2026

The public planner/report gate passed 25 Node tests with real Git fixtures. A
reversible single email-page whitespace edit exercised actual local selection:
bootstrap passed once, then all 15 selected scenarios, then all 40 remaining
scenarios. The complete report gate verified the ordinary 56-scenario inventory,
zero skips, retries or flakes; owned-runtime cleanup passed. The edited page was
restored byte-for-byte. Wall time was 79.99 s under Node 24.13.0 with four workers,
cached container images and Go binary. This one run verifies behavior; it is not
a repeated performance comparison against the baseline.

Public CLI checks rejected `--grep`, `--project` and `--shard` before startup.
Complete mode also rejected a temporarily introduced `test.only` while listing
the baseline. An unchanged focused diff started no browser. The new runtime
ownership guard refused existing metadata; the wrapper's SIGTERM interruption
exited 130 and passed the owned-runtime cleanup guard.
Evidence stays in ignored `.ui-craft/affected-complete-{report,selection,proof}.json`,
`.ui-craft/affected-complete.log` and `.ui-craft/affected-interrupted.log`.
