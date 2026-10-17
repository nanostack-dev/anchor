# Playwright research: maintainable, fast browser coverage

Reviewed on **2026-10-05**. This is the evidence behind the Anchor browser-test
guide. The ten articles below are authored technical articles, rather than search
snippets. Official Playwright documentation verifies the framework behavior;
article recommendations and this project's decisions are distinguished below.
The operational commands and environment rules live in [the browser-test
README](../README.md).

The installed Playwright version was **1.60.0** when this research started.
The online documentation evolves independently. Before using a newly documented
API, check the installed version and its release notes; this research does not
require upgrading Playwright.

## Ten articles and what we take from them

1. **[How low-level API calls can stabilize your end-to-end tests](https://www.checklyhq.com/blog/how-low-level-api-calls-stabilize-your-end-to-end-tests/)**
   — Nico Domino, Checkly; published June 16, 2022, updated April 25, 2025.

   Checkly describes splitting resource creation and deletion into independent UI
   tests, with API calls establishing prerequisites and cleaning up afterwards.
   Adopt the separation: a delete test should create its own resource through the
   API, while a create test must still create through the browser. This reduces
   repeated navigation and cascading failures without removing either UI
   behavior from coverage. Use Playwright's native request context rather than
   copying the article's older Axios/browser-launch infrastructure.

2. **[Why you shouldn't run tests sequentially](https://www.checklyhq.com/blog/playwright-tests-in-sequence/)**
   — Nočnica Mellifera, Checkly; published March 4, 2025, updated March 31, 2025.

   Hidden dependencies between tests impede parallel execution and make failures
   harder to interpret. Shared fixture code can remove duplication without
   requiring another test to run first. Adopt independent scenarios with their
   own mutable resources. The article discusses Checkly's distributed monitoring
   environment; its scheduling examples are not runtime guarantees for this
   local suite. Explicit setup-project dependencies remain useful and differ
   from ordering ordinary feature tests.

3. **[Why you should never use page.waitForTimeout() in Playwright](https://www.checklyhq.com/blog/never-use-page-waitfortimeout/)**
   — Nočnica Mellifera, Checkly; published March 5, 2025, updated March 31, 2025.

   Fixed delays waste time when the application is ready early and still fail
   when it is ready late. Use assertions that wait for the actual result instead.
   Adopt a ban on fixed sleeps in committed tests. If a control needs more time,
   identify the observable readiness condition first. Retrying an assertion is
   different from rerunning the entire test: do not add suite retries as a
   substitute for correct synchronization.

4. **[Track Frontend JavaScript exceptions with Playwright fixtures](https://www.checklyhq.com/blog/track-frontend-javascript-exceptions-with-playwright/)**
   — Stefan Judis, Checkly; published November 20, 2023, updated July 23, 2025.

   A shared page fixture can listen for uncaught JavaScript exceptions before
   navigation and assert afterwards. This catches failures that an otherwise
   successful journey might miss, with little additional browser work. Adopt
   uncaught-error collection as a supporting signal. Scope exclusions to a
   specific intentional failure and explain them; blanket suppression weakens
   coverage. Expected HTTP failures in negative tests must not be confused with
   uncaught application exceptions.

5. **[Optimizing Test Runtime: Playwright Sharding vs. Workers](https://currents.dev/posts/optimizing-test-runtime-playwright-sharding-vs-workers)**
   — Joshua Adeyemi, Currents; published December 9, 2025.

   Workers compete for the resources of one machine; sharding adds machines,
   startup cost and report coordination. Adopt measurement before increasing
   concurrency. Its suggested suite sizes and speedups are illustrative, not
   Anchor targets, and no commercial orchestrator is needed for this suite.
   Correction from official documentation: sharding can distribute individual
   tests with `fullyParallel`, so it is not always a simple alphabetical split
   of entire files. Separate CPU limits from shared-backend contention.

6. **[9 Playwright Best Practices and Pitfalls to Avoid](https://betterstack.com/community/guides/testing/playwright-best-practices/)**
   — Ayooluwa Isaiah, Better Stack Community; updated January 14, 2026.

   Plan coverage around user workflows, use meaningful test/step names, isolate
   cases and choose relevant browsers. Adopt a feature-to-scenario inventory
   before declaring coverage complete. Treat this as a vendor-authored tutorial;
   framework details are verified below. The article's idealized sharding
   speedups depend on balanced work and available capacity. Its advice to focus
   on critical flows does not justify omitting Anchor features requested for
   this milestone.

7. **[A better global setup in Playwright reusing login with project dependencies](https://dev.to/playwright/a-better-global-setup-in-playwright-reusing-login-with-project-dependencies-14)**
   — Debbie O'Brien, Playwright's technical blog on DEV; published March 15, 2023.

   Setup projects expose authentication setup in reports and traces while
   allowing fixtures. Saved authentication avoids logging in for every unrelated
   feature. Adopt that principle where account state can be safely shared, or
   use worker-specific auth fixtures for mutable state. Keep dedicated tests for
   UI login, logout and guest protection. Do not copy published demo credentials
   or assume that reusing authentication also isolates backend resources.

8. **[Setup a local dev server for your Playwright tests](https://dev.to/playwright/setup-a-local-dev-server-for-your-playwright-tests-33m9)**
   — Debbie O'Brien, Playwright's technical blog on DEV; published March 7, 2023.

   `webServer` starts the application and waits for an HTTP readiness endpoint;
   `baseURL` centralizes the frontend origin. Adopt one managed local server for
   the suite, rather than building or launching it in every test. Reusing a
   developer server is an optional iteration optimization: it needs an explicit
   choice, because a process on the right port may serve another worktree or
   stale code. CI should start the selected build cleanly.

9. **[Playwright's UI Mode - watch mode and time travel debugging](https://dev.to/playwright/playwrights-ui-mode-watch-mode-and-time-travel-debugging-10g5)**
   — Debbie O'Brien, Playwright's technical blog on DEV; published May 5, 2023,
   edited May 10, 2023.

   UI mode helps inspect actions, DOM snapshots and failures, run selected tests
   and watch files during development. Adopt it as the interactive demonstration
   and debugging surface, with headless runs providing repeatable timings.
   Current official documentation adds an important setup limitation: UI mode
   does not automatically run dependency projects. Run required setup manually,
   or design lazy worker fixtures so selected feature tests remain runnable.

10. **[How I Used AI to Fix Our E2E Test Architecture](https://debbie.codes/blog/how-i-used-ai-to-fix-our-e2e-test-architecture/)**
    — Debbie O'Brien, Debbie Codes; published April 29, 2026.

    This firsthand migration separates expensive worker setup from test-owned
    mutable data, proves one small slice before scaling, and records repeated
    before/after runs. Adopt the measurement and fixture ownership approach. The
    reported cleanup project interfered with other CI pipelines: cleanup must
    target resources created by its own run. Maintain explicit file ownership
    for parallel agents and preserve failure assertions when repairing tests.
    The author's performance numbers are not predictions for Anchor.

## Framework facts verified against official documentation

| Topic | Verified behavior and implication | Primary source |
| --- | --- | --- |
| User-visible checks | Test behavior users observe; isolated tests and web-first assertions reduce coupling and timing races. Keep frontend/backend integration real for the journey under test. | [Best practices](https://playwright.dev/docs/best-practices) |
| Locators | Prefer accessible roles and labels. Single-element actions fail on ambiguous matches. Refine by a dialog or a uniquely named row rather than using `first()` to silence ambiguity. | [Locators](https://playwright.dev/docs/locators) |
| Waiting | Actions perform actionability checks; web assertions retry until their condition is satisfied. A visible control alone does not prove the requested business operation succeeded. | [Auto-waiting](https://playwright.dev/docs/actionability) |
| Network readiness | `networkidle` is discouraged for tests. Assert the relevant UI outcome instead of waiting for the whole application to stop networking. | [Page load-state API](https://playwright.dev/docs/api/class-page#page-wait-for-load-state) |
| Authentication | Shared saved state is appropriate for tests that do not affect each other's server state. Mutating tests should use separate accounts per parallel worker. Auth files contain impersonation material and stay gitignored. API login is supported; maintain explicit UI login tests. | [Authentication](https://playwright.dev/docs/auth) |
| Fixture scope | Worker fixtures run once per worker process; test fixtures run per test. Unused nonautomatic fixtures are lazy. Dependencies set up before consumers and tear down afterwards. Share immutable prerequisites, keep mutable resources test-owned. | [Fixtures](https://playwright.dev/docs/test-fixtures) |
| API prerequisites | Request contexts can prepare backend state and verify postconditions. Browser-associated request contexts share cookies; standalone contexts have separate cookies and need disposal. | [API testing](https://playwright.dev/docs/api-testing) |
| Setup visibility | Project dependencies are the recommended global-setup approach for reports, traces and fixtures. Ordinary feature tests must still be independently executable. | [Global setup and teardown](https://playwright.dev/docs/test-global-setup-teardown) |
| Parallelism | Files run in parallel by default; tests within a file are sequential unless configured otherwise. Failed workers are replaced, so worker setup must tolerate restarts. | [Parallelism](https://playwright.dev/docs/test-parallel) |
| Sharding | `fullyParallel` enables distribution at test granularity; otherwise sharding operates on files. Blob reports can be merged. Test-count balance is not a guarantee of duration balance. | [Sharding](https://playwright.dev/docs/test-sharding) |
| Traces | `on-first-retry` records retry attempts; `retain-on-failure` records every test and discards successful traces. The latter still has recording overhead. Full-time traces are performance-heavy. | [Trace viewer](https://playwright.dev/docs/trace-viewer) |
| Retries | A pass after a failed attempt is classified as flaky. Serial groups skip later cases after a failure and retry the group together. Prefer independent tests; report flakes, even when the run exits successfully. | [Retries](https://playwright.dev/docs/test-retries) |
| Timeouts | Test timeout includes fixture setup and `beforeEach`; teardown has a separate allowance. Expect, action, navigation and whole-run timeouts are different controls. Give expensive fixtures their own budget instead of making every test slow to fail. | [Timeouts](https://playwright.dev/docs/test-timeouts) |
| Application server | `webServer` can manage multiple processes. Readiness is checked at the configured URL. Reuse requires knowing which build is already running. | [Web server](https://playwright.dev/docs/test-webserver) |
| Interactive mode | UI mode supports filtering and inspection, but dependency setup must be run explicitly. Bind the demonstration server to loopback: traces and secrets become network-accessible when bound to all interfaces. | [UI mode](https://playwright.dev/docs/test-ui-mode) |
| Browser installation | Official CI guidance discourages caching browser binaries by default: restoration may cost as much as download, and Linux OS dependencies still need installation. Install only the browsers used by that job. | [Continuous integration](https://playwright.dev/docs/ci#caching-browsers) |

## Anchor decisions derived from the research

These are project recommendations, not quoted framework requirements. The
implemented configuration and recorded runs determine which have been adopted.

- **Preserve feature coverage while removing redundant work.** Seed prerequisites
  through the real API. Exercise the target action in the browser, assert its
  visible result, and check persistence where relevant. A setup request does not
  count as coverage of the corresponding creation UI.
- **Separate authentication from unrelated features.** Login, reload, logout and
  guest access get explicit fresh-session cases. Other features start with
  fixture authentication. Use unique users or isolated products/organizations
  according to which server state is actually shared.
- **Own every mutation.** Names include a run identifier, worker identity and a
  test-specific suffix. Fixtures retain created IDs and clean up only those IDs,
  in dependency order. Avoid deleting by a broad name prefix or resetting a
  shared backend as test teardown. Mutating Anchor tests run locally with a
  disposable database, following `anchor-ui/AGENTS.md`.
- **Keep abstraction small.** Fixtures manage lifetime, API helpers express
  concrete resource operations, and short feature helpers express repeated UI
  actions. Keep Playwright assertions visible in scenarios. Avoid generic
  click/wait wrappers and a page-object hierarchy that obscures failures.
- **Measure startup and execution separately.** Record build/backend readiness
  time, suite wall time, test count, worker count, slowest scenarios and flakes.
  Compare three identical runs before and after tuning; do not compare a cold
  build with a warm server or remove scenarios to claim a speedup.
- **Increase concurrency only after isolation works.** Start with a small worker
  count, measure both browser CPU/memory and backend contention, then compare
  one, two and four workers with the same scenario set. Introduce shards only
  when measured runtime warrants their extra startup and reporting work.
- **Bound failure cost.** Use observable waits, narrow assertions and reasonable
  per-operation budgets. Keep local retries off for diagnosis. A small CI retry
  allowance can collect diagnostics, but a flaky test remains work to fix.
- **Use recordings selectively.** Generated disposable accounts can support
  local failure traces. Real dev login credentials should keep the existing
  smoke recording policy. Reports and auth state remain local ignored artifacts.
  Benchmark headless execution separately from interactive UI mode.
- **Keep the coverage inventory honest.** Every shipped route/action needs an
  independently runnable scenario or an explicit limitation. Component tests
  retain exhaustive presentational/validation combinations; browser tests cover
  their integration and user outcomes. Disabled, skipped or never-run cases are
  not verified coverage.

## Maintaining this research

When changing fixture architecture, worker count, retries, browser coverage or
the Playwright version, recheck the relevant primary documentation above and
record the reason plus measured effect in the operational guide. Add a newly
discovered pitfall only after reproducing it, explaining its cause, preserving
the intended assertion and verifying the repair. Link the scenario or helper
that embodies the fix so future agents have an executable example.

Do not silently turn a failed expectation into an optional assertion, skip a
scenario, inject a response for the feature under test, or increase a global
timeout to make a failure disappear. A product bug should be recorded separately
from a test-harness bug. Keep this article list as research provenance; put
current commands, feature coverage and incident recipes in the smaller guide
files maintained alongside the tests.
