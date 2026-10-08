# Agent delivery workflow

This repository owns its setup, validation and review requirements. Installed Nanostack, Matt Pocock or Emil skills may help execute them; no workspace checkout or skill download is required to follow this procedure.

## Implement and verify

1. Read `CONTEXT.md`, relevant ADRs, the affected component guide and the current scripts/configuration. Work in an isolated Git worktree from current `origin/main`; preserve other developers' files. Change the OpenAPI contract before generated server/client code.
2. Define the changed behavior and acceptance criteria, including permissions, failure states and persistence. Select the affected existing E2E journeys and their prerequisites; add a regression for a reproduced defect.
3. Run affected-area E2E locally against this worktree, plus relevant service/contract, lint, type and build checks. Confirm actual scenario selection and source identity. Discovery, authored tests and an accepted launch do not prove execution.
4. Broaden selection for shared authentication, routing, fixtures or backend contracts. Use the complete local suite when its impact warrants it. Never weaken assertions, exclude a known failure or add retries to obtain green results.
5. Update authoritative documentation in the same PR when behavior changes or a verified finding will help another developer: current behavior in `docs/technical`, setup/test repairs in `docs/development`, operational repair in `docs/runbooks`, and resolved vocabulary in `CONTEXT.md`. Add an ADR only for a consequential trade-off whose reversal costs matter and whose rationale is surprising; preserve previous ADR numbers and record supersession in a new decision. Keep unresolved proposals in research and significant incidents in postmortems when useful. Update `docs/README.md` for new documents.
6. Verify documentation links and any changed command against its owning script. Record observed symptom, verified cause, effective fix and verification, with code/test/issue references. Exclude credentials, customer data and sensitive raw logs.
7. Commit and push the locally verified change. Keep every PR template section and its preview marker. Wait for all required CI on the current head and confirm that it actually selects the changed behavior. Diagnose a failed product assertion locally; retry infrastructure failures only after new evidence or a confirmed transient recovery.

Documentation-only changes use link, structure and command-provenance checks. Running product E2E is required for changed behavior, not for changes to documentation alone. State which checks executed and what CI legitimately skipped.

## Cloud runtime exception

A cloud executor can continue with CI when the documented owned local runtime is inaccessible. First demonstrate the exact missing capability through the authorized setup (Docker/container execution or approved development identity). A failing product assertion and a preference for CI do not qualify.

Run feasible local checks and preserve normal E2E fixtures/assertions. Put a warning at the very top of the PR, before its template:

```text
Affected-area local E2E could not be completed in this cloud environment.
Blocker: <documented command and safe observed error or missing capability>.
Local E2E not executed: <affected scenarios and prerequisites>.
Please provide <specific approved runtime access or secure provisioning>.
Local checks completed: <commands and results>.
Verification is continuing through CI: <current-head results and links>.
```

Continue CI iteration with the limitation disclosed. Request secure access, never secret values in the PR. Do not mutate shared deployments or weaken authentication to bypass the blocker. CI success does not become a local pass. After access is restored, run the affected local selection, verify owned cleanup and replace the warning with executed evidence. Any unavailable responsive or independent review remains outstanding.

## Browser-visible review

Before implementing a browser-visible change, read the project's local feature-review adapter and browser guide. Backend changes affecting a UI journey have the same requirement.

- List every changed action, loading/error/empty state and persistence boundary. Use disposable local data, accessible locators and assertion-based waits; update route coverage and impact selection where present.
- Confirm discovery, then execute the same affected selection on **mobile, tablet and desktop**, with bootstrap dependencies enabled. Account for every expected case on each profile. Zero matches, skips, retries and failures leave review incomplete. Record the actual emulated device and engine; emulation is not physical-device evidence.
- Capture meaningful states after assertions. Inspect text, clipping/overflow, reachable controls, focus and navigation on each profile. Rendered UI changes need before/after image pairs from base and branch with matched viewport, theme and data, using the project's `pnpm ui-shot`.
- Record readable videos on all three profiles. Use review-only pacing/checkpoint holds, inspect normal-speed playback, and state sampling limitations. Guest contexts inherit the profile's viewport and recording settings. Keep input captions disabled; inspect captures for secrets before publishing.
- Creating/changing a component also requires worst-case stories for long/unbreakable text, absent/empty data, one item, large counts and relevant error/loading states. Fix broken or visibly poor outcomes in the same change; record remaining fragile cases and open decisions. The optional `break-ui` skill can assist this local requirement.
- Ask a separate agent to validate the final diff, rerun affected responsive scenarios and inspect the media. Repair findings and refresh checks/media/review after a behavioral UI repair. Report a ready/not-ready verdict with evidence.
- Close owned managers/services and verify cleanup using the local runtime guide. The complete regression gate and its ordinary capture policy remain intact; responsive review supplements it.
- Fill the PR with tested source, exact scenario commands/results, inspected screenshots/videos, visual and independent verdicts, cleanup and coverage boundaries. Upload through supported `gh pr create/edit --attach` and inspect the saved GitHub media. Local paths and artifact-download promises do not satisfy attached evidence. Keep media in ignored `.ui-craft/`.

Completion means affected local checks and current-head CI pass, documentation matches the final source, and applicable responsive/independent review and PR media are complete. This procedure does not grant permission to merge or deploy.
