# internal/ssh

SSH connection management, command execution, SFTP transfers, and port
forwarding for ssh-mcp.

## Purpose

This package is the core SSH layer of ssh-mcp. It manages connections to
multiple named servers, executes commands in exec or persistent-shell mode,
performs SFTP file transfers with path validation, and manages local and
remote port forwarding. It also defines the typed error contract (`ToolError`
with stable codes) that the MCP tools surface to agents.

## Contents

- `client.go` – the `SSHClient` interface and `realClient` implementation.
  Handles connection establishment (direct or via proxy), keepalive
  goroutine, and mutex-protected access to the underlying `*ssh.Client`.
  Also defines `ExecResult`.
- `manager.go` – `ConnectionManager` for multiple named servers. Supports
  lazy connection, concurrent-attempt deduplication, reconnection with
  exponential backoff, parallel `ConnectAll`, graceful `Disconnect`, and
  `Invalidate` to force a reconnect on next use.
- `auth.go` – `BuildAuthMethods` constructs SSH auth methods in order:
  private key, agent, password, keyboard-interactive. Only configured
  methods are included.
- `exec.go` – `ExecCommand` runs a command in exec mode. Applies the command
  template, validates against the policy, allocates a PTY when enabled, caps
  output, enforces timeout, and extracts exit code and signal.
- `shell.go` – persistent shell session mode. Uses random marker lines to
  frame each command and capture its output and exit code, with a
  ready-detection handshake and per-session mutex.
- `sftp.go` – `SFTPClient` wraps `github.com/pkg/sftp` with local and remote
  path validation and concurrent transfer support for uploads, downloads,
  directory sync, and remote listings.
- `forward.go` – `ForwardManager` for local and remote port forwarding with
  open, close, and list actions; tracks active forwards per connection.
- `proxy.go` – `dialProxy` dials a target through a SOCKS5, HTTP CONNECT, or
  HTTPS CONNECT proxy.
- `errors.go` – `ToolError` type, `Code` constants (stable strings such as
  `SSH_CONNECTION_FAILED`, `COMMAND_TIMEOUT`), `NewToolError`, `AsToolError`,
  and `CodeFromError`.
- `shell_test.go`, `integration_test.go`,
  `parallel_integration_test.go` – unit and integration tests.

## Integration with the project

- Created by `internal/mcp.New`, which passes the loaded `*config.Config` to
  `NewConnectionManager` and registers the MCP tools that call into it.
- Depends on `internal/config` (server configs, timeouts), `internal/policy`
  (command validation), and `internal/paths` (SFTP path validation).
- Uses `golang.org/x/crypto/ssh` for the SSH protocol,
  `github.com/pkg/sftp` for SFTP, and `golang.org/x/net/proxy` for SOCKS5.

## Notes

- `HostKeyCallback` is set to `ssh.InsecureIgnoreHostKey`. This is a known
  caveat: host key verification is not performed. Operators in
  security-sensitive environments should be aware of this before deploying.
- The keepalive goroutine sends `keepalive@openssh.com` global requests at
  `keepalive_interval` and closes the client after `keepalive_count_max`
  consecutive failures, forcing a reconnect on next use.
- `ToolError` carries a stable `Code` and a `Retriable` flag so agent-side
  retry logic can decide whether to retry a failed operation. The error
  codes mirror the original TypeScript implementation to keep that contract
  stable.
- All access to the underlying `*ssh.Client` is guarded by a read/write mutex
  to prevent data races between keepalive, close, session creation, and
  connection checks.
