# internal/mcp/tools

Implementations of the MCP tools exposed by ssh-mcp.

## Purpose

This package registers the eight MCP tools that ssh-mcp exposes to AI
assistants. Each tool is a thin adapter that parses MCP tool arguments,
delegates to the `ssh.ConnectionManager` (and, for status, to
`internal/status`), and returns the result as an MCP tool response.

## Contents

- `tools.go` - `RegisterAll` registers all eight tools on the given
  `*server.MCPServer`; shared helpers `commonConnectionNameArg` (the optional
  `connectionName` argument) and `errorResult` (converts an error into an
  MCP error result, preserving `ssh.ToolError` codes).
- `execute.go` - the `execute-command` tool. Runs a command on a remote
  server via SSH and returns stdout, stderr, exit code, and duration.
- `upload.go` - the `upload` tool. Uploads a local file to a remote server
  via SFTP, validating local and remote paths against configured allowed
  paths.
- `download.go` - the `download` tool. Downloads a remote file to the local
  machine via SFTP using a temp file plus atomic rename, validating paths.
- `listremote.go` - the `list-remote` tool. Lists the contents of a remote
  directory via SFTP, returning structured file entries (name, size, mode,
  modTime, isDir).
- `listservers.go` - the `list-servers` tool. Lists all configured SSH
  servers with their connection status.
- `serverstatus.go` - the `server-status` tool. Collects system status
  (hostname, OS, CPU, memory, disk, GPUs, processes, services) via a single
  batched SSH command, delegating to `internal/status`.
- `dirsync.go` - the `dir-sync` tool. Recursively syncs a directory between
  local and remote via SFTP in `upload` or `download` direction with
  concurrent file transfers.
- `portforward.go` - the `port-forward` tool. Opens, closes, and lists local
  and remote port forwards. Holds a per-connection `*ssh.ForwardManager` map
  guarded by a mutex.

## Integration with the project

- Registered onto the `*server.MCPServer` by `internal/mcp.New` via
  `RegisterAll`.
- Each tool calls into `internal/ssh.ConnectionManager` to obtain a client
  and perform the underlying operation.
- `serverstatus.go` additionally depends on `internal/status` for the
  batched status probe logic.

## Notes

- All tools share an optional `connectionName` argument that defaults to the
  first configured server when omitted.
- Errors are returned as MCP error results via `errorResult`, which preserves
  the stable `ssh.ToolError` code and retriable flag so agent-side retry
  logic can inspect them.
- `portforward.go` keeps package-level state (the `forwardManagers` map) so
  that forwards persist across tool calls within a single server process.
