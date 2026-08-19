# internal/install

Package `install` writes ssh-mcp server entries into the MCP configuration
files of supported AI clients.

## Purpose

Each AI client stores its MCP server list in a JSON file at a different
location and, in the case of VS Code, under a different root key. This
package centralizes the per-client path and root-key mapping so the
`ssh-mcp install` subcommand can register ssh-mcp in any supported client
with a single command, merging non-destructively alongside existing
servers.

## Contents

- `install.go` - client spec table, path resolution, JSON merge logic, the
  `Install` entry point, and the `DockerEntry` builder for container-based
  installs.
- `install_test.go` - unit tests covering merge semantics, the VS Code
  `servers` quirk, backup creation, parent-dir creation, per-OS path
  resolution, and the docker entry shape (config mode, agent socket,
  default image, minimal entry).

## Supported clients

| Client | Root key | Config path (per OS) |
| --- | --- | --- |
| claude-desktop | `mcpServers` | `%APPDATA%\Claude\claude_desktop_config.json` (Win), `~/Library/Application Support/Claude/claude_desktop_config.json` (mac), `~/.config/Claude/claude_desktop_config.json` (Linux) |
| claude-code | `mcpServers` | `~/.claude/settings.json` |
| cursor | `mcpServers` | `~/.cursor/mcp.json` |
| Devin | `mcpServers` | `~/.codeium/Devin/mcp_config.json` |
| vscode | `servers` | `~/.vscode/mcp.json` |
| cline | `mcpServers` | `.../Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` |

VS Code is the only major client that uses `servers` instead of
`mcpServers`. Copying a config from another client into VS Code without
this difference will silently fail.

## Docker install mode

In addition to the direct-executable install, the package provides a
`DockerEntry` builder that produces an `Entry` launching ssh-mcp inside a
container via `docker run -i --rm`. The `--client` flag still selects which
agent config file is written; only the entry shape changes.

`DockerEntry` accepts a `DockerEntryOptions` struct with:

- `Image` - container image (defaults to `DefaultDockerImage`, the GHCR
  primary registry).
- `ConfigPath` - host-side TOML config path, mounted read-only at
  `/config.toml` inside the container.
- `Home` - host home directory, bind-mounted at the same absolute path so
  that `~/` paths in the config resolve identically inside the container.
- `AgentSocket` - host SSH agent socket path (value of `$SSH_AUTH_SOCK`),
  mounted at the same path with `SSH_AUTH_SOCK` forwarded.
- `ExtraArgs` - additional ssh-mcp CLI flags appended after the image name
  (used in single-server mode).

Sensitive values (`SSH_MCP_PASSWORD`, `SSH_MCP_PASSPHRASE`,
`SSH_MCP_2FA_CODE`) are forwarded from the host environment via `-e` and
never appear in the args.

### Path remapping

Mounting the host home directory at the same absolute path means `~/...`
references in the TOML config work unchanged inside the container. Local
file upload/download paths outside the home directory require the user to
add extra `-v` mounts to the generated entry, because the container
filesystem is otherwise isolated.

### Platform notes

Docker install mode is best supported on Linux, where bind mounts and unix
sockets map directly. On macOS and Windows (Docker Desktop), host paths are
translated through a VM layer and absolute paths differ; some manual `-v`
adjustment may be needed. SSH agent forwarding via a unix socket works on
Linux and macOS but not on Windows (named pipes are not forwarded by
Docker Desktop).

## Integration with the project

- The `ssh-mcp install` subcommand in `cmd/ssh-mcp/install.go` calls
  `install.Install` with an `Entry` built from the resolved executable path
  and the user-provided config/flags.
- The merge logic preserves all existing top-level keys and server entries,
  creates a `.bak` backup before overwriting an existing file, and creates
  parent directories as needed.

## Notes

- Path resolution uses `os.UserHomeDir()` and `%APPDATA%` on Windows. On
  macOS and Linux the app-support/config directory is derived from the home
  directory inside each client's `pathFor` function.
- `os.Executable()` is used by the CLI layer (not this package) to set
  `Entry.Command`, so `ssh-mcp install` must be run from the installed
  binary location for the command path to be correct.
