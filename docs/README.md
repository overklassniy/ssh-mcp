# docs

Source of truth for the ssh-mcp user documentation.

## Purpose

This folder holds the Markdown source for the project's deep
documentation. The root `README.md` is a short quickstart; everything
here is the full reference. The contents are mirrored to the GitHub
Wiki by the `wiki-sync` GitHub Actions workflow (see
`.github/workflows/wiki-sync.yml`).

## How the wiki sync works

On every push to `main` that touches `docs/**`, the `wiki-sync` workflow
copies the contents of this folder to `overklassniy/ssh-mcp.wiki.git`
using the `Andrew-Chen-Wang/github-wiki-action` action. The wiki tab on
GitHub then renders the same Markdown you see here.

Consequences for editing:

- Edit files here, not in the wiki UI. Edits made in the wiki UI are
  overwritten on the next push to `main` that touches `docs/`.
- `Home.md` becomes the wiki landing page (GitHub wikis use `Home.md`,
  not `README.md`, for the homepage).
- `_Sidebar.md` controls the wiki left navigation shown on every page.
- `README.md` (this file) is excluded from the sync by the workflow's
  `ignore` input, because it only describes the folder locally and is
  not wiki content.

## Contents

- `Home.md` - wiki landing page. Project overview and entry point to the
  rest of the documentation.
- `_Sidebar.md` - wiki navigation sidebar, shown on every wiki page.
- `Architecture.md` - package layout, dependency graph, and request data
  flow from an MCP tool call to a remote SSH command.
- `Installation.md` - full installation guide covering all supported AI
  clients, Docker mode, and MCPB one-click bundles.
- `Configuration.md` - complete TOML configuration reference: the
  `[defaults]` section, `[[server]]` entries, auth methods, policy,
  allowed paths, keepalive, and transport modes.
- `Tools.md` - reference for the eight MCP tools: arguments, return
  values, and examples.
- `Troubleshooting.md` - common issues and their fixes.

## Integration with the project

- The root `README.md` links into this folder for deep topics and points
  at the GitHub Wiki for the rendered version.
- The `wiki-sync` workflow in `.github/workflows/` reads from this
  folder and pushes to the wiki git backend.
- Source-level documentation (per-package `README.md` files under
  `cmd/`, `internal/`, `mcpb/`) describes implementation details for
  contributors; this folder describes user-facing behavior.

## Notes

- One-time manual prerequisite: before the first workflow run, create a
  dummy page in the wiki via the GitHub web UI. This initializes the
  `.wiki.git` backend that the workflow pushes to. Without it, the
  workflow fails because the wiki repo does not exist yet.
- Links to source files (for example `cmd/ssh-mcp/main.go`) are
  rewritten by the sync action to absolute `blob/` URLs pinned to the
  pushed commit, so they work both in this folder on GitHub and inside
  the rendered wiki.
- The sync uses `strategy: clone`, which appends a commit to the wiki
  repo history. It does not force-push.
