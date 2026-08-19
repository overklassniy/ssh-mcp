# mcpb

MCPB (MCP Bundle) packaging source for ssh-mcp.

## Purpose

This folder holds the `manifest.json` and packaging instructions used to
build a `.mcpb` bundle - a zip archive containing the ssh-mcp binary and a
manifest that describes the server, its tools, and the user-configurable
SSH options. MCPB is the cross-client standard for one-click installation
of local MCP servers, supported by Claude Desktop, Claude Code, and MCP for
Windows.

## Contents

- `manifest.json` - the MCPB manifest (spec v0.3). Declares ssh-mcp as a
  `binary` server type with `user_config` for SSH host, port, username,
  password, private key, passphrase, agent, and keyboard-interactive (2FA)
  toggle. Sensitive values (password, passphrase) are passed via
  environment variables rather than CLI args.
- `.mcpbignore` - files excluded from the bundle when packaging.

## How the bundle is built

The `make mcpb` target (see the root `Makefile`) builds a per-platform
binary, places it at `server/ssh-mcp` (or `server/ssh-mcp.exe` on Windows)
inside a staging directory alongside `manifest.json`, and zips the staging
directory into `dist/ssh-mcp-<os>-<arch>.mcpb`.

A resulting bundle has this layout:

```
ssh-mcp-<os>-<arch>.mcpb (zip)
  manifest.json
  server/
    ssh-mcp        (or ssh-mcp.exe on Windows)
```

## Integration with the project

- The binary is built from `cmd/ssh-mcp` and is self-contained (Go
  statically links its dependencies).
- The manifest's `mcp_config.args` reference the single-server CLI flags
  defined in `cmd/ssh-mcp/main.go`. The `--try-keyboard` flag uses the
  `--flag=value` form because cobra bool flags do not accept a separate
  value argument.
- The `SSH_MCP_PASSWORD` and `SSH_MCP_PASSPHRASE` environment variables are
  read as fallbacks in `cmd/ssh-mcp/main.go`, which is how the manifest
  injects sensitive `user_config` values without exposing them on the
  command line.

## Notes

- MCPB one-click install is currently supported by Claude Desktop, Claude
  Code, and MCP for Windows. Cursor, Devin, and VS Code users should use
  `ssh-mcp install` or the JSON snippets in the root README instead.
- Platform-specific bundles are produced per OS/architecture; the manifest
  declares `compatibility.platforms: ["win32", "darwin", "linux"]` and uses
  a `platform_overrides` entry to select the `.exe` binary on Windows.
