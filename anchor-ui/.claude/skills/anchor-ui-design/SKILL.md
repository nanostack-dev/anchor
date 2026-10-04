---
name: anchor-ui-design
description: Anchor UI ownership, shared Nanostack components, semantic tokens, surfaces and feedback. Load before adding, reshaping or reviewing Anchor UI.
---

# Anchor UI design

Read `docs/ui-system.md` for the complete consumer contract, matching Echopoint. Anchor is light-only.

1. Search the current Nanostack design-system source and sibling registry before adding UI. Use public `@nanostackorg/design-system` components, blocks and layout; never local shadcn copies.
2. Pages and routes compose closed parts by semantic props. Shared components accept no `className` or `style` and must never be restyled through selectors, children, slots or token overrides.
3. Anchor-specific visuals are typed product components. Their implementations can use `Box` and Tailwind with semantic design tokens. A neutral component another product needs belongs in the design system.
4. Use one surface owner per region. Never wrap an existing table/card/alert/empty surface in another card. `Page` supplies titles; tables do not repeat them.
5. Use `Skeleton`/`Spinner` for loading, `Alert`/`FieldError` for errors, `Empty` for empty states, and `StatusBadge` or shared semantic Badge tones for status. Preserve retry, stale results, pending/disabled actions and accessible feedback.
6. Use semantic colors and `-on-tint` text on tinted surfaces; no raw palette colors, arbitrary color utilities or `dark:` product classes. Shared light tokens and fonts are the Echopoint theme.
7. Test meaningful keyboard, focus, overflow and domain behavior in colocated stories. Run all guide gates; attach matched before/after `pnpm ui-shot` images to the PR, never the commit.
