# internal/mcp

Wires the SSH connection manager to the MCP protocol server.

## Purpose

This package bridges the ssh-mcp SSH layer and the Model Context Protocol. It
creates an `mcp-go` stdio server, constructs the `ssh.ConnectionManager` from
the loaded configuration, registers all MCP tools, and runs the server until
a shutdown signal is received.

## Contents

- `server.go` – the `Server` type wrapping an `*server.MCPServer` and a
  `*ssh.ConnectionManager`. `New` takes a `*config.Config` and an optional
  config file path; when the path is non-empty, `Run` starts a
  `config.Watcher` that reloads the config file on change and calls
  `manager.Reload` without restarting the server. `Run` sets up
  SIGINT/SIGTERM handling, optionally pre-connects to all servers when any
  server has `pre_connect=true`, serves the MCP protocol over stdio, and
  disconnects cleanly on exit. `Manager` exposes the connection manager
  for testing.
- `tools/` – the implementations of the individual MCP tools.

## Integration with the project

- Created by `cmd/ssh-mcp` (`mcp.New(cfg, configPath)` followed by
  `srv.Run()`).
- Depends on `internal/config` (for the `*config.Config` and
  `config.Watcher`), `internal/ssh` (for `ConnectionManager`), and
  `internal/mcp/tools` (for tool registration).
- Uses `github.com/mark3labs/mcp-go` for the MCP protocol and stdio
  transport.

## Notes

- stdout is reserved for MCP stdio framing; all logging goes to stderr via
  `log/slog`. Writing anything other than protocol messages to stdout breaks
  the transport.
- Pre-connect is opt-in: it only runs when at least one server has
  `pre_connect=true`, and individual connection failures are logged as
  warnings rather than aborting startup.
- Hot-reload is opt-in: it only runs when a config file path is provided
  (file-based config via `--config`). In single-server CLI mode, no
  watcher is started.
- The `var _ = mcp.NewTool` line keeps the `mcp` import used even when the
  only direct reference is inside `tools/`.
