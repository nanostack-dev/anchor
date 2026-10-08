# Anchor browser test guide

Read this before adding, repairing or tuning browser tests. The suite exercises
Anchor's real browser and local API. Feature coverage and runnable tests are the
completion criteria, not the number of test files.

Showing tests: follow the [UI handoff](../README.md). Use `pnpm test:e2e:status`
for read-only worktree, runtime and inventory diagnostics. Reviewing changes:
use the [review criteria](review.md). Inspecting CI failures: use
[CI failure evidence](ci-failures.md).
Before implementing a browser-visible feature or behavior change, load
[Anchor feature review](../../../.claude/skills/anchor-feature-review/SKILL.md).
It links the shared Nanostack procedure and maps it to this project's commands,
fixtures and evidence; keep verified Anchor troubleshooting in this guide.

1. Identify the shipped route and visible action in [coverage.md](coverage.md).
   Read that UI source and its public API contract. Preserve read-only and
   placeholder boundaries; API fixture creation does not cover a creation UI.
2. Add one independent scenario in the owning `e2e/features/<domain>/` folder.
   Import `test` and `expect` from `support/fixtures`; use `world` for a new
   product, a worker account, authenticated API prerequisites and scoped cleanup.
   `support/ui` contains the shared real login and product selection steps.
3. Drive the behavior under test through accessible roles, labels and visible
   text. Assert the meaningful result, including persistence when appropriate.
   Prepare unrelated data via supported API calls. Keep assertions visible in
   the scenario and helpers specific to repeated user actions.
4. Run the narrow test first. Diagnose failures using the relevant UI/API output.
   [Troubleshooting](troubleshooting.md) records reproduced causes and verified
   repairs. Product failures keep a failing assertion until fixed or explicitly
   reported; skipped/optional assertions never count as verified coverage.
5. Run the affected local selection, including new or modified scenarios and
   existing regressions; broaden it when shared dependencies warrant it. Update
   the coverage matrix and require the complete CI gate before review readiness.
   For performance tuning, compare repeated timings using
   [performance.md](performance.md) with an identical scenario set. New tests own
   resources even under shuffled/concurrent runs. Follow the shared
   [agent workflow](../../../AGENTS.md#agent-workflow) for the documented cloud
   exception when local runtime access is unavailable.
6. Update this guide when a convention changes; record new failure recipes with
   the failing scenario, cause, repair and verification command. Use one source
   of truth for operational commands in `package.json` and configuration in the
   Playwright config. Add links here instead of growing one large manual.

## Suite boundaries

- `test:e2e:app`: disposable local TimescaleDB, Redis, Mailpit and real Anchor;
  generated local accounts, isolated product per test, worker auth reused only
  for unrelated features. It covers browser features including mutations.
- `test:e2e:smoke`: existing read-only deployed-dev login smoke, using your local
  dev account. Its credentials and recording policy remain separate.
- `test:e2e`: previous local bulk-delete regression. Storybook/unit/backend
  suites retain their distinct component and contract responsibilities.
- `test:e2e:ui`: loopback Playwright UI for selection, inspection and reruns.
  Worker fixtures bootstrap lazily when UI mode omits dependency projects.

The local suite boots one backend and builds one frontend per run. Fresh browser
contexts isolate cookies and UI state; unique fixture IDs isolate server state.
Keep external requests out of ordinary local tests. Mailpit verifies SMTP
delivery; Clerk webhook processing uses a local product integration and signed
webhook fixtures rather than a live vendor account.

The [research](research.md) records ten articles and primary-doc checks behind
these decisions. [Coverage](coverage.md), [performance](performance.md) and
[troubleshooting](troubleshooting.md) carry evidence and maintenance work.
The [selective testing guide](selective-testing.md) describes changed-file
selection, dependency-map maintenance and the complete CI gate. Its
[research](selective-testing-research.md) records the sources and tradeoffs.

## Headless frontend port

App, responsive review and complete verification runs accept `E2E_FRONTEND_PORT`
when another worktree owns the default port 3015. Use a whole port from 1 through
65535; invalid values fail before startup. For example:

```sh
E2E_FRONTEND_PORT=13015 ANCHOR_E2E_RUNTIME_NAMESPACE=fraud-verification pnpm test:e2e:verify --base origin/main
```

The readiness URL, browser origin, build API origin and owned runtime frontend
origin use the same port. A listener already on the selected port is still
refused. The override applies to headless app/review/complete runs. Set it on that
command only; Playwright UI, doctor and benchmark commands require port 3015
and do not support the override. The runtime namespace isolates
managed metadata and services; it does not reserve or change a port itself.

## Parallel agent ownership

Assign independent domains: `auth/platform`, `access`, `licensing`, and
`integrations/email`. Each agent owns its domain's specs/helpers and linked
troubleshooting file. One coordinator owns shared fixtures/runtime/config,
`coverage.md`, the generated inventory and this index. Agree on ownership before
editing a shared helper; report a required shared change to its owner.

API and browser fixtures own unique resources even when agents share the local
runtime. Only the coordinator starts, rebuilds or stops that runtime. Finish all
source/test edits and close other test runners before benchmarking; the benchmark
rejects changed scenarios/helpers/configuration. Integration of a domain requires
its passing affected local selection, the complete CI gate and the updated
coverage/troubleshooting evidence. A route annotation alone is not proof that
its controls were exercised.
