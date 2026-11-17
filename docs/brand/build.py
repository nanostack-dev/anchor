"""Build the Anchor logo files from one copy of the geometry.

The mark is an anchor drawn with twelve rounded squares on a 5 x 5 grid, in the
same construction as the Echopoint mark: every corner radius is 20% of its square,
and the icon puts the mark on a tile with a 23% corner radius.

Usage, from the repository root:
  python3 docs/brand/build.py --font /path/to/outfit-latin-wght-normal.woff2

Needs fontTools with brotli (pip install fonttools brotli), rsvg-convert
(brew install librsvg) and ImageMagick (brew install imagemagick). The font is
Outfit (SIL Open Font License, https://github.com/Outfitio/Outfit-Fonts), the
variable file that @fontsource-variable/outfit ships; the script sets it to
SemiBold (600) and outlines the word, so no output needs the font.
"""
import argparse
import os
import subprocess

from fontTools.pens.boundsPen import BoundsPen
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTFont
from fontTools.varLib.instancer import instantiateVariableFont

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
ASSETS = os.path.join(ROOT, "docs", "brand", "assets")
PUBLIC = os.path.join(ROOT, "anchor-ui", "public")

BLUE, LIME, NIGHT, INK, WHITE = "#1F5BBD", "#A0E33B", "#090E1A", "#0F1729", "#FFFFFF"

# The anchor on its 5 x 5 grid: ring, stock, shank, arm tips beside the shank, crown.
PATTERN = ["..X..", ".XXX.", "..X..", "X.X.X", ".XXX."]
SQUARE, GAP = 10, 1  # the gap is 10% of a square
MARK = 5 * SQUARE + 4 * GAP  # 54
RADIUS = 0.2

TILE_RADIUS = 0.23
ICON_FILL = 0.50  # the mark is half the tile
FAVICON_FILL = 0.62  # the favicon cut fills more of the tile, for 32 px and below


def squares(size, x0=0.0, y0=0.0):
    """The mark's squares as (x, y, side) scaled so the mark is `size` wide, starting at (x0, y0)."""
    k = size / MARK
    return [(x0 + j * (SQUARE + GAP) * k, y0 + i * (SQUARE + GAP) * k, SQUARE * k)
            for i, row in enumerate(PATTERN) for j, cell in enumerate(row) if cell == "X"]


def rects(sq, fill=None, cls=None):
    attr = f' fill="{fill}"' if fill else f' class="{cls}"' if cls else ""
    return "".join(f'<rect x="{x:.2f}" y="{y:.2f}" width="{s:.2f}" height="{s:.2f}" rx="{s * RADIUS:.2f}"{attr}/>'
                   for x, y, s in sq)


def tile(size, fill, radius=True):
    rx = f' rx="{size * TILE_RADIUS:.2f}"' if radius else ""
    return f'<rect width="{size:g}" height="{size:g}"{rx} fill="{fill}"/>'


def svg(view_box, body, label=None):
    a = f' role="img" aria-label="{label}"' if label else ""
    return f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="{view_box}"{a}>{body}</svg>\n'


def write(path, text):
    with open(path, "w") as f:
        f.write(text)
    print("wrote", os.path.relpath(path, ROOT))


def raster(src, dst, size):
    subprocess.run(["rsvg-convert", "-w", str(size), "-h", str(size), "-o", dst, src], check=True)
    print("wrote", os.path.relpath(dst, ROOT))


def icon_body(size, tile_fill, mark_fill, fill=ICON_FILL, radius=True):
    m = size * fill
    return tile(size, tile_fill, radius) + rects(squares(m, (size - m) / 2, (size - m) / 2), mark_fill)


def wordmark(font_path, text="anchor", size=46.08, tracking=-0.02):
    """The word outlined to one path, baseline at y=0; returns the path, its ink bounds and the x-height."""
    font = TTFont(font_path)
    if "fvar" in font:
        font = instantiateVariableFont(font, {"wght": 600})
    upm = font["head"].unitsPerEm
    gs = font.getGlyphSet()
    cmap = font.getBestCmap()
    k = size / upm
    pen, bounds = SVGPathPen(gs), BoundsPen(gs)
    x = 0.0
    for ch in text:
        name = cmap[ord(ch)]
        gs[name].draw(TransformPen(pen, (k, 0, 0, -k, x, 0)))
        gs[name].draw(TransformPen(bounds, (k, 0, 0, -k, x, 0)))
        x += gs[name].width * k + tracking * size
    return pen.getCommands(), bounds.bounds, font["OS/2"].sxHeight * k


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--font", required=True, help="Outfit variable font (.woff2 or .ttf)")
    a = ap.parse_args()

    for name, fill in (("blue", BLUE), ("lime", LIME), ("white", WHITE), ("ink", INK)):
        write(os.path.join(ASSETS, f"anchor-mark-{name}.svg"), svg(f"0 0 {MARK} {MARK}", rects(squares(MARK), fill), "Anchor"))
    write(os.path.join(ASSETS, "anchor-icon.svg"), svg("0 0 64 64", icon_body(64, BLUE, WHITE), "Anchor"))
    write(os.path.join(ASSETS, "anchor-icon-dark.svg"), svg("0 0 64 64", icon_body(64, LIME, NIGHT), "Anchor"))
    write(os.path.join(ASSETS, "anchor-icon-square.svg"), svg("0 0 64 64", icon_body(64, BLUE, WHITE, radius=False), "Anchor"))
    raster(os.path.join(ASSETS, "anchor-icon.svg"), os.path.join(ASSETS, "anchor-icon-512.png"), 512)
    raster(os.path.join(ASSETS, "anchor-icon.svg"), os.path.join(ASSETS, "anchor-icon-192.png"), 192)

    # Lockup: icon of height H, gap 0.27 H, type 0.72 H, x-height centred on the icon (the Echopoint rule).
    d, (bx0, by0, bx1, by1), xh = wordmark(a.font)
    x0 = 64 + 0.27 * 64 - bx0
    base = 32 + xh / 2
    width = x0 + bx1
    for name, tile_fill, mark_fill, ink in (("light", BLUE, WHITE, INK), ("dark", LIME, NIGHT, WHITE)):
        body = icon_body(64, tile_fill, mark_fill) + f'<path transform="translate({x0:.2f} {base:.2f})" d="{d}" fill="{ink}"/>'
        write(os.path.join(ASSETS, f"anchor-lockup-{name}.svg"), svg(f"0 0 {width:.2f} 64", body, "Anchor"))
    for name, ink in (("ink", INK), ("white", WHITE)):
        vb = f"{bx0:.2f} {by0:.2f} {bx1 - bx0:.2f} {by1 - by0:.2f}"
        write(os.path.join(ASSETS, f"anchor-wordmark-{name}.svg"), svg(vb, f'<path d="{d}" fill="{ink}"/>', "Anchor"))

    # App files. The favicon follows the browser's colour scheme; logo.svg is the blue mark for the app's <img> uses.
    style = (f"<style>.t{{fill:{BLUE}}}.m{{fill:{WHITE}}}"
             f"@media (prefers-color-scheme:dark){{.t{{fill:{LIME}}}.m{{fill:{NIGHT}}}}}</style>")
    m = 64 * FAVICON_FILL
    fav = style + f'<rect class="t" width="64" height="64" rx="{64 * TILE_RADIUS:.2f}"/>' + rects(squares(m, (64 - m) / 2, (64 - m) / 2), cls="m")
    write(os.path.join(PUBLIC, "favicon.svg"), svg("0 0 64 64", fav))
    write(os.path.join(PUBLIC, "logo.svg"), svg(f"0 0 {MARK} {MARK}", rects(squares(MARK), BLUE), "Anchor"))
    tmp = os.path.join(PUBLIC, ".favicon-light.svg")
    with open(tmp, "w") as f:
        f.write(svg("0 0 64 64", icon_body(64, BLUE, WHITE, FAVICON_FILL)))
    pngs = []
    for px in (16, 32, 48):
        p = os.path.join(PUBLIC, f".favicon-{px}.png")
        subprocess.run(["rsvg-convert", "-w", str(px), "-h", str(px), "-o", p, tmp], check=True)
        pngs.append(p)
    subprocess.run(["magick", *pngs, os.path.join(PUBLIC, "favicon.ico")], check=True)
    print("wrote anchor-ui/public/favicon.ico")
    raster(os.path.join(ASSETS, "anchor-icon-square.svg"), os.path.join(PUBLIC, "apple-touch-icon.png"), 180)
    for p in [tmp, *pngs]:
        os.remove(p)


if __name__ == "__main__":
    main()
