# Selecting affected Anchor browser tests

Researched October 5, 2026. The [operational guide](selective-testing.md) describes
the implemented explicit feature ownership and complete CI gate. This note
records the sources behind prioritizing browser tests while retaining full coverage.
Selection can miss dependencies; it does not preserve coverage automatically.

## Relevant primary readings

| Reading and date | Mechanics and limits relevant to Anchor |
| --- | --- |
| [Iterate quickly using the new `--only-changed` option](https://dev.to/playwright/iterate-quickly-using-the-new-only-changed-option-55m2) — Simon Knott, Playwright, August 21, 2024 | Demonstrates that changing a utility **imported by a spec** selects its dependent tests. Recommends a changed-first CI run followed by the full run. The example does not discover which application source a browser navigation exercises. |
| [Optimize Testing with Feature-Based Testing](https://nx.dev/docs/kb/feature-based-testing) — Nx, updated September 25, 2026 | Splits a monolithic E2E project into feature-scoped projects so `nx affected -t e2e` can select feature targets. Retains top-level smoke tests and cross-feature journeys; requires isolated data for concurrent features. Closest match to Anchor's domain folders. |
| [3 Test Splitting Techniques that Cut E2E Times up to 90%](https://nx.dev/blog/test-splitting-techniques) — Miroslav Jonaš, Nx, April 23, 2025 | Distinguishes distributing test files from selecting affected features. Its manual-splitting example declares feature `implicitDependencies` separately from the task dependency that builds the whole served application. A whole-app E2E dependency otherwise selects unrelated feature tests. The headline is not a measured saving for Anchor. |
| [Turborepo 2.11](https://turborepo.dev/blog/2-11) — Anthony Shew, September 18, 2026 | Adds experimental Go/`go.work` workspace discovery and affectedness, alongside explicit cross-toolchain task dependencies. Go support is no longer accurately described as JavaScript-only tooling. This release does not establish automatic HTTP feature-to-spec relationships. |
| [The Rise of Test Impact Analysis](https://martinfowler.com/articles/rise-test-impact-analysis.html) — Paul Hammant, August 22, 2017 | Describes his coverage-based experiments: run each test separately, record production files exercised, then use changes against a reference to select tests. Capturing runtime execution is a different mechanism from a JavaScript import graph. Instrumentation and maintaining the map have a cost. |
| [Predictive test selection](https://engineering.fb.com/2018/11/21/developer-tools/predictive-test-selection/) — Mateusz Machalica, Alex Samylkin, Meredith Porth and Satish Chandra, Facebook Engineering, November 21, 2018 | Learns failure probabilities from historical changes and test results, validates detection rates, and retrains as the codebase evolves. Exhaustive testing still precedes deployment. Its reported large-scale results are probabilistic and cannot be transferred to Anchor's 56 scenarios. |

## What Playwright actually knows

The [official CI guidance](https://playwright.dev/docs/ci#fail-fast) describes
`--only-changed` as a dependency-graph heuristic that can miss tests and explicitly
requires the full suite afterward. It also requires the comparison ref to exist
locally; its GitHub example uses a non-shallow checkout. This documentation is
undated and was checked on the research date.

Checked against Anchor's installed Playwright **1.60.0** and its
[versioned dependency collector](https://github.com/microsoft/playwright/blob/v1.60.0/packages/playwright/src/transform/compilationCache.ts):
selection uses collected test-file dependencies and explicitly registered
external dependencies. The collector excludes `node_modules`. This is a source
check, not a promise that every runtime input is represented.

Local applicability: [schema.e2e.ts](../features/licensing/schema.e2e.ts) imports
fixtures/helpers and navigates to routes;
[api.ts](../support/api.ts) sends HTTP requests;
[playwright.app.config.ts](../../playwright.app.config.ts) starts the frontend and
Go runtime through a child command. **Inference:** loading a page URL or calling
a Go endpoint does not itself add that page's React implementation or Go service
files to the test runner's import graph. Editing a spec/helper is therefore a
useful `--only-changed` case; changing an application page or backend feature
needs an explicit mapping or broader fallback.

## Recommended Anchor approach

For the current [56-scenario inventory](coverage.md), start with a small reviewed
Git-path-to-domain manifest rather than introducing a new build system or
predictive service solely for selection. This recommendation adapts Nx's
explicit feature scopes; it is not a claim that folder names prove isolation.

- Compute changes against the PR's actual base/merge-base, with both refs
  available. Union every matching domain and include changed/new specs. Handle
  both paths of a rename; missing refs or unmapped executable files select all.
- Map feature-local UI and Go files to their owning scenarios **and consumers**.
  For example, licensing changes may affect tenancy views or emitted events;
  review those relationships rather than assuming one folder is sufficient.
- Select all for shared auth/session, product context, navigation, shared UI,
  generated API/OpenAPI contracts, migrations, dependency manifests/lockfiles,
  common backend infrastructure, runtime/configuration and shared test fixtures.
  Keep critical startup/login and cross-feature journeys in the selection.
- Run the union in **one Playwright invocation**, preserving bootstrap project
  dependencies and the suite's single owned runtime. Splitting into independent
  commands can multiply startup/build costs; measure total wall time including
  setup. Preserve every selected scenario's assertions and isolation.
- Print changed paths, selection reasons and selected scenario count. Validate
  manifest examples, default-to-all behavior and newly added source paths.
  Compare selected outcomes with full runs to discover missing relationships.

Use selection for local iteration and, if measured useful, an early CI failure
signal. Keep the full suite before declaring the change complete or merge-ready.
Running a subset and then all tests duplicates passing work; benchmark whether
earlier failure feedback justifies that cost. A later policy that skips full PR
runs would need an explicit accepted risk, evidence of selection misses, and a
full-run schedule; none of the readings guarantees unchanged bug detection.

Revisit graph tooling if the repository develops independently scoped projects,
or runtime coverage/predictive selection if full-suite cost becomes substantial.
Keep source-to-feature mappings alongside [coverage](coverage.md), update them
with new features, and recheck version-specific behavior after tooling upgrades.
