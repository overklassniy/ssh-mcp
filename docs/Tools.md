# Tools

ssh-mcp exposes eight MCP tools. All tools accept an optional
`connectionName` argument that selects the configured server; when
omitted, the first configured server is used.

Errors are returned as MCP error results that preserve the stable
`ssh.ToolError` code and a `Retriable` flag, so agent-side retry logic
can decide whether to retry.

## execute-command

Runs a command on a remote server and returns stdout, stderr, exit
code, and duration.

| Argument | Type | Required | Description |
| --- | --- | --- | --- |
| `command` | string | yes | The command to run |
| `connectionName` | string | no | Target server (defaults to first) |

Behavior:

- The command is validated against the server's whitelist/blacklist
  before execution.
- If `command_template` is set, the command is wrapped in the template
  (which must contain a `<command>` or `<quotedCommand>` placeholder).
- A PTY is allocated when `pty` is enabled for the server.
- Output is capped at `max_output_bytes`.
- The configured `timeout` is enforced.
- In `shell` transport mode, the command runs in the persistent shell
  session and is framed by random marker lines.

Example prompt:

> Run `df -h` on the web server.

## upload

Uploads a local file to a remote server via SFTP.

| Argument | Type | Required | Description |
| --- | --- | --- | --- |
| `localPath` | string | yes | Local file to upload |
| `remotePath` | string | yes | Remote destination path |
| `connectionName` | string | no | Target server |

Both paths are validated: the local path must be within an allowed local
root, and the remote path must be within an allowed remote root (if
configured).

## download

Downloads a remote file to the local machine via SFTP.

| Argument | Type | Required | Description |
| --- | --- | --- | --- |
| `remotePath` | string | yes | Remote file to download |
| `localPath` | string | yes | Local destination path |
| `connectionName` | string | no | Target server |

The download uses a temp file plus an atomic rename, so a failed
transfer does not leave a partial file at the destination. Paths are
validated as with `upload`.

## list-remote

Lists the contents of a remote directory via SFTP.

| Argument | Type | Required | Description |
| --- | --- | --- | --- |
| `remotePath` | string | yes | Remote directory to list |
| `connectionName` | string | no | Target server |

Returns structured file entries: name, size, mode, modTime, and isDir
for each entry.

## dir-sync

Recursively syncs a directory between local and remote via SFTP, with
concurrent file transfers.

| Argument | Type | Required | Description |
| --- | --- | --- | --- |
| `localPath` | string | yes | Local directory |
| `remotePath` | string | yes | Remote directory |
| `direction` | string | yes | `upload` or `download` |
| `connectionName` | string | no | Target server |

In `upload` direction, local files are copied to the remote directory.
In `download` direction, remote files are copied to the local directory.
Transfers run concurrently for speed. Paths are validated as with
`upload`/`download`.

## port-forward

Opens, closes, and lists local and remote port forwards.

| Argument | Type | Required | Description |
| --- | --- | --- | --- |
| `action` | string | yes | `open`, `close`, or `list` |
| `type` | string | for `open`/`close` | `local` or `remote` |
| `localAddress` | string | for `open`/`close` | Local bind address |
| `localPort` | int | for `open`/`close` | Local port |
| `remoteAddress` | string | for `open`/`close` | Remote target address |
| `remotePort` | int | for `open`/`close` | Remote target port |
| `connectionName` | string | no | Target server |

Forwards persist across tool calls within a single ssh-mcp process. The
forward state is held in a per-connection map guarded by a mutex.

- **Local forward**: `ssh -L localAddress:localPort -> remoteAddress:remotePort`
- **Remote forward**: `ssh -R remoteAddress:remotePort -> localAddress:localPort`

`list` returns all active forwards for the connection.

## server-status

Collects system status from a remote server via a single batched SSH
command.

| Argument | Type | Required | Description |
| --- | --- | --- | --- |
| `connectionName` | string | no | Target server |

Rather than issuing many separate SSH commands, this tool joins all
probes into one remote script delimited by random marker lines, runs it
once, and parses the output. Collected fields include hostname, OS, CPU,
memory, disk, GPUs, processes, and services.

Each probe command is validated against the server's command policy
before being included in the batched script, so a restrictive whitelist
cannot be bypassed via status collection.

The probe commands assume a Linux remote with common utilities (`ip`,
`lscpu` or `/proc/cpuinfo`, `nvidia-smi` for GPUs, `systemctl` or
`/etc/init.d` for services, `free`, `df`, `ps`, `top`, `uname`). Missing
utilities produce empty values rather than errors. Loopback addresses
(`127.x`) are filtered out of the collected IP addresses.

If the batched command fails, every field is left unset and `reachable`
is still reported as `true`; interpret a mostly-empty status as a probe
failure.

## list-servers

Lists all configured SSH servers with their connection status.

No arguments.

Returns the server name and whether each is currently connected. Useful
for the agent to discover which servers are available before running a
command.
