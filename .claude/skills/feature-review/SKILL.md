---
name: feature-review
description: Complete an Anchor feature or behavior change with independent Playwright tests on mobile, tablet and desktop, reviewed screenshots and readable PR videos.
---

# Feature review

Run this before calling a feature or behavior change complete. The implementing
agent owns the repair and PR evidence; a separate validation agent checks the
result. Work in the feature's isolated worktree. Read
`anchor-ui/e2e/guide/README.md` and the relevant domain tests first.

## Validate the changed behavior

1. List the changed user actions, relevant loading/error/empty states and
   persistence boundaries. Extend the owning `e2e/features/<domain>/` scenarios
   to assert those outcomes through the real UI. Reuse `world` for isolated API
   prerequisites, accessible locators, web-first assertions and controlled real
   responses for races. Keep presentation pacing out of correctness waits.
   Use `guestPage` from the shared fixtures when a journey needs an independent
   unauthenticated browser: it inherits the selected device and records guest
   evidence. Any additional context used for feature behavior must inherit those
   settings too; desktop-only authentication setup is a prerequisite, not device
   coverage.
2. Run the narrow ordinary suite while implementing. Update route coverage and
   the impact manifest for new routes/dependencies; run `pnpm check:e2e` after
   inventory changes. Record reproduced failures and verified repairs in the
   relevant `e2e/guide/troubleshooting/` file. The existing guide is the source
   of best practices; evolve it when a verified solution changes a convention.
3. Run every changed browser behavior through `pnpm test:e2e:review` on all three
   projects. This config shares one disposable backend and bootstrap, records
   each profile at its actual viewport and writes to ignored `.ui-craft/review/`.
   Use explicit spec locations and meaningful titles to keep this run focused.
   Confirm discovery with `--list`, then run with the identical selection. Keep
   dependencies and all three projects enabled. A zero-match, skipped, retried
   or failing scenario is incomplete validation.

   From `anchor-ui`, for example:

   ```sh
   pnpm test:e2e:review e2e/features/platform/products.e2e.ts --grep 'product create validation' --list
   pnpm test:e2e:review e2e/features/platform/products.e2e.ts --grep 'product create validation'
   pnpm exec playwright show-report .ui-craft/review/report
   ```

   Look up current devices, dimensions and pacing in
   `playwright.review.config.ts`. These are Chromium device emulations; name
   actual hardware or additional browser engines separately when tested.
   The shared bootstrap is infrastructure and runs once on desktop. A change to
   first-run initialization itself needs fresh-runtime scenarios on each profile;
   the shared bootstrap does not establish three-profile coverage of that UI.
4. Add `captureReviewCheckpoint(page, testInfo, "meaningful-state")` from
   `e2e/support/review` immediately after an assertion at states a reviewer needs
   to inspect: a dialog, validation error, saved/reloaded result or changed
   layout. It captures and briefly holds that state only in review mode. The
   ordinary suite keeps its normal speed. A passing test also captures its final
   viewport. Capture full-page images separately when scrolling content matters.
5. Inspect the screenshots for each profile: clipping/overflow, readable text,
   reachable navigation/actions, dialog/keyboard focus, touch interaction and
   the actual changed states. Fix findings in the same change and rerun the
   affected scenarios. Screenshots require a visual verdict; generating PNGs
   alone does not validate layout. For rendered UI changes, use `pnpm ui-shot`
   to capture before/after pairs from the PR base and feature branch with the
   same data, viewport and light theme, for each affected profile. Follow the
   existing screenshot workflow and keep captures in `.ui-craft/`.
6. Finish the ordinary complete gate with `pnpm test:e2e:verify`, plus the
   applicable UI/Storybook/service checks. Responsive evidence supplements the
   complete gate. Normal CI retains its unpaced desktop suite and existing
   failure diagnostics; the three-profile recording is a focused review run.

For a service or tooling change with no browser-visible behavior, state that
boundary in the PR and provide the corresponding contract/service/tooling tests.
If the service change affects an existing UI journey, validate that journey on
all three profiles. An unavailable device/browser/runtime or a product failure
is an explicit outstanding requirement, with its failing command and evidence.

## Delegate independent validation

After the implementing agent has passing checks and inspected its captures,
dispatch a validation agent with the worktree, tested commit/diff, changed
behaviors, exact scenario selection and local report/capture paths. Ask it to:

- Read the changed UI and tests and identify missing behavior or weak assertions.
- Rerun the affected scenarios on mobile, tablet and desktop using the review
  setup; inspect each profile's meaningful screenshots and watch the videos.
- Return findings with evidence and a ready/not-ready verdict, including actual
  scenario results, visual observations and playback readability.

One coordinator owns app startup/teardown and shared configuration. Agents own
separate domains; a validation agent reports repairs to the implementing agent
instead of editing its files concurrently. Reuse a runtime only through the
managed ownership/fingerprint checks. Shut it down and verify cleanup when done.
An independent agent's approval requires evidence from the final tested source.

## Attach review evidence

Watch each relevant video at normal playback speed before attaching it. The
review config paces actions and checkpoint holds; use `E2E_REVIEW_SLOW_MO` to
increase pacing if important transitions still pass too quickly. Keep text
readable and trim unrelated setup when useful. Preserve the important action,
feedback and final outcome, at normal speed. Ordinary regression tests retain
assertion-based waits; deliberate holds exist only for presentation captures.

Playwright saves videos after browser contexts close. If the installed `gh`
requires a supported upload format, convert the selected WebM to MP4 without
speeding it up (requires `ffmpeg`):

```sh
ffmpeg -i .ui-craft/review/results/<case-profile>/video.webm -c:v libx264 -pix_fmt yuv420p -movflags +faststart -an .ui-craft/review/<profile>.mp4
```

Label each profile's screenshots and video in the PR's Feature review section.
Include the tested commit, selected scenarios, results per profile, visual
verdict, independent validator's verdict and any remaining coverage boundary.
Use `gh pr create --attach` or `gh pr edit <number> --attach` to upload the images
and videos to GitHub; the repository's CLI supports both. Keep every PR template
section and the preview marker. Read `gh pr edit --help` for attachment syntax.

```sh
gh pr edit <number> --attach .ui-craft/review/mobile.mp4 --attach .ui-craft/review/tablet.mp4 --attach .ui-craft/review/desktop.mp4
```

Verify the saved PR description contains working GitHub attachment links. If
upload partly fails, inspect the existing PR and retry only missing files. An
artifact download, local path or promised recording does not satisfy the video
attachment requirement. Keep evidence out of commits, use only disposable local
fixture data and inspect captures for secrets before upload. After a UI change
following validation, refresh its tests, screenshots/video and independent review.

The feature is complete when all changed behaviors pass on the three profiles,
screenshots and videos have been inspected, the validator's findings are
resolved, the complete gate passes and the PR contains the review evidence.

Primary references: [projects and dependencies](https://playwright.dev/docs/test-projects),
[device emulation](https://playwright.dev/docs/emulation),
[videos](https://playwright.dev/docs/videos) and
[test best practices](https://playwright.dev/docs/best-practices).
