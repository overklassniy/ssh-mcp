# internal/policy

Command whitelist and blacklist validation using regular expressions.

## Purpose

This package implements the command security model for ssh-mcp, mirroring the
behavior of the original TypeScript ssh-mcp-server. Before a command is
executed on a remote server, it is checked against an optional whitelist and
an optional blacklist of regular-expression patterns.

## Contents

- `policy.go` - the `Policy` type holding compiled whitelist and blacklist
  regexps; `New` (compiles pattern strings, returning an error that names the
  offending pattern and kind on invalid regex); `HasWhitelist` and
  `HasBlacklist` reporters; and `Validate`, which returns
  `(allowed bool, reason string)`.
- `policy_test.go` - unit tests for whitelist-only, blacklist-only, combined,
  and invalid-pattern cases.

## Integration with the project

- Constructed per server by `internal/ssh.ConnectionManager` from
  `ServerConfig.Whitelist` and `ServerConfig.Blacklist`.
- Used by `internal/ssh` (`exec.go` and `shell.go`) to validate commands
  before execution.
- Used by `internal/status` (`collector.go`) to filter individual probe
  commands before they are batched into the status script.

## Notes

- When both lists are configured, the whitelist is checked first: a command
  must match at least one whitelist pattern, then must not match any
  blacklist pattern.
- An empty pattern string is skipped (not compiled).
- Invalid regex patterns produce an error naming the pattern and the kind
  (`whitelist` or `blacklist`), so misconfigurations surface at startup
  rather than at first use.
- When no whitelist is configured, the connection manager logs a warning
  that all commands will be allowed.
