# Installation

ssh-mcp can be installed three ways: as a local binary registered into
an AI client, as a Docker container, or as a one-click MCPB bundle. All
three produce the same result: an MCP server entry in your client's
config file.

## Prerequisites

- One or more SSH-reachable remote servers.
- An MCP-compatible AI client: Claude Desktop, Claude Code, Cursor,
  Devin, VS Code, or Cline.
- For local binary install: Go 1.26+ to build, or a prebuilt binary from
  the Releases page.
- For Docker install: Docker or Docker Desktop.

## Option A: local binary

### 1. Build the binary

```sh
git clone https://github.com/overklassniy/ssh-mcp.git
cd ssh-mcp
make build
```

This produces an `ssh-mcp` executable in the repo root. Alternatively,
download a prebuilt binary for your platform from the Releases page and
put it on your `PATH`.

### 2. Create a config file

See [Configuration](Configuration) for the full reference. A minimal
config:

```toml
[[server]]
name = "web"
host = "example.com"
username = "deploy"
port = 22
private_key = "~/.ssh/id_ed25519"
```

Save it as `ssh-mcp.toml`.

### 3. Register in your client

```sh
ssh-mcp install --client claude-code --config ./ssh-mcp.toml
```

Replace `claude-code` with one of:

| Client | Value |
| --- | --- |
| Claude Desktop | `claude-desktop` |
| Claude Code | `claude-code` |
| Cursor | `cursor` |
| Devin | `Devin` |
| VS Code | `vscode` |
| Cline | `cline` |

Use `--dry-run` first to preview the entry without writing it:

```sh
ssh-mcp install --client claude-code --config ./ssh-mcp.toml --dry-run
```

The install command merges non-destructively into your client's existing
config file, creates a `.bak` backup before overwriting, and creates
parent directories as needed. It never removes other server entries.

### 4. Restart your client

MCP clients read their config at startup. Restart Claude Desktop (or
your chosen client) so it picks up the new entry.

## Option B: Docker

Docker mode registers a `docker run -i --rm` entry instead of a direct
executable. The host home directory is bind-mounted at the same path so
`~/` references in the TOML resolve identically inside the container,
the SSH agent socket is forwarded when `SSH_AUTH_SOCK` is set, and
sensitive values are passed via `-e` from the host environment.

```sh
ssh-mcp install --client claude-code --docker --config ./ssh-mcp.toml
```

The image defaults to `ghcr.io/overklassniy/ssh-mcp:latest`. Override it
with `--docker-image`:

```sh
ssh-mcp install --client claude-code --docker \
  --docker-image ghcr.io/overklassniy/ssh-mcp:dev \
  --config ./ssh-mcp.toml
```

### Platform notes for Docker mode

Docker install mode is best supported on Linux, where bind mounts and
unix sockets map directly. On macOS and Windows (Docker Desktop), host
paths are translated through a VM layer and absolute paths differ; some
manual `-v` adjustment may be needed. SSH agent forwarding via a unix
socket works on Linux and macOS but not on Windows (named pipes are not
forwarded by Docker Desktop).

Local file upload/download paths outside the home directory require
extra `-v` mounts in the generated entry, because the container
filesystem is otherwise isolated.

## Option C: MCPB one-click bundle

MCPB (MCP Bundle) is the cross-client standard for one-click
installation of local MCP servers. It is supported by Claude Desktop,
Claude Code, and MCP for Windows.

1. Download the `.mcpb` bundle for your platform from the Releases page.
2. Open the bundle in your client. The client prompts for SSH connection
   details (host, port, username, password or private key, passphrase,
   2FA toggle) on first install.
3. Sensitive values (password, passphrase) are passed via environment
   variables, not CLI args.

Cursor, Devin, and VS Code users should use Option A or B instead,
as those clients do not currently support MCPB one-click install.

### Building bundles from source

```sh
make mcpb
```

This builds per-platform bundles into `dist/`, each containing
`manifest.json` and the binary under `server/`. Requires `zip` on
`PATH`.

## Option D: paste-in config

If you do not want to run `ssh-mcp install`, you can paste a
ready-made server entry directly into your client's config file
under its `mcpServers` key (or `servers` for VS Code). Two runtime
modes are supported: Docker (uses the published multi-arch image)
and `go run` (fetches and compiles the latest tagged release on
first run, the Go-native equivalent of `npx -y <package>`).

The `snippet` subcommand generates these blocks for your exact
setup so you do not have to hand-write them:

```sh
ssh-mcp snippet --docker --host 192.168.1.1 --user root
ssh-mcp snippet --gorun  --host 192.168.1.1 --user root
```

Both print a `"ssh-mcp": { ... }` entry wrapped in a minimal
`mcpServers` object. Copy the inner entry into your client's config.

Sensitive values (password, passphrase) are always placed in the
`env` block, never in `args`, so they do not leak into process
listings.

### Docker, single-server, password auth

```json
{
  "mcpServers": {
    "ssh-mcp": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "SSH_MCP_PASSWORD",
        "-e", "SSH_MCP_PASSPHRASE",
        "-e", "SSH_MCP_2FA_CODE",
        "ghcr.io/overklassniy/ssh-mcp:latest",
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

The `-e` flags forward the env vars from the docker process into
the container. Setting `SSH_MCP_PASSWORD` in the `env` block makes
it available to docker, which passes it through to ssh-mcp inside
the container.

### Docker, single-server, private-key auth

```json
{
  "mcpServers": {
    "ssh-mcp": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "SSH_MCP_PASSWORD",
        "-e", "SSH_MCP_PASSPHRASE",
        "-e", "SSH_MCP_2FA_CODE",
        "-e", "HOME=/home/you",
        "-v", "/home/you:/home/you",
        "ghcr.io/overklassniy/ssh-mcp:latest",
        "--host", "192.168.1.1",
        "--port", "22",
        "--user", "root",
        "--transport", "exec",
        "--private-key", "/home/you/.ssh/id_ed25519"
      ],
      "env": {
        "SSH_MCP_PASSPHRASE": "your-passphrase"
      }
    }
  }
}
```

The home directory is bind-mounted at the same path so that `~/`
and absolute key paths resolve identically inside the container.
Omit the `SSH_MCP_PASSPHRASE` env var if your key has no passphrase.

### Docker, multi-server via config file

```json
{
  "mcpServers": {
    "ssh-mcp": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "-e", "SSH_MCP_PASSWORD",
        "-e", "SSH_MCP_PASSPHRASE",
        "-e", "SSH_MCP_2FA_CODE",
        "-e", "HOME=/home/you",
        "-v", "/home/you:/home/you",
        "-v", "/home/you/ssh-mcp.toml:/config.toml:ro",
        "ghcr.io/overklassniy/ssh-mcp:latest",
        "--config", "/config.toml"
      ]
    }
  }
}
```

The TOML config is mounted read-only at `/config.toml`. See
[Configuration](Configuration) for the full reference.

### Docker on Windows (GUI clients)

Windows GUI clients (Claude Desktop, Cursor) often do not inherit
the shell `PATH`, so `docker` may not be found. Wrap the command
with `cmd /c`:

```json
{
  "mcpServers": {
    "ssh-mcp": {
      "command": "cmd",
      "args": [
        "/c", "docker",
        "run", "-i", "--rm",
        "-e", "SSH_MCP_PASSWORD",
        "-e", "SSH_MCP_PASSPHRASE",
        "-e", "SSH_MCP_2FA_CODE",
        "ghcr.io/overklassniy/ssh-mcp:latest",
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

On macOS with nvm or Homebrew, the same issue can affect `docker`;
use the absolute path from `which docker` as the `command` value.

### go run, single-server

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

This requires the Go toolchain (Go 1.26+) on the host. On first
run, `go run` fetches and compiles the latest tagged release, then
starts the stdio MCP server. Subsequent runs use the build cache.
Pin a specific version by replacing `@latest` with `@v1.0.0`.

### Platform notes for paste-in Docker config

The same caveats from [Option B](#option-b-docker) apply: Docker
Desktop on macOS and Windows translates host paths through a VM
layer, so bind mounts may need manual adjustment. SSH agent
forwarding via a unix socket works on Linux and macOS but not on
Windows. Local file upload/download paths outside the home
directory require extra `-v` mounts.

## Single-server mode (no config file)

For quick one-off use, you can skip the TOML file and pass connection
details as CLI flags. The binary builds a single-server config from the
flags:

```sh
ssh-mcp --host example.com --username deploy --port 22 \
  --private-key ~/.ssh/id_ed25519
```

In this mode, `--ssh-config` can resolve a host alias from
`~/.ssh/config`:

```sh
ssh-mcp --host myalias --ssh-config ~/.ssh/config
```

Alias resolution only runs when the host is not an IP address and does
not contain a dot, and only when `--ssh-config` is provided.

## Verifying the install

After installing and restarting your client, ask the agent to list the
configured servers:

> Use the list-servers tool.

You should see your server with a `connected` or `disconnected` status.
Then try a simple command:

> Run `uname -a` on the web server using execute-command.

If something goes wrong, see [Troubleshooting](Troubleshooting).
