# Anchor browser test guide

Read this before adding, repairing or tuning browser tests. The suite exercises
Anchor's real browser and local API. Feature coverage and runnable tests are the
completion criteria, not the number of test files.

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
5. Run the full suite, update the coverage matrix, then compare repeated timings
   using [performance.md](performance.md). New tests own resources even under
   shuffled/concurrent runs. Keep the scenario set identical during tuning.
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
its passing narrow run, the whole suite and the updated coverage/troubleshooting
evidence. A route annotation alone is not proof that its controls were exercised.
