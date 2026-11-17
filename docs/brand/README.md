# Anchor logo

The Anchor mark is an **anchor drawn in squares**: eleven rounded squares on a five-by-five grid, forming the ring, the stock, the shank, the two arm tips and the crown. It uses the same construction as the [Echopoint mark](https://github.com/nanostack-dev/echopoint/blob/main/docs/brand/README.md): rounded squares with a 20% corner radius, a tile with a 23% corner radius, the product's primary color. The two icons sit side by side as one family.

[`build.py`](build.py) generates every file below from one copy of the geometry. Change the geometry there and rebuild; never edit an output. In code, use the components in `anchor-ui/src/components/brand/AnchorLogo.tsx`, never a copied SVG.

```sh
python3 docs/brand/build.py --font anchor-ui/node_modules/@fontsource-variable/outfit/files/outfit-latin-wght-normal.woff2
```

It needs `fontTools` with `brotli` (`pip install fonttools brotli`), `rsvg-convert` (`brew install librsvg`) and ImageMagick (`brew install imagemagick`). Run `pnpm install` at the repository root first if the font path is missing.

## Construction

The mark is drawn on a 54 × 54 grid. Each square is 10 units with a 2-unit corner radius (20%), and squares sit 1 unit apart (10% of a square), so the grid pitch is 11.

| Row | Squares (columns 0–4) | Part |
| --- | --- | --- |
| 0 | 2 | Ring |
| 1 | 1, 2, 3 | Stock |
| 2 | 2 | Shank |
| 3 | 0, 2, 4 | Arm tips and shank |
| 4 | 1, 2, 3 | Crown |

The **icon** puts the mark on a square tile:

- Tile corner radius: 23% of the tile.
- Mark size: 50% of the tile, centered.

The **favicon cut** fills 62% of the tile instead of 50%, so each square stays about 2 px wide at 16 px. Use it only for the favicon and other icons of 32 px or less.

## Color

The logo uses the product's primary color in each theme, as Echopoint does. Blue and lime never appear together.

| Name | Hex | Token | Use |
| --- | --- | --- | --- |
| Blue | `#1F5BBD` | `--primary` (light) | Icon tile, mark on light backgrounds |
| Lime | `#A0E33B` | `--primary` (dark) | Icon tile, mark on dark backgrounds |
| Night | `#090E1A` | `--background` (dark) | Mark on a lime tile |
| Ink | `#0F1729` | `--foreground` (light) | Wordmark on light, one-color print |
| White | `#FFFFFF` | `--primary-foreground` (light) | Mark on a blue tile, wordmark on dark |

anchor-ui is light-only today; the dark files exist for dark surfaces outside the app (slides, social cards, the browser tab in dark mode).

## Wordmark and lockup

- The wordmark is **anchor**, all lowercase, set in Outfit SemiBold (600) with −2% letter spacing, the Echopoint wordmark's type. The files are outlined, so they need no font.
- In a sentence, write the name as "Anchor".
- Lockup: icon on the left, wordmark on the right. With an icon height of H, the gap is 0.27 H, the type size is 0.72 H, and the x-height is centered on the icon.

## Clear space and minimum size

- Clear space around the icon, the mark or the lockup: one square of the mark (10 of its 54 units) on every side.
- Minimum sizes: icon 16 px (favicon cut below 32 px), mark alone 16 px, lockup 96 px wide.

## Don't

- Rotate, mirror or stretch the mark.
- Add, remove or resize squares, or change the gaps.
- Give one square a different color from the others.
- Add outlines, gradients, shadows or glows.
- Put Blue and Lime side by side.
- Place the bare mark on a photo or a busy background. Use the icon.

## Files

| File | Use |
| --- | --- |
| `assets/anchor-icon.svg` | App icon, light. Blue tile, white mark. |
| `assets/anchor-icon-dark.svg` | App icon, dark. Lime tile, Night mark. |
| `assets/anchor-icon-square.svg` | Full-bleed square for platforms that round corners themselves (iOS, app stores). |
| `assets/anchor-icon-512.png`, `assets/anchor-icon-192.png` | Raster icon for listings and manifests. |
| `assets/anchor-mark-{blue,lime,white,ink}.svg` | The mark without a tile. |
| `assets/anchor-lockup-light.svg`, `assets/anchor-lockup-dark.svg` | Icon and wordmark. |
| `assets/anchor-wordmark-{ink,white}.svg` | Wordmark alone. |
| `anchor-ui/public/favicon.svg` | Favicon cut. Follows the browser's light or dark scheme. |
| `anchor-ui/public/favicon.ico` | 16, 32 and 48 px fallback for browsers without SVG favicons. |
| `anchor-ui/public/apple-touch-icon.png` | 180 px home-screen icon. |
| `anchor-ui/public/logo.svg` | The blue mark, for places that can only take an image URL. |

## In code

- `AnchorLogo` draws the mark in `currentColor`. Sizes: `sm` 16 px, `md` 20 px, `lg` 24 px, `xl` 96 px, `hero` 160 px. Pass `label` when the mark stands alone; without it the mark is hidden from assistive technology.
- `AnchorIcon` draws the icon with `bg-primary` and `text-primary-foreground`, so it follows the theme on its own. Sizes: `sm` 32 px, `md` 36 px, `lg` 40 px.
- The sign-in screens use `AnchorIcon` beside the name and `AnchorLogo` in the loading panel.
- The sidebar brand slot shows the Anchor mark: with `@nanostackorg/design-system` 0.2.x the slot draws the primary tile and Anchor passes `logo={<AnchorLogo size="md" />}`; from 0.3.2 the slot draws no tile and Anchor passes `mark={<AnchorIcon size="sm" />}`. Without `mark`, 0.3.x shows the Nanostack mark, which the design system's own demos keep.
- The story `Brand/Anchor logo` (`AnchorLogo.stories.tsx`) shows every size.
