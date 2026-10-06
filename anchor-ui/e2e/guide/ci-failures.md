# Inspecting a CI browser failure

Open the failed **Browser app (isolated)** workflow run, read the failing scenario
in **Test the isolated app**, then download the `anchor-browser-app-*` artifact.
The cleanup step runs before upload; a failed test keeps its failure status even
when teardown succeeds. The complete coverage gate still rejects skips,
retries, flaky passes and missing scenarios.

Extract the artifact and locate `report/index.html` inside the archive. From
`anchor-ui`, serve that folder with:

```sh
pnpm exec playwright show-report /absolute/path/to/extracted-artifact/report
```

Keep the report's `data/` directory beside `index.html`: it contains the attached
failure screenshots and page context. The JSON result, selection plan and
completion proof remain available separately. A failing run may have no
completion proof because the gate never accepted that run. Artifacts expire
after seven days; download evidence before investigating an older failure.

## Traces when a screenshot is insufficient

Run this workflow manually on the relevant branch with **Record traces and
retain failures** enabled. This sets `E2E_TRACE=1`; the default PR run leaves
tracing off and retries at zero. Retain-on-failure records every scenario and
discards successful recordings, so this diagnostic mode adds execution overhead.
It runs the same complete inventory rather than replacing coverage with retries.

The downloaded HTML report offers the failed scenario's trace attachment. Its
original `trace.zip` also remains in the artifact for local inspection:

```sh
pnpm exec playwright show-trace /absolute/path/to/trace.zip
```

The suite uses disposable local accounts and fixtures. Keep recordings within
the repository's access boundary: screenshots, page context and traces may show
fixture data; traces also contain browser requests and responses. Upload paths
include reports and browser evidence, rather than runtime environment files,
credential metadata or backend logs.

## Verify changes to reporting

Run `pnpm test:e2e:artifacts` after changing reporters, tracing or artifact
packaging. It imports the app configuration into a temporary two-case browser
fixture with no backend or Docker startup. One case intentionally fails; the
verification requires the Playwright child to exit with status 1, one passing
case, one failing case, zero retries/skips/flakes, and copied HTML attachments
that remain readable through the report after removing the original output
folders. It checks
default tracing off and opt-in tracing, including discarded passing recordings.
Temporary fixtures and output are removed when the verifier exits.

Measure normal full-suite timings before changing the default tracing policy.
The small packaging fixture validates evidence integrity; its elapsed time and
report size do not establish full-suite recording overhead. See
[performance.md](performance.md) for comparable suite measurements.

Reporter and artifact behavior follow the primary
[Playwright reporter documentation](https://playwright.dev/docs/test-reporters#html-reporter)
and [trace options](https://playwright.dev/docs/api/class-testoptions#test-options-trace).
