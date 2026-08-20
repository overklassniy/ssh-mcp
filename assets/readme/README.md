# assets/readme

Visual assets used by the project's root README homepage.

## Purpose

This folder holds the SVG and raster visuals embedded in the root
`README.md`. It is the README's image layer; the root `README.md` is the
content layer. Source-of-truth user documentation lives in `docs/`, not
here.

## Contents

- `hero.svg` – the editable source for the project hero. Light
  technical layout (GitHub light theme palette): project name,
  one-line value, mono metadata, and a terminal/connection motif (an
  `ssh-mcp $ execute-command` prompt with a blinking cursor, plus a
  connection line drawn from an `agent` node to a `remote` node through
  a `policy` gate). Carries SMIL entrance animation that plays in
  browsers; static renderers (including GitHub's SVG sanitizer) show
  the settled composition, so it doubles as the static fallback.
- `hero.gif` – the GitHub-safe animated hero embedded at the top of the
  root `README.md`. Rendered from `hero.svg` via `hero-motion.json` so
  the entrance sequence plays directly on GitHub, which strips SVG
  animation. 1200x360, 5-second loop.
- `hero-motion.json` – the motion spec used to render `hero.gif`:
  per-layer enter/exit timing for the title, terminal, connection draw,
  and policy gate.
- `social-preview.svg` – the editable source for the GitHub social
  preview image. Terminal/connection motif recomposed for the 2:1
  social preview ratio (1280x640). Note: this asset still uses the
  earlier dark palette and was not part of the hero relight.
- `social-preview.png` – the rendered 1280x640 PNG that GitHub expects
  as the repository social preview. Generated from
  `social-preview.svg` with Inkscape.
- `dockerhub-logo.svg` – the editable source for the Docker Hub
  repository logo. Square 512x512 mark using the same light palette as
  the hero: project name, a terminal prompt, and a simplified
  agent-to-remote connection line through a policy gate.
- `dockerhub-logo.png` – the rendered 512x512 PNG that Docker Hub
  expects as the repository logo. Generated from `dockerhub-logo.svg`
  with Inkscape.

## Integration with the project

- The root `README.md` embeds `hero.gif` via an HTML align block with
  `width="100%"` and descriptive alt text; `hero.svg` is kept as the
  editable source and static fallback.
- Uses only GitHub-safe SVG features: paths, shapes, text, lines, and
  simple transforms. No `<script>`, no `foreignObject`, no remote fonts,
  no remote image URLs. The hero's SMIL animation is browser-only;
  GitHub strips it, which is why the animated GIF is embedded instead.
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

## Docker Hub logo upload

Docker Hub does not expose a logo upload API, so the logo cannot be
synced by the `dockerhub-sync` workflow. Upload `dockerhub-logo.png`
manually once:

1. Go to Docker Hub > overklassniy/ssh-mcp > Settings.
2. Under Logo, upload `dockerhub-logo.png` (512x512 PNG).
3. Save.

The SVG is kept as the editable source. To regenerate the PNG after
editing the SVG:

```sh
inkscape --export-type=png --export-filename=dockerhub-logo.png \
  --export-width=512 --export-height=512 dockerhub-logo.svg
```

## Hero GIF regeneration

The animated GIF is rendered from `hero.svg` plus `hero-motion.json`
with the `beautify-github-readme` skill's render script. It needs
`ffmpeg`, Pillow, and an `rsvg-convert`-compatible SVG rasterizer
(`rsvg-convert`, `sips`, or an Inkscape CLI shim exposing the same
`<input.svg> -o <output.png>` interface).

```sh
python3 .devin/skills/beautify-github-readme/scripts/render_motion_gif.py \
  assets/readme/hero.svg assets/readme/hero.gif \
  --spec assets/readme/hero-motion.json
```

Keep `hero.svg` as the static fallback and editable source; only
re-embed `hero.gif` in the root `README.md` after regenerating it.
