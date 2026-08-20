# cmd/ssh-mcp

The `main` package and entry point for the `ssh-mcp` binary.

## Purpose

This package implements the CLI front end of ssh-mcp. It parses command-line
flags (or a TOML config file path), loads the validated configuration, and
starts the MCP stdio server. It also provides the `install` subcommand that
registers ssh-mcp into a supported AI client's MCP config file.

## Contents

- `main.go` – the root `cobra.Command`. Defines all single-server and
  transport/security flags, sets up stderr logging (stdout is reserved for
  MCP stdio), loads configuration from a TOML file or from CLI flags,
  resolves SSH config aliases, and runs the MCP server via `internal/mcp`.
- `install.go` – the `install` subcommand. Mirrors the root command's
  config/single-server flags, resolves its own executable path, builds an
  `install.Entry` (sensitive values go into the env map, not args), and
  delegates to `internal/install.Install`. Supports `--dry-run`. Also
  supports a `--docker` mode that registers a `docker run` entry instead of
  a direct executable, with an optional `--docker-image` override.

## Integration with the project

- Imports `internal/config` (`Load`, `FromServer`), `internal/mcp` (`New`,
  `Run`), `internal/sshconfig` (`Lookup` for alias resolution), and
  `internal/install` (`Install`, `SupportedClients`, `RootKey`).
- The binary produced from this package is what gets packaged into `.mcpb`
  bundles (see `mcpb/`) and registered by the `install` subcommand.
- Built by the root `Makefile` (`make build`) and by `goreleaser.yml` for
  release artifacts.

## Notes

- `version` is injected at build time via `-ldflags` and defaults to `dev`.
- Logging always goes to stderr; stdout must stay clean because the MCP
  protocol uses it for stdio framing.
- Password and passphrase fall back to the `SSH_MCP_PASSWORD` and
  `SSH_MCP_PASSPHRASE` environment variables when the corresponding flags
  are empty. This is how MCPB and other launchers inject sensitive values
  without exposing them on the command line.
- SSH config alias resolution only runs in single-server mode when the host
  is not an IP address and does not contain a dot, and only when
  `--ssh-config` is provided.
- The `install` subcommand accepts `--docker` to register a container-based
  entry instead of a direct executable. The entry uses `docker run -i --rm`
  with the host home directory bind-mounted at the same path (so `~/`
  references in the TOML resolve identically), the SSH agent socket
  forwarded when `SSH_AUTH_SOCK` is set, and sensitive values passed via
  `-e` from the host environment. The image defaults to
  `ghcr.io/overklassniy/ssh-mcp:latest` and can be overridden with
  `--docker-image`. Example:
  ```
  ssh-mcp install --client claude-code --docker --config ./ssh-mcp.toml --dry-run
  ```
