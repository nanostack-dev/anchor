# UI system and consumer rule

Anchor uses the same published `@nanostackorg/design-system` components, tokens and variable fonts as Echopoint. Anchor remains light-only. The product owns routes, data, permissions, resource names, and domain visuals.

## Consumer rule

- Import UI only from `@nanostackorg/design-system` (root, `components/<name>`, `layout/<name>`, `blocks/<name>`, `provider`). Never copy a shadcn component into the product or import package internals.
- Wire Tailwind v4 in `src/styles.css`: import Tailwind, then the package stylesheet, and register `../node_modules/@nanostackorg/design-system/dist` with `@source`. Load Plus Jakarta Sans, Outfit and Geist Mono through Fontsource. Shared tokens are complete colors: use `var(--primary)`, never `hsl(var(--primary))`.
- Wrap the app in `DesignSystemProvider linkComponent={RouterLink}` so `ButtonLink`, `TextLink` and navigation link parts use the product router. Preserve external links, search parameters and fragments.
- Pages and routes compose components, blocks and layout blocks with props only. They pass no `className` or `style`.
- Before adding a component, search the design-system exports, components, blocks and layouts, then the sibling private registry. Read current source and theme tokens from the design-system repository's `origin/main`, not installed package output. A different look for the same use case needs an existing or shared variant. A visual explained by Anchor's domain belongs in a typed product component. A part a second product would use unchanged belongs in the design system.
- Product components are closed too: expose typed semantic variants. Within their implementation, compose shared parts by props or build on `Box` using design tokens. Never restyle shared markup, including named parts and rendered children, with classes, CSS selectors or token overrides.
- Style product visuals with Tailwind and semantic tokens. Repeated looks become typed product components. Custom CSS needs a concrete reason such as keyframes Tailwind cannot express. Do not author palette classes, arbitrary color utilities or `dark:` product classes.
- Generic missing components and variations return to the design-system repository. Apps pin exact published versions and carry no patched copies.

## Feedback and surfaces

Use shared `Alert`, `Empty`, `Skeleton`, `Spinner`, `Badge` and their semantic props. `FormAlert`, `StatusBadge` and `FormValidationError` adapt Anchor state to those parts. Toasts use `toast.add({ type, title, description, actionProps })` and the shared `Toaster`; mount it once in the app and in the Storybook preview.

Keep one surface owner per region. A table, card, alert or empty state must not receive a second card wrapper. `Page` owns the title and breadcrumbs; tables do not repeat them. Temporary overlays own their elevation. Loading, error, stale results, empty and pending states must stay actionable and distinguishable.

## Architecture and behavior

Routes own routing and auth; pages own data orchestration; `src/components/<domain>` holds typed Anchor visuals shared by those pages. New independent slices may use `src/features/<feature>/components`. Keep stories next to owned UI, and app-wide language/contract stories in `src/stories/system`. Avoid moving unrelated modules just to change their folder.

`AnchorDataTable` retains TanStack Table v8 server pagination/sorting/filtering, visibility, selection and bulk mutations. Its rendering uses shared Table components. The design-system `DataTable` uses v9 and is not a drop-in replacement. Neither selection modes nor all-matching callbacks may be lost when editing this adapter. Bulk actions stay page-scoped and preserve partial-failure retry.

License fields, schema DSL, permission trees, mutation orchestration and email content are product-specific. Recharts must match the package's chart version so charts share one context. Monaco remains an email editor engine; its surrounding UI uses library parts.

## Verification

`pnpm check` runs Biome and `check:ui`, which enforces public imports, closed shared styling, token colors and page composition. `pnpm typecheck` checks both TS projects; build alone does not check types. Run unit tests, production build and `pnpm test-storybook` before pushing.

The Storybook Vitest project explicitly installs the Tailwind plugin; it does not inherit the app plugin list. Test roles, accessible names, keyboard/focus and user-visible behavior. Give each `SelectContent` an explicit `aria-label` or `aria-labelledby` for the option list, as well as labeling its trigger; `check:ui` enforces the list name. Assert open-menu names and wait for closure/focus return before checking the settled state. Zero overlay transition durations only in test setup when required. Mutating Playwright e2e runs only against a local backend and disposable database.

Capture matched before/after image pairs with `pnpm ui-shot <story-id> --base http://localhost:6007`, using identical fixtures, viewport and light theme. Attach `.ui-craft/` images through `gh pr create/edit --attach`; never commit them. Update this guide when provider, styling or folder conventions change.
