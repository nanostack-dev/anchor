# Anchor UI — Nanostack Style Migration

This documents the migration of `anchor-ui` onto the shared Nanostack
application style (OpenSpec change `unify-anchor-ui-with-nanostack-style`).
Anchor remains a **standalone shadcn app**; it does not import `packages/ui`
and is **light-mode only**.

## Source of truth

- **Primitives:** shadcn is the source of truth. Local primitives in
  `src/components/ui/**` are aligned to the current product style (rounded
  shapes, semantic tokens). Update them via the shadcn workflow or by aligning
  local source — do **not** copy primitives from the Nanostack registry just
  for convenience.
- **Style reference:** the shared Nanostack reference design is the reference
  token system. Anchor mirrors it.

> Note: the change proposal prose described a "red primary / warm off-white"
> theme, but the actual reference design uses a **blue primary**
> (`--primary: 217 72% 43%`) on a cool off-white background with Plus Jakarta
> Sans / Outfit fonts. The reference **code** is authoritative, so Anchor
> adopts those values.

## Theme tokens (`src/styles.css`)

- HSL CSS variables wrapped via `hsl(var(--token))` in `@theme inline`,
  matching the reference. Tailwind v4, CSS-first config (no `tailwind.config`).
- Added semantic tokens: `success`, `warning`, `destructive` (+ `-foreground`),
  `border-strong`, `accent-soft`, `surface-*`, `highlight`.
- Radius scale extended to `rounded-3xl` / `rounded-4xl` (used by Button,
  Badge, Page sections).
- Fonts: body `Plus Jakarta Sans` (Inter fallback), headings `Outfit`
  (Geist fallback), loaded via Google Fonts `@import`.
- **Light only.** The `.dark` token block is removed. `@custom-variant dark`
  is kept so shadcn primitives' `dark:` utilities still compile but stay inert.
  Product/route code must not author `dark:` classes.

## Shared components added

- `components/ui/spinner.tsx`, `components/ui/empty.tsx`,
  `components/ui/field.tsx` — standard shadcn surfaces.
- `components/common/StatusBadge.tsx` — semantic status tones
  (`success | warning | destructive | info | neutral`). Use this for all table
  / status pills instead of hard-coded `bg-*-100 text-*-800` spans.
- `components/layout/app-shell.tsx` — local `AppShell` adapter mirroring the
  shared Nanostack shell composition (skip link, sticky topbar, sidebar inset).
- Added `success` / `warning` variants to `Badge` and `Alert`.

## Decisions

| Question | Decision |
| --- | --- |
| Icon library | **Keep Lucide.** Port the reference *styling* only; no Phosphor switch. |
| Breadcrumbs | **Anchor-local route adapter** in `components/common/Page.tsx`, derived from the TanStack Router location. |
| Data table | **Keep `AnchorDataTable`** (already supports manual/server pagination, sorting, filtering, faceted filters, column visibility, and page / all-matching selection). The registry `DataTable` would lose behavior. Restyled in place — loading uses `Skeleton`, empty/no-results uses muted standard text, borders use semantic tokens. |
| `@nanostack` registry dependency | **Not added.** Anchor uses local shadcn primitives + local `AppShell`/`Page` adapters, so the whole migration ships as one Anchor PR. |
| `warning` token | **First-class.** Added `--warning` / `--warning-foreground` plus `warning` Badge/Alert variants for statuses like `INVITED`, `ROTATED`, `QUEUED`. |
| `Page` API | **Preserved.** `Page` keeps its `title` / `description` / `breadCrumbs` / `pageInfo` props (now also `actions`); internals restyled to the shared Nanostack header composition, so the 19 route call-sites are unchanged. |

## What stays product-local vs. registry

- **Product-local (stay in Anchor):** product/API-key/role dialogs, integration
  setup pages, product/org column definitions, `sidebar-config`, the
  `ProductTopBar` / product selector, the `AnchorDataTable` adapter.
- **Registry-eligible (future, separate `nanostack-registry` PR):** the
  restyled `VerticalStepper` if it stays app-neutral; promotion was **not**
  done in this change because the registry is a separate repo.

## Frontend rules going forward

Migrated and new UI must follow: shadcn workflow for primitives, semantic
tokens (no hard-coded palette classes, no `dark:` in product code), Vercel
React best practices, Vercel composition patterns, and the web interface
guidelines (focus, keyboard nav, accessible dialog titles, labels, responsive
behavior).

**Surfaces — no box-in-a-box (nested cards).** AI tooling tends to stack
containers and produce double borders. Keep one elevation per region: a
component that already draws a bordered/elevated surface (a `Card`, the
`AnchorDataTable` card, an `Alert`, `Empty`) must not be wrapped in another
`Card`. Pick a single surface owner — the reusable component *or* the page,
never both. `AnchorDataTable` owns its card, so render it bare (no wrapping
`Card`); the `<Page>` supplies the title/description (don't duplicate it inside
the table). Use `Empty` for placeholder states, not a title-only `Card`.

## Verification

`pnpm check` (biome) · `pnpm build` (vite + tsc) · `pnpm test` (vitest) all
green. Manual desktop/mobile browser inspection of key surfaces is still
recommended before release.
