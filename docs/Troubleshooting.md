# Troubleshooting

Common issues and their fixes.

## The agent cannot see ssh-mcp tools

**Cause**: the MCP client has not loaded the server entry.

**Fix**:

1. Confirm the entry was written: run
   `ssh-mcp install --client <client> --config ./ssh-mcp.toml --dry-run`
   and check the output.
2. Restart the client. MCP clients read their config at startup.
3. Check the client's config file for the `ssh-mcp` entry under
   `mcpServers` (or `servers` for VS Code).

## Connection failures

### `SSH_CONNECTION_FAILED`

The SSH client could not connect to the remote server.

**Checks**:

- Verify `host`, `port`, and `username` in the config.
- Verify the auth method. If using a private key, confirm the path is
  correct and the key is readable. If using an agent, confirm
  `SSH_AUTH_SOCK` is set and the key is loaded (`ssh-add -l`).
- If connecting through a proxy, verify `proxy_url` is reachable and the
  proxy type (SOCKS5 or HTTP CONNECT) is correct.
- Test the same connection with the `ssh` CLI to isolate whether the
  issue is ssh-mcp or the server itself.

### Authentication fails with a private key

- If the key has a passphrase, set it via `passphrase` in the config or
  the `SSH_MCP_PASSPHRASE` environment variable.
- For 2FA, enable `try_keyboard` and provide the code via
  `SSH_MCP_2FA_CODE`.

## Commands are rejected

### Command blocked by policy

If a whitelist is configured, the command must match at least one
whitelist pattern. If a blacklist is configured, the command must not
match any blacklist pattern.

**Fix**: adjust the `whitelist`/`blacklist` regex patterns in the server
config. Remember the whitelist is checked first when both are set.

### `COMMAND_TIMEOUT`

The command exceeded the configured `timeout`.

**Fix**: increase `timeout` for the server, or run a faster variant of
the command. Note that `max_output_bytes` caps captured output; a
command that produces huge output may appear to hang until the cap is
reached.

## SFTP transfers fail

### Path outside allowed roots

If `allowed_local_paths` or `allowed_remote_paths` is configured, the
path must fall within one of the allowed roots.

**Fix**: add the directory to the allowed paths, or move the file inside
an already-allowed root.

### Remote path must be absolute

Remote paths must be absolute POSIX paths (for example `/srv/file.txt`,
not `file.txt` or `~/file.txt`). Tilde is not expanded on the remote
side.

### Docker mode: path not mounted

In Docker install mode, the host home directory is bind-mounted at the
same path, so `~/` paths work. Paths outside the home directory require
extra `-v` mounts in the generated entry. Edit the client's config file
to add the needed `-v` mounts.

## Docker mode issues

### SSH agent forwarding does not work on Windows

Docker Desktop on Windows does not forward named pipes, so SSH agent
forwarding via a unix socket does not work. Use a private key file
instead, or run ssh-mcp as a local binary on Windows.

### Host paths differ on macOS/Windows

On macOS and Windows (Docker Desktop), host paths are translated through
a VM layer and absolute paths differ from what the container sees. Some
manual `-v` adjustment may be needed in the generated entry.

## server-status returns mostly empty fields

If `server-status` returns `reachable: true` but most fields are empty,
the batched probe script failed. This typically means the remote is not
a Linux host or is missing the expected utilities (`ip`, `lscpu`,
`free`, `df`, `ps`, `top`, `uname`, optionally `nvidia-smi` and
`systemctl`). Missing utilities produce empty values, not errors.

**Fix**: install the missing utilities on the remote, or restrict which
probes run by tightening the whitelist to only the commands the remote
supports.

## Wiki sync workflow fails

### Permission denied on push

The workflow uses `${{ github.token }}` with `permissions: contents:
write`. If the repository's default token permissions are set to
read-only (Settings > Actions > General > Workflow permissions), the
push fails. Either:

- Set the workflow's `permissions: contents: write` explicitly (already
  done in `wiki-sync.yml`), and ensure the repo allows workflows to
  override the default, or
- Change the repo default to "Read and write permissions".

## Logging

All logging goes to stderr (stdout is reserved for MCP stdio framing).
To see logs when running manually:

```sh
ssh-mcp --config ./ssh-mcp.toml 2>ssh-mcp.log
```

When running under an MCP client, the client captures stderr; check the
client's MCP server logs (for example, Claude Desktop's
`mcp.log`).
