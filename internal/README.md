# internal

Root of the internal packages for ssh-mcp.

## Purpose

This folder holds all reusable, non-entry-point logic for the project. Under
Go's `internal` mechanism, nothing under this tree can be imported by modules
outside `github.com/overklassniy/ssh-mcp`, which keeps the implementation
details private to the binary.

## Contents

- `config/` - TOML configuration schema, loading, defaults, validation, and
  CLI-flag merging.
- `install/` - registers ssh-mcp server entries into the MCP config files of
  supported AI clients.
- `mcp/` - wires the SSH connection manager to the MCP protocol server using
  `mark3labs/mcp-go` over stdio.
- `paths/` - validates local and remote file paths against configured
  allowed-path lists to prevent path traversal.
- `policy/` - command whitelist/blacklist validation using regular
  expressions.
- `ssh/` - SSH connection management, command execution (exec and shell
  modes), SFTP file transfers, and local/remote port forwarding.
- `sshconfig/` - parses OpenSSH-style `~/.ssh/config` and resolves host
  aliases to connection parameters.
- `status/` - collects remote system status via a single batched SSH
  command.

## Integration with the project

- The `cmd/ssh-mcp` entry point imports `config`, `mcp`, `sshconfig`, and
  `install`.
- `mcp` depends on `config`, `ssh`, and `mcp/tools`.
- `ssh` depends on `config`, `policy`, and `paths`.
- `status` depends on `policy`.
- `config` depends on `sshconfig`.

## Notes

- Each subpackage has its own `README.md` describing its purpose, contents,
  integration points, and constraints in more detail.
- Do not export types or functions from these packages that are not needed
  by other packages in the module; the `internal` boundary enforces
  privacy, but minimal APIs keep the surface area small.
