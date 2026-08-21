<p align="center">
  <img src="./assets/readme/hero.gif" width="100%" alt="ssh-mcp: an MCP server that gives AI assistants secure, scoped SSH access to remote servers. Light technical hero with a terminal prompt and an agent-to-remote connection line drawn through a policy gate.">
</p>

<p align="center">
  <a href="https://github.com/overklassniy/ssh-mcp/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/overklassniy/ssh-mcp/ci.yml?style=flat-square&label=CI" alt="CI status"></a>
  <a href="https://hub.docker.com/r/overklassniy/ssh-mcp"><img src="https://img.shields.io/docker/v/overklassniy/ssh-mcp?style=flat-square&logo=docker&logoColor=white&label=Docker%20Hub" alt="Docker Hub"></a>
  <a href="https://go.dev/doc/go1.26"><img src="https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.26+"></a>
  <a href="https://modelcontextprotocol.io/"><img src="https://img.shields.io/badge/MCP-server-6E4B9E?style=flat-square" alt="MCP server"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/overklassniy/ssh-mcp?style=flat-square" alt="License: MIT"></a>
</p>

# ssh-mcp

An MCP (Model Context Protocol) server that gives AI assistants secure,
scoped SSH access to one or more remote servers.

`ssh-mcp` runs over stdio and works with any MCP-compatible client:
Claude Desktop, Claude Code, Cursor, Devin, VS Code, and Cline. It
exposes eight tools for command execution, SFTP file transfer, directory
sync, port forwarding, and remote system status collection.

## Why it is different

AI assistants that need to operate on remote servers typically either
get a raw shell (unsafe, unscoped) or have no SSH access at all.
`ssh-mcp` sits in between: it gives the agent a small, auditable set of
tools, each gated by an optional command whitelist/blacklist and path
restrictions, so the operator controls exactly what the agent can do.

- **Command policy** – `whitelist` and `blacklist` regex patterns gate
  every command before execution. Without a whitelist, all commands are
  allowed and a warning is logged at startup.
- **Path restrictions** – `allowed_local_paths` and `allowed_remote_paths`
  confine SFTP transfers to specific directories and reject path
  traversal.
- **Secrets via env vars** – password, passphrase, and 2FA code are
  passed via environment variables, never on the command line.
- **Hot-reload** – when started with `--config`, the config file is
  watched for changes and reloaded without restarting the server. Add,
  remove, or modify servers and the change takes effect within seconds.

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

See [Tools](docs/Tools.md) for arguments and examples.

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

See [Architecture](docs/Architecture.md) for the package layout,
dependency graph, and full request data flow.

## Quickstart

```sh
git clone https://github.com/overklassniy/ssh-mcp.git
cd ssh-mcp
make build
```

Create a minimal `ssh-mcp.toml`:

```toml
[[server]]
name = "web"
host = "example.com"
username = "deploy"
port = 22
private_key = "~/.ssh/id_ed25519"
```

Register it in your client:

```sh
ssh-mcp install --client claude-code --config ./ssh-mcp.toml
```

Restart your AI client, then ask the agent:

> Use the list-servers tool.

You should see your server with a `connected` or `disconnected` status.

## Installation

Three install paths produce the same result – an MCP server entry in
your client's config file. See [Installation](docs/Installation.md) for
the full guide.

- **Local binary** – `make build`, then `ssh-mcp install --client <client>
  --config ./ssh-mcp.toml`. Supported clients: `claude-desktop`,
  `claude-code`, `cursor`, `Devin`, `vscode`, `cline`.
- **Docker** – `ssh-mcp install --client <client> --docker --config
  ./ssh-mcp.toml`. Defaults to `ghcr.io/overklassniy/ssh-mcp:latest`.
- **MCPB one-click bundle** – download the `.mcpb` bundle from the
  Releases page and open it in a client that supports MCPB (Claude
  Desktop, Claude Code, MCP for Windows).
- **Paste-in config** – skip the installer and paste a ready-made
  server entry directly into your client's config. The `snippet`
  subcommand generates it for your exact setup:

  ```sh
  ssh-mcp snippet --docker --host 192.168.1.1 --user root
  ssh-mcp snippet --gorun  --host 192.168.1.1 --user root
  ```

  Docker uses the published multi-arch image; `--gorun` uses
  `go run github.com/overklassniy/ssh-mcp/cmd/ssh-mcp@latest` (the
  Go-native equivalent of `npx -y <package>`, requires the Go
  toolchain). Secrets stay in the `env` block, never in `args`. See
  [Installation](docs/Installation.md#option-d-paste-in-config) for
  ready-to-paste JSON blocks.

Single-server mode skips the TOML file and takes connection details as
CLI flags:

```sh
ssh-mcp --host example.com --username deploy --port 22 \
  --private-key ~/.ssh/id_ed25519
```

### Paste-in config (go run)

If you have the Go toolchain (Go 1.26+) installed, you can skip the
binary build entirely. Add this entry to your client's `mcpServers`
config (e.g. `~/.config/devin/mcp_config.json`):

```json
{
  "mcpServers": {
    "ssh-mcp": {
      "command": "go",
      "args": [
        "run",
        "github.com/overklassniy/ssh-mcp/cmd/ssh-mcp@latest",
        "--host", "192.168.1.1",
        "--port", "22",
        "--user", "root",
        "--transport", "exec"
      ],
      "env": {
        "SSH_MCP_PASSWORD": "your-password"
      }
    }
  }
}
```

On first run, `go run` fetches and compiles the latest tagged release,
then starts the stdio MCP server. Subsequent runs use the build cache.
For private-key auth, replace `SSH_MCP_PASSWORD` with
`SSH_MCP_PASSPHRASE` and add `--private-key` to `args`. Pin a specific
version by replacing `@latest` with `@v1.0.0`.

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

See [Configuration](docs/Configuration.md) for the full reference:
auth methods, command policy, path restrictions, keepalive, transport
modes, and validation rules.

## Security model

`ssh-mcp` does not verify remote host keys (`InsecureIgnoreHostKey`).
Security is enforced through two operator-configured mechanisms:

- **Command policy** – `whitelist`/`blacklist` regex patterns gate every
  command, including the individual probe commands used by
  `server-status`, so a restrictive whitelist cannot be bypassed.
- **Path restrictions** – `allowed_local_paths` and `allowed_remote_paths`
  confine SFTP operations; remote paths must be absolute POSIX paths.

Sensitive values are injected via environment variables:

| Env var | Used for |
| --- | --- |
| `SSH_MCP_PASSWORD` | Password auth |
| `SSH_MCP_PASSPHRASE` | Private key passphrase |
| `SSH_MCP_2FA_CODE` | 2FA code for keyboard-interactive auth |

## Compatibility

- **Clients** – Claude Desktop, Claude Code, Cursor, Devin, VS Code,
  Cline.
- **Platforms** – Windows, macOS, Linux (`amd64`, `arm64`).
- **Runtime** – Go 1.26+ to build, or a prebuilt binary from the
  Releases page.

## Documentation

- [Installation](docs/Installation.md) – local binary, Docker, and MCPB
  one-click bundles.
- [Configuration](docs/Configuration.md) – full TOML reference.
- [Tools](docs/Tools.md) – the eight MCP tools, arguments, and examples.
- [Architecture](docs/Architecture.md) – package layout and request
  data flow.
- [Troubleshooting](docs/Troubleshooting.md) – common issues and fixes.

The same pages are mirrored to the repository's GitHub Wiki by the
`wiki-sync` workflow; edit them in `docs/`, not in the wiki UI.

## License

MIT. See [LICENSE](LICENSE) for the full text. The project also declares
MIT in `mcpb/manifest.json`.
