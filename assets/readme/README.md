# assets/readme

Visual assets used by the project's root README homepage.

## Purpose

This folder holds the SVG and raster visuals embedded in the root
`README.md`. It is the README's image layer; the root `README.md` is the
content layer. Source-of-truth user documentation lives in `docs/`, not
here.

## Contents

- `hero.svg` - the project-native hero shown at the top of the root
  `README.md`. Monochrome technical layout: project name, one-line
  value, mono metadata, and a terminal/connection motif (an
  `ssh-mcp $ execute-command` prompt with a cursor, plus a thin ruled
  line from an `agent` node to a `remote` node passing through a
  `policy` gate). Dark canvas so it reads on both GitHub light and dark
  backgrounds.
- `social-preview.svg` - the editable source for the GitHub social
  preview image. Same dark palette and terminal/connection motif as the
  hero, recomposed for the 2:1 social preview ratio (1280x640).
- `social-preview.png` - the rendered 1280x640 PNG that GitHub expects
  as the repository social preview. Generated from
  `social-preview.svg` with Inkscape.

## Integration with the project

- Embedded by the root `README.md` via an HTML align block with
  `width="100%"` and descriptive alt text.
- Uses only GitHub-safe SVG features: paths, shapes, text, lines, and
  simple transforms. No `<script>`, no `foreignObject`, no remote fonts,
  no essential animation, no remote image URLs.
- System font stack only (`-apple-system`, `BlinkMacSystemFont`,
  `Segoe UI`, `PingFang SC`, `sans-serif`, and a monospace stack for the
  motif tokens), so it renders consistently without bundled fonts.

## Conventions

- Lowercase hyphenated filenames.
- One `1200`-unit-wide `viewBox` for full-width modules; `width="100%"`
  embeds.
- Every visual module includes `<title>` and `<desc>` for accessibility.
- Keep copy that changes often (commands, links, version numbers that
  move fast) out of SVG; the hero only carries stable identity metadata.

## Notes

- Preview full-width assets at a conservative `900` CSS-pixel GitHub
  render width and at a `360`-pixel mobile width before publishing.
- Discarded variants should be removed unless explicitly retained as
  source explorations.

## Social preview upload

GitHub does not read the social preview from the repository. To set it,
upload `social-preview.png` manually:

1. Go to the repository Settings > Social preview.
2. Upload `social-preview.png` (1280x640 PNG).
3. Save.

The SVG is kept as the editable source. To regenerate the PNG after
editing the SVG:

```sh
inkscape --export-type=png --export-filename=social-preview.png \
  --export-width=1280 --export-height=640 social-preview.svg
```
