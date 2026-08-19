# Architecture

This page describes the package layout of ssh-mcp, the dependency graph
between packages, and the data flow of a single MCP tool call from the
AI client to a remote SSH command.

## Package layout

```
ssh-mcp/
├── cmd/ssh-mcp/          entry point: CLI parsing, config loading, install subcommand
├── internal/
│   ├── config/           TOML schema, loading, defaults, validation, SSH config resolution
│   ├── mcp/              MCP stdio server wiring
│   │   └── tools/        the eight MCP tool implementations
│   ├── ssh/              SSH connection manager, exec, shell, SFTP, port forwarding
│   ├── policy/           command whitelist/blacklist regex validation
│   ├── paths/            local and remote path validation (traversal prevention)
│   ├── sshconfig/        OpenSSH ~/.ssh/config parser and alias resolver
│   └── status/           batched remote system status collection
└── mcpb/                 MCPB bundle manifest and packaging source
```

All logic lives under `internal/`, so none of it can be imported by
modules outside `github.com/overklassniy/ssh-mcp`. The only public
entry point is the `ssh-mcp` binary built from `cmd/ssh-mcp`.

## Dependency graph

```
cmd/ssh-mcp
├── internal/config
│   └── internal/sshconfig
├── internal/mcp
│   ├── internal/config
│   ├── internal/ssh
│   │   ├── internal/config
│   │   ├── internal/policy
│   │   └── internal/paths
│   └── internal/mcp/tools
│       ├── internal/ssh
│       └── internal/status
│           └── internal/policy
└── internal/install
```

Key properties:

- `cmd/ssh-mcp` is the only package that imports `install` and `mcp`.
- `mcp` depends on `config`, `ssh`, and `mcp/tools`.
- `ssh` depends on `config`, `policy`, and `paths`.
- `status` depends on `policy` (each status probe is policy-validated
  before being batched, so a restrictive whitelist cannot be bypassed).
- `config` depends on `sshconfig` for `~/.ssh/config` alias resolution.
- No package imports `cmd/ssh-mcp`; the dependency graph points inward.

## Data flow of a tool call

This traces an `execute-command` tool call from the AI client to the
remote server and back.

1. **Client sends an MCP request.** The AI client (Claude Desktop,
   Cursor, etc.) sends a JSON-RPC `tools/call` message over the ssh-mcp
   process's stdin. The message names the tool and includes arguments
   such as `command` and `connectionName`.

2. **mcp-go dispatches to the tool handler.** `internal/mcp` created an
   `mcp-go` stdio server at startup and registered all eight tools via
   `tools.RegisterAll`. mcp-go parses the JSON-RPC frame and calls the
   handler registered for `execute-command`.

3. **Tool handler resolves the connection.** The handler in
   `internal/mcp/tools/execute.go` reads the optional `connectionName`
   argument (defaulting to the first configured server) and asks the
   `ssh.ConnectionManager` for a client for that server.

4. **Connection manager connects (lazily).** If no connection exists
   yet, `ConnectionManager` dials the server using the auth methods from
   `BuildAuthMethods` (private key, agent, password,
   keyboard-interactive, in that order). Connections can go direct or
   through a SOCKS5/HTTP CONNECT proxy. A keepalive goroutine sends
   `keepalive@openssh.com` requests and closes the client after
   `keepalive_count_max` consecutive failures, forcing a reconnect on
   next use.

5. **Command is validated and executed.** `ExecCommand` in
   `internal/ssh/exec.go` applies the command template, validates the
   command against the server's `policy.Policy` (whitelist then
   blacklist), optionally allocates a PTY, runs the command with the
   configured timeout, caps output at `max_output_bytes`, and extracts
   the exit code and any signal.

6. **Result returns up the stack.** `ExecResult` (stdout, stderr, exit
   code, duration) travels back through the tool handler, which wraps it
   as an MCP tool response. If an error occurred, `errorResult`
   preserves the stable `ssh.ToolError` code and retriable flag so the
   agent can decide whether to retry.

7. **Response goes to the client.** mcp-go writes the JSON-RPC response
   to stdout. The AI client receives it and presents the result to the
   user or acts on it.

## Transport modes

Each server configures a `transport` of either `exec` or `shell`:

- **exec** (default): each command runs in its own SSH session via
  `session.Run`. Stateless, simple, slightly higher per-command overhead.
- **shell**: a persistent shell session is kept open per server.
  Commands are framed by random marker lines so their output and exit
  code can be extracted from the stream. Lower per-command overhead and
  preserves shell state (environment, working directory) across
  commands.

## Startup and shutdown

`internal/mcp.New` constructs the `Server` (wrapping an `*MCPServer` and
a `*ConnectionManager`) and registers all tools. `Server.Run`:

1. Sets up SIGINT/SIGTERM handlers.
2. If any server has `pre_connect=true`, calls `ConnectAll` to open all
   connections in parallel. Individual failures are logged as warnings,
   not fatal.
3. Serves the MCP protocol over stdio (stdout for protocol, stderr for
   logging).
4. On shutdown signal, disconnects all connections gracefully.

stdout is reserved for MCP stdio framing. Anything else written to
stdout breaks the transport. All logging uses `log/slog` to stderr.

## Error contract

`internal/ssh/errors.go` defines `ToolError` with a stable string `Code`
(for example `SSH_CONNECTION_FAILED`, `COMMAND_TIMEOUT`) and a
`Retriable` flag. The tool handlers convert errors via `errorResult` so
the agent-side retry logic can inspect the code and decide whether to
retry. The codes mirror the original TypeScript implementation to keep
that contract stable across the rewrite.
