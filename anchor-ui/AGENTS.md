# Agent Guide: anchor-ui

Admin dashboard for the Anchor OaaS platform (Vite + React + TanStack Router/Query).

## Generated client — `src/client/` (do not edit)

`@hey-api/openapi-ts` generates `types.gen.ts`, `zod.gen.ts` (schemas prefixed `z`), and `@tanstack/react-query.gen` from `../apps/anchor/cmd/http/openapi.yaml`. Config: `openapi-ts.config.ts`.

Regenerate after **any** OpenAPI change, before writing frontend code:

```sh
./apps/anchor/generate_anchor.sh   # from the anchor app root
pnpm openapi-ts                    # then, inside anchor-ui
```

CI regenerates the client and fails if the result differs from the commit. Do not
leave `src/client/` stale.

- Validate forms with the generated Zod schemas — never hand-write validation for a field that exists in the spec. The generated schemas mark every field optional (OpenAPI default), so layer `.superRefine()` on top for required-field rules and UX messages.
- API fields are snake_case, form state is camelCase — map when surfacing field errors.

## Forms & state

- Local form state: `useState` + generated Zod schema. Server state: TanStack Query.
- Write-only fields (passwords, secrets) are never pre-populated from a server response; guard with `useRef` so query invalidation does not reset the form.

## Testing

Writing, extending, diagnosing, tuning or showing browser tests: read `e2e/guide/README.md` first. Reviewing browser changes or asynchronously populated forms: read `e2e/guide/review.md`.

Completing a feature or behavior change: follow `../.claude/skills/feature-review/SKILL.md`; mobile, tablet and desktop tests, reviewed screenshots and an attached readable PR video are completion requirements.

anchor-ui is an **app**: Storybook component tests for reusable UI + Playwright e2e per feature (same pattern as `echopoint/apps/frontend/e2e/`). Run the mutating e2e suite against a local backend, never prod.

Query by role/accessible name — no CSS/XPath selectors, no snapshot churn.

Creating or changing a component: run the `break-ui` skill on it (not in your skills? WebFetch `https://raw.githubusercontent.com/emilkowalski/skills/main/skills/break-ui/SKILL.md`). Its worst-case data lands as stories beside the demo story (`WorstCase`, plus `Empty` and `One` where they apply) in place of the skill's dev toggle, so the story suite guards it. Fix every Broken and Ugly finding in the same PR; list the Fragile rows and open decisions in the PR body.

## Verification

Run these before you push. CI runs the same four as separate steps.

```sh
pnpm check        # biome and shared UI contract
pnpm typecheck    # tsc, both projects
pnpm test         # vitest
pnpm build        # vite only, no type check
pnpm test-storybook # styled Chromium component/feature tests
```

`pnpm build` does not check types. `pnpm typecheck` is a different command. Run
both. `pnpm typecheck:app` skips `.storybook` and `vitest.workspace.ts`; the
deploy pipeline runs that one as a precondition, before anything ships.

Capture before/after images with `pnpm ui-shot` (Storybook port 6007); attach via `gh pr create/edit --attach`. `.ui-craft/` is ignored local scratch and never committed.

## Code style

- Avoid comments — name variables and functions clearly instead. Comment only a genuinely complex algorithm.

## UI work

Follow [the UI system and design-system consumer rule](docs/ui-system.md), shared with Echopoint. Import UI only from `@nanostackorg/design-system`; no local shadcn copies or styling props on shared components. Pages and routes compose closed components, blocks and layout; typed product visuals use `Box` with semantic tokens internally. Search the design system before adding UI. A neutral missing part goes back to the design system.

Read current design-system source and tokens (`src/styles.css`) from the sibling repo's `origin/main`, not `node_modules`. Primary path from `anchor-ui` is `../../nanostack-design-system`; inside `anchor/worktrees/<topic>/anchor-ui` it is `../../../../nanostack-design-system`.


Surface/elevation rules, semantic tokens and feedback states live in `docs/ui-system.md`. Light mode only: never author `dark:` classes.

For editable resource lists, open a dedicated detail page for View and Edit. Use one resource detail component for both modes; row clicks and Edit actions open edit mode, while View actions open read-only mode. Keep other row controls independent.
