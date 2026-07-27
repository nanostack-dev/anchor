---
name: anchor-ui-design
description: Visual and structural conventions for anchor-ui components — surface/elevation rules that prevent nested-card "box-in-a-box" layouts, semantic color tokens instead of raw palette classes, and the canonical loading/error/empty/status treatments. Load before adding, restyling, or reviewing any anchor-ui component, page, or table.
---

# anchor-ui design rules

Light mode only. Never author `dark:` classes in product or route code. Framer Motion for transitions. Check `src/components/` for an existing shadcn component before creating a new one.

## Surfaces — no box-in-a-box

AI tooling frequently stacks containers, producing double borders. Don't.

- **One elevation per region.** A component that already renders a bordered/elevated surface (`Card`, the `AnchorDataTable` card, `Alert`, `Empty`) must not be wrapped in another `Card` or bordered container. A thing is *either* the card *or* inside one — never both.
- **Single surface owner.** Either the reusable component draws the card or the page/section does. Not both.
- **`AnchorDataTable` owns its card.** Do not wrap it in `<Card>`. `<Page>` supplies the title/description — do not repeat the title inside the table.
- **No touching or stacked borders, shadows, rounded boxes.** Two adjacent bordered surfaces collapse to one.
- **Placeholders/empty states** use `Empty`, not a title-only `Card` that duplicates the page heading.

## Tokens

Semantic tokens and variants only — no hard-coded palette classes (`bg-blue-500`, `text-green-700`, `bg-slate-100`). Use `primary` / `muted` / `accent` / `success` / `warning` / `destructive` plus their `-foreground` pairs.

Prefer `gap-*` over `space-x/y-*`, and `size-N` over an equal `h-N w-N`.

## Feedback states

| State | Use |
|---|---|
| Loading | `Skeleton` / `Spinner` — no custom `animate-pulse` colored blocks |
| Error | `Alert variant="destructive"` or `text-destructive` |
| Empty | `Empty` |
| Status | `StatusBadge` (`@/components/common/StatusBadge`) or a `Badge` variant — never a raw colored `<span>` pill |

No `style={{ color }}` literals.
