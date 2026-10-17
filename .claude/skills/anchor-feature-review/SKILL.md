---
name: anchor-feature-review
description: Bind the repo-owned browser review procedure to Anchor's managed Playwright runtime, responsive projects, fixtures, screenshots and PR evidence. Read before implementing a browser-visible Anchor change.
---

# Anchor feature review

Read the repo-owned [agent delivery and browser review procedure](../../../docs/development/agent-workflow.md) before implementing a browser-visible change, including a backend change that affects a UI journey. It owns the completion criteria; this adapter supplies project commands and fixtures. Installed shared skills are optional assistance, and no workspace or remote skill download is required.

All paths in the bindings below are repository-relative; run frontend commands from `anchor-ui/`.

## Project bindings

| Shared requirement | Anchor source |
| --- | --- |
| Setup and runtime ownership | `anchor-ui/e2e/README.md`; `anchor-ui/playwright.app.config.ts` |
| Scenario conventions and domain ownership | `anchor-ui/e2e/guide/README.md`; `anchor-ui/e2e/features/<domain>/` |
| Narrow implementation run | `pnpm test:e2e:app <spec>`; `anchor-ui/package.json` |
| Coverage and affected selection | `anchor-ui/e2e/guide/coverage.md`; `anchor-ui/e2e/guide/selective-testing.md` |
| Review profiles, viewport, pacing and capture policy | `anchor-ui/playwright.review.config.ts` |
| Real UI prerequisites and guest contexts | `world` and `guestPage` in `anchor-ui/e2e/support/fixtures.ts`; actions in `support/ui.ts` |
| Named screenshot checkpoints | `captureReviewCheckpoint` in `anchor-ui/e2e/support/review.ts` |
| Layout captures | `pnpm ui-shot`; `anchor-ui/scripts/ui-shot.mjs`; matched light-theme before/after pairs in ignored `.ui-craft/` |
| Regression gate and CI evidence | `pnpm test:e2e:verify --base <actual-PR-base-ref>`; `anchor-ui/e2e/guide/ci-failures.md` |
| Applicable UI checks | `anchor-ui/AGENTS.md`, Verification section |
| Reproduced failures and repairs | `anchor-ui/e2e/guide/troubleshooting.md` and its domain files |
| PR fields and upload mechanism | `.github/pull_request_template.md`; installed `gh pr create/edit --help` attachment options |

The managed app creates disposable local accounts and products; use it for
mutating review journeys. Saved development credentials belong to the separate
read-only smoke suite. Review reuses the managed app bootstrap once; initialization
changes still need fresh-runtime coverage on each selected profile.
Resolve the actual PR base for stack-layer verification. Close interactive runners
before the complete gate, which requires a fresh managed runtime and refuses filters.

## Focused review commands

Read `e2e/guide/README.md` and the owning scenario before choosing a selection.
For example, verify discovery, then execute the same product-validation journey:

```sh
pnpm test:e2e:review e2e/features/platform/products.e2e.ts --grep 'product create validation' --list
pnpm test:e2e:review e2e/features/platform/products.e2e.ts --grep 'product create validation'
pnpm exec playwright show-report .ui-craft/review/report
```

Keep the `bootstrap` dependency and all `mobile`, `tablet` and `desktop` projects
enabled. These profiles use Chromium emulation; inspect their current device
settings in the review config. `guestPage` inherits those settings and records
guest evidence. After a meaningful assertion, call
`captureReviewCheckpoint(page, testInfo, "meaningful-state")`; it captures and
holds that state only in review mode. The local review procedure governs inspection
and independent validation against the final source.

Review JSON, HTML and captures live in ignored `.ui-craft/review/`. Adjust
`E2E_REVIEW_SLOW_MO` only for review recordings. Keep action captions disabled
because filled values, including passwords, can appear in them. For GitHub uploads,
convert finalized WebM recordings without changing their speed when necessary:

```sh
ffmpeg -i .ui-craft/review/results/<case-profile>/video.webm -c:v libx264 -pix_fmt yuv420p -movflags +faststart -an .ui-craft/review/<profile>.mp4
```

Fill the PR's Feature review section using the local evidence requirements;
preserve every template section and the `<!-- preview-deploy -->` marker. Upload
with the installed CLI's `--attach` support and verify the saved media players.
After shutting down owned services, verify managed cleanup:

```sh
node scripts/e2e-runtime.mjs verify-stopped
```
