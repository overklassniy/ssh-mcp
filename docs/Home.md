# ssh-mcp

An MCP (Model Context Protocol) server that gives AI assistants secure,
scoped SSH access to one or more remote servers.

ssh-mcp runs over stdio and works with any MCP-compatible client:
Claude Desktop, Claude Code, Cursor, Devin, VS Code, and Cline. It
exposes eight tools for command execution, SFTP file transfer, directory
sync, port forwarding, and remote system status collection.

## Why

AI assistants that need to operate on remote servers typically either
get a raw shell (unsafe, unscoped) or have no SSH access at all. ssh-mcp
sits in between: it gives the agent a small, auditable set of tools,
each gated by an optional command whitelist/blacklist and path
restrictions, so the operator controls exactly what the agent can do.

## Where to start

- New to the project? Read [Installation](Installation) first, then
  [Configuration](Configuration).
- Want to understand the internals? Read [Architecture](Architecture).
- Looking for what a specific tool does? See [Tools](Tools).
- Something went wrong? See [Troubleshooting](Troubleshooting).

## Quickstart

1. Build the binary: `make build`.
2. Create a minimal `ssh-mcp.toml` (see [Configuration](Configuration)).
3. Register it in your client:
   `ssh-mcp install --client claude-code --config ./ssh-mcp.toml`.
4. Restart your AI client and ask it to run a command on your server.

## Tools at a glance

| Tool | Description |
| --- | --- |
| `execute-command` | Run a command on a remote server |
| `upload` | Upload a local file via SFTP |
| `download` | Download a remote file via SFTP |
| `list-remote` | List a remote directory via SFTP |
| `dir-sync` | Recursively sync a directory between local and remote |
| `port-forward` | Open, close, and list local/remote port forwards |
| `server-status` | Collect system status (CPU, memory, disk, GPU, services) |
| `list-servers` | List configured servers and their connection status |

See [Tools](Tools) for arguments and examples.

## Security model

ssh-mcp does not verify remote host keys (`InsecureIgnoreHostKey`).
Security is enforced through two mechanisms the operator configures:

- **Command policy**: `whitelist` and `blacklist` regex patterns gate
  every command before execution. Without a whitelist, all commands are
  allowed and a warning is logged at startup.
- **Path restrictions**: `allowed_local_paths` and `allowed_remote_paths`
  confine SFTP transfers to specific directories and reject path
  traversal.

Sensitive values (password, passphrase, 2FA code) are passed via
environment variables, never on the command line.
