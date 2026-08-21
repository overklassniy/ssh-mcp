# Configuration

ssh-mcp is configured with a TOML file. The schema has a top-level
`[defaults]` section and one or more `[[server]]` entries. Each server
inherits from `[defaults]`, then from built-in defaults, and is
validated at startup.

## File location

Pass the config path with `--config`:

```sh
ssh-mcp --config ./ssh-mcp.toml
```

The `install` subcommand takes the same `--config` flag and embeds the
path in the client entry it writes.

## Full example

```toml
[defaults]
username = "deploy"
port = 22
private_key = "~/.ssh/id_ed25519"
timeout = "30s"
keepalive_interval = "30s"
keepalive_count_max = 3
max_output_bytes = 1048576
transport = "exec"

[[server]]
name = "web"
host = "web.example.com"
whitelist = ["^systemctl status .*", "^journalctl .*", "^ls .*"]
allowed_remote_paths = ["/var/log", "/srv"]
allowed_local_paths = ["~/downloads"]

[[server]]
name = "db"
host = "db.example.com"
username = "postgres"        # overrides defaults.username
port = 2222                  # overrides defaults.port
password = "..."             # or use SSH_MCP_PASSWORD
transport = "shell"
pty = true
pre_connect = true
```

## `[defaults]`

Optional. Values set here apply to every server that does not override
them.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `username` | string | – | SSH username |
| `port` | int | 22 | SSH port (1-65535) |
| `password` | string | – | Password auth (or `SSH_MCP_PASSWORD`) |
| `private_key` | string | – | Path to a private key file |
| `passphrase` | string | – | Private key passphrase (or `SSH_MCP_PASSPHRASE`) |
| `agent` | bool | false | Use the SSH agent (`SSH_AUTH_SOCK`) |
| `try_keyboard` | bool | false | Try keyboard-interactive auth (2FA) |
| `timeout` | duration | `30s` | Per-command timeout |
| `keepalive_interval` | duration | `30s` | Keepalive request interval |
| `keepalive_count_max` | int | 3 | Failures before closing the connection |
| `max_output_bytes` | int | 1048576 | Cap on captured stdout/stderr |
| `transport` | string | `exec` | `exec` or `shell` |
| `pty` | bool | false | Allocate a PTY for commands |
| `command_template` | string | – | Wrap commands (must contain `<command>` or `<quotedCommand>`) |

## `[[server]]`

One entry per remote server. Required fields: `name`, `host`,
`username`, `port` (or inherited from defaults), and at least one auth
method.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `name` | string | required | Unique server name used by `connectionName` |
| `host` | string | required | Hostname or IP address |
| `username` | string | required | SSH username (inheritable) |
| `port` | int | 22 | SSH port (inheritable) |
| `password` | string | – | Password auth (or `SSH_MCP_PASSWORD`) |
| `private_key` | string | – | Path to a private key file |
| `passphrase` | string | – | Private key passphrase (or `SSH_MCP_PASSPHRASE`) |
| `agent` | bool | false | Use the SSH agent |
| `try_keyboard` | bool | false | Try keyboard-interactive auth (2FA) |
| `timeout` | duration | `30s` | Per-command timeout |
| `keepalive_interval` | duration | `30s` | Keepalive interval |
| `keepalive_count_max` | int | 3 | Failures before reconnect |
| `max_output_bytes` | int | 1048576 | Output cap |
| `transport` | string | `exec` | `exec` or `shell` |
| `pty` | bool | false | Allocate a PTY |
| `command_template` | string | – | Wrap commands |
| `pre_connect` | bool | false | Connect at startup instead of lazily |
| `whitelist` | []string | – | Regex patterns a command must match |
| `blacklist` | []string | – | Regex patterns a command must not match |
| `allowed_local_paths` | []string | – | Local roots for SFTP uploads/downloads |
| `allowed_remote_paths` | []string | – | Remote roots for SFTP operations |
| `proxy_url` | string | – | SOCKS5 or HTTP CONNECT proxy URL |
| `ssh_config` | string | – | Path to an OpenSSH config for alias resolution |

## Authentication

Auth methods are tried in this order: private key, agent, password,
keyboard-interactive. Only configured methods are attempted. At least
one method is required.

Sensitive values should be provided via environment variables instead of
the config file:

| Env var | Used for |
| --- | --- |
| `SSH_MCP_PASSWORD` | Password auth (fallback when `password` is empty) |
| `SSH_MCP_PASSPHRASE` | Private key passphrase (fallback when `passphrase` is empty) |
| `SSH_MCP_2FA_CODE` | 2FA code for keyboard-interactive auth |

This is how MCPB and other launchers inject sensitive values without
exposing them on the command line.

## Command policy

`whitelist` and `blacklist` are lists of regular expressions. Before a
command runs, it is checked against the policy:

1. If a whitelist is configured, the command must match at least one
   whitelist pattern.
2. If a blacklist is configured, the command must not match any
   blacklist pattern.

When both are set, the whitelist is checked first. When no whitelist is
configured, all commands are allowed and a warning is logged at startup.
Invalid regex patterns produce an error at startup naming the offending
pattern and kind, so misconfigurations surface immediately.

The same policy also gates the individual probe commands used by the
`server-status` tool, so a restrictive whitelist cannot be bypassed via
status collection.

## Path restrictions

`allowed_local_paths` and `allowed_remote_paths` confine SFTP transfers
(uploads, downloads, directory sync, remote listings) to specific
directories.

- Local paths are resolved to absolute form and checked against the
  allowed roots. The current working directory is always included as the
  first local root. Symlinks are not expanded (to avoid Windows 8.3
  short-name issues); `filepath.Clean` is used instead. Null bytes in a
  path are rejected outright.
- Remote paths must be absolute POSIX paths. When `allowed_remote_paths`
  is empty, any absolute POSIX path is accepted and a warning is logged.

## Transport modes

- `exec` (default): each command runs in its own SSH session. Stateless.
- `shell`: a persistent shell session per server. Commands are framed by
  random marker lines so output and exit code can be extracted. Preserves
  shell state across commands.

## Duration format

Duration fields accept Go duration strings: `30s`, `5m`, `1h30m`, etc.

## Validation rules

At startup, each server is validated:

- `name` must be unique across all servers.
- `host` is required.
- `username` is required (directly or via defaults).
- `port` must be in 1-65535.
- At least one auth method must be configured.
- `transport` must be `exec` or `shell`.
- `command_template`, when set, must contain a `<command>` or
  `<quotedCommand>` placeholder.
- `pty` is a tri-state field (`*bool`) so the distinction between "unset"
  and "explicitly false" is preserved during default inheritance.

## SSH config alias resolution

In single-server mode, pass `--ssh-config` to resolve a host alias from
your OpenSSH config:

```sh
ssh-mcp --host myalias --ssh-config ~/.ssh/config
```

Resolution fills in missing fields (`HostName`, `User`, `Port`,
`IdentityFile`) from the matching `Host` block. It only runs when the
host is not an IP address and does not contain a dot. First-match-wins
semantics match OpenSSH. `Include` directives are followed recursively
with cycle detection.

## Hot-reload

When ssh-mcp is started with `--config`, the config file is watched for
changes and reloaded automatically without restarting the server. This
applies only to file-based config (the `--config` flag), not to
single-server CLI mode.

### How it works

- The config file's modification time is polled every 2 seconds.
- Polling is used instead of filesystem notifications because config
  files often live on network or cloud-synced filesystems where
  kernel-level notifications are unreliable.
- When a change is detected, the file is reloaded and validated. If
  parsing or validation fails, the error is logged and the previous
  configuration is kept.
- The reloaded configuration is applied atomically to the connection
  manager:
  - Servers that were removed have their connections closed.
  - Servers whose config changed in any way have their connections
    closed so the next operation reconnects with the new settings.
  - Servers whose config is unchanged keep their existing connections.
  - Command policies (whitelist/blacklist) are always rebuilt from the
    new config, so policy changes take effect immediately without
    reconnection.

### What triggers a reload

Any change to the config file's modification time, including:

- Adding or removing a `[[server]]` entry.
- Changing any field on an existing server (host, port, auth method,
  whitelist, timeouts, etc.).
- Editing the `[defaults]` section.

### Limitations

- The reload only affects file-based config. Single-server mode
  (`--host` and related CLI flags) does not support hot-reload.
- If the config file is deleted or becomes unreadable, the watcher logs
  a warning and retries on the next poll cycle. The last successfully
  loaded configuration remains active.
- The polling interval is 2 seconds, so there is a brief delay between
  saving the file and the change taking effect.
