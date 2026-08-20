# ssh-mcp

An MCP (Model Context Protocol) server that gives AI assistants secure,
scoped SSH access to one or more remote servers.

`ssh-mcp` runs over stdio and works with any MCP-compatible client:
Claude Desktop, Claude Code, Cursor, Devin, VS Code, and Cline. It
exposes eight tools for command execution, SFTP file transfer, directory
sync, port forwarding, and remote system status collection.

![ssh-mcp hero](https://raw.githubusercontent.com/overklassniy/ssh-mcp/master/assets/readme/hero.gif)

## Quick start with Docker

```sh
docker pull overklassniy/ssh-mcp:latest
```

The image is multi-arch (`linux/amd64`, `linux/arm64`) and based on a
stripped, static distroless runtime (typically 15-18 MB).

### Available tags

| Tag | Description |
| --- | --- |
| `:latest` | Latest stable release |
| `:1.0.0` | Pinned version (replace with the release you want) |
| `:1.0` | Major.minor rolling tag |
| `:1` | Major rolling tag |
| `:dev` | Latest build from the master branch (not for production) |

### Run with a config file

Mount your `ssh-mcp.toml` and home directory (so `~/` paths resolve),
then let `ssh-mcp install` register the container in your client:

```sh
ssh-mcp install --client claude-code --docker --config ./ssh-mcp.toml
```

This writes a `docker run -i --rm` entry into your client's MCP config,
bind-mounting the host home directory at the same path, forwarding
`SSH_AUTH_SOCK` when set, and passing sensitive values via `-e` from the
host environment.

The image defaults to `ghcr.io/overklassniy/ssh-mcp:latest`. To use this
Docker Hub mirror instead, override it:

```sh
ssh-mcp install --client claude-code --docker \
  --docker-image overklassniy/ssh-mcp:latest \
  --config ./ssh-mcp.toml
```

### Run directly with docker run

```sh
docker run -i --rm \
  -v "$HOME:$HOME" \
  -v "$PWD/ssh-mcp.toml:/config.toml:ro" \
  -e SSH_AUTH_SOCK \
  -v "$SSH_AUTH_SOCK:$SSH_AUTH_SOCK" \
  overklassniy/ssh-mcp:latest --config /config.toml
```

Pass sensitive values via `-e`:

```sh
docker run -i --rm \
  -v "$HOME:$HOME" \
  -v "$PWD/ssh-mcp.toml:/config.toml:ro" \
  -e SSH_MCP_PASSWORD \
  overklassniy/ssh-mcp:latest --config /config.toml
```

### Platform notes for Docker mode

Docker install mode is best supported on Linux, where bind mounts and
unix sockets map directly. On macOS and Windows (Docker Desktop), host
paths are translated through a VM layer and absolute paths differ; some
manual `-v` adjustment may be needed. SSH agent forwarding via a unix
socket works on Linux and macOS but not on Windows (named pipes are not
forwarded by Docker Desktop).

Local file upload/download paths outside the home directory require
extra `-v` mounts, because the container filesystem is otherwise
isolated.

## Why it is different

AI assistants that need to operate on remote servers typically either
get a raw shell (unsafe, unscoped) or have no SSH access at all.
`ssh-mcp` sits in between: it gives the agent a small, auditable set of
tools, each gated by an optional command whitelist/blacklist and path
restrictions, so the operator controls exactly what the agent can do.

- **Command policy** - `whitelist` and `blacklist` regex patterns gate
  every command before execution. Without a whitelist, all commands are
  allowed and a warning is logged at startup.
- **Path restrictions** - `allowed_local_paths` and `allowed_remote_paths`
  confine SFTP transfers to specific directories and reject path
  traversal.
- **Secrets via env vars** - password, passphrase, and 2FA code are
  passed via environment variables, never on the command line.

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

See the [Tools docs](https://github.com/overklassniy/ssh-mcp/blob/master/docs/Tools.md) for arguments and examples.

## How it works

1. The AI client sends an MCP `tools/call` request over the `ssh-mcp`
   process's stdin.
2. `ssh-mcp` resolves the target connection (defaulting to the first
   configured server).
3. The command or transfer is validated against the server's command
   policy and allowed paths.
4. The operation runs over SSH/SFTP (or opens a port forward) with the
   configured timeout and output cap.
5. A structured result returns to the client. Errors carry a stable
   `ssh.ToolError` code and a `Retriable` flag so the agent can decide
   whether to retry.

See the [Architecture docs](https://github.com/overklassniy/ssh-mcp/blob/master/docs/Architecture.md) for the package layout,
dependency graph, and full request data flow.

## Installation

Three install paths produce the same result - an MCP server entry in
your client's config file. See the
[Installation guide](https://github.com/overklassniy/ssh-mcp/blob/master/docs/Installation.md)
for the full reference.

- **Local binary** - `make build`, then `ssh-mcp install --client <client>
  --config ./ssh-mcp.toml`. Supported clients: `claude-desktop`,
  `claude-code`, `cursor`, `Devin`, `vscode`, `cline`.
- **Docker** - `ssh-mcp install --client <client> --docker --config
  ./ssh-mcp.toml`. Defaults to `ghcr.io/overklassniy/ssh-mcp:latest`;
  override with `--docker-image overklassniy/ssh-mcp:latest` to use this
  Docker Hub mirror.
- **MCPB one-click bundle** - download the `.mcpb` bundle from the
  [Releases page](https://github.com/overklassniy/ssh-mcp/releases) and
  open it in a client that supports MCPB (Claude Desktop, Claude Code,
  MCP for Windows).

Single-server mode skips the TOML file and takes connection details as
CLI flags:

```sh
ssh-mcp --host example.com --username deploy --port 22 \
  --private-key ~/.ssh/id_ed25519
```

## Configuration

`ssh-mcp` is configured with a TOML file: a top-level `[defaults]`
section and one or more `[[server]]` entries. Each server inherits from
`[defaults],` then from built-in defaults, and is validated at startup.

```toml
[defaults]
username = "deploy"
port = 22
private_key = "~/.ssh/id_ed25519"
timeout = "30s"
transport = "exec"

[[server]]
name = "web"
host = "web.example.com"
whitelist = ["^systemctl status .*", "^journalctl .*", "^ls .*"]
allowed_remote_paths = ["/var/log", "/srv"]
allowed_local_paths = ["~/downloads"]
```

See the
[Configuration reference](https://github.com/overklassniy/ssh-mcp/blob/master/docs/Configuration.md)
for auth methods, command policy, path restrictions, keepalive,
transport modes, and validation rules.

## Security model

`ssh-mcp` does not verify remote host keys (`InsecureIgnoreHostKey`).
Security is enforced through two operator-configured mechanisms:

- **Command policy** - `whitelist`/`blacklist` regex patterns gate every
  command, including the individual probe commands used by
  `server-status`, so a restrictive whitelist cannot be bypassed.
- **Path restrictions** - `allowed_local_paths` and `allowed_remote_paths`
  confine SFTP operations; remote paths must be absolute POSIX paths.

Sensitive values are injected via environment variables:

| Env var | Used for |
| --- | --- |
| `SSH_MCP_PASSWORD` | Password auth |
| `SSH_MCP_PASSPHRASE` | Private key passphrase |
| `SSH_MCP_2FA_CODE` | 2FA code for keyboard-interactive auth |

## Compatibility

- **Clients** - Claude Desktop, Claude Code, Cursor, Devin, VS Code,
  Cline.
- **Platforms** - Windows, macOS, Linux (`amd64`, `arm64`).
- **Runtime** - Go 1.26+ to build, or a prebuilt binary from the
  Releases page.
- **Image** - distroless static, multi-arch (`linux/amd64`,
  `linux/arm64`), typically 15-18 MB.

## Documentation

- [Installation](https://github.com/overklassniy/ssh-mcp/blob/master/docs/Installation.md) - local binary, Docker, and MCPB one-click bundles.
- [Configuration](https://github.com/overklassniy/ssh-mcp/blob/master/docs/Configuration.md) - full TOML reference.
- [Tools](https://github.com/overklassniy/ssh-mcp/blob/master/docs/Tools.md) - the eight MCP tools, arguments, and examples.
- [Architecture](https://github.com/overklassniy/ssh-mcp/blob/master/docs/Architecture.md) - package layout and request data flow.
- [Troubleshooting](https://github.com/overklassniy/ssh-mcp/blob/master/docs/Troubleshooting.md) - common issues and fixes.

The same pages are mirrored to the repository's
[GitHub Wiki](https://github.com/overklassniy/ssh-mcp/wiki).

## Source and issues

- **Source**: [github.com/overklassniy/ssh-mcp](https://github.com/overklassniy/ssh-mcp)
- **Issues**: [github.com/overklassniy/ssh-mcp/issues](https://github.com/overklassniy/ssh-mcp/issues)
- **Security**: see the
  [Security policy](https://github.com/overklassniy/ssh-mcp/blob/master/SECURITY.md)

## License

MIT. See the
[LICENSE file](https://github.com/overklassniy/ssh-mcp/blob/master/LICENSE)
for the full text. The project also declares MIT in
`mcpb/manifest.json`.
