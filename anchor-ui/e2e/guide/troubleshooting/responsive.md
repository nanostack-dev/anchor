# Responsive review findings

## A passing product test clipped the Refresh button

The independent review at `6ec4ae24` passed all 19 selected executions, including
six mobile cases, but the mobile saved-product-config screenshot cut off part of
**Refresh products**. The existing visible-result assertions did not establish
that the whole action fit inside the viewport.

`ProductTopBar` let the selected-product button keep its intrinsic width beside
the refresh action. The parent header also needed room for the sidebar trigger
and padding. A valid 100-character product name in the AppShell WorstCase story
reproduced the overflow more strongly, including at tablet widths.

Constrain the product-owned selector wrapper to the available width, use the
shared Button's `width="fill"` prop and reserve space for the refresh action.
Keep the full accessible name and a full-value affordance when the trigger text
truncates. Product menu content must also wrap long names within the available
popup width. Shared components retain their supported semantic props.

Truncate the variable product name, not the fixed **Working on:** prefix. The
first repair fit every control but reduced that prefix to **W…** on mobile and
tablet. Hide the redundant visual prefix below the small-screen breakpoint;
keep it fully readable above that breakpoint and retain the full accessible
label on every device. Enlarged mobile text can wrap the refresh action onto
the next line rather than reducing the selected name to zero width.

The product lifecycle now polls the Refresh button's bounding box against the
current viewport after reloading saved configuration. AppShell stories exercise
the selector, refresh action and complete menu text with realistic worst-case
data. `toBeVisible()` remains useful for visibility; viewport bounds guard this
specific clipping failure.

Verify the repair with the same device selection and meaningful screenshot:

```sh
pnpm test:e2e:review e2e/features/platform/products.e2e.ts --grep 'product create validation'
pnpm test-storybook
pnpm test:e2e:verify
```

Before/after PR images use `pnpm ui-shot layout-appshell--worst-case` with the
same fixture, light theme and profile dimensions. Images and videos stay under
ignored `.ui-craft/`; the independent validator must inspect the fixed screenshot
as well as the passing assertions.

## Mobile video dimensions differ by one pixel

The 393×851 mobile screenshots and test metadata confirmed the selected device.
The finalized WebM encoder produced 392×850 frames at 25 fps; tablet 768×1024 and
desktop 1440×900 recordings retained their even dimensions. Encoding normalizes
odd dimensions. Check configured emulation plus PNG dimensions and meaningful
UI content before diagnosing a desktop-context fallback from video metadata.

Inspect the actual file with `ffprobe`; retain its native speed when converting
to MP4. The [feature review skill](../../../../.claude/skills/feature-review/SKILL.md)
owns recording, playback and attachment instructions.
