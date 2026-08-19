# internal/config

TOML configuration schema, loading, defaults, and validation for ssh-mcp.

## Purpose

This package defines the configuration model used by ssh-mcp: a top-level
`Config` with a `[defaults]` section and one or more `[[server]]` entries. It
loads TOML files, applies defaults, validates required fields, and provides a
single-server constructor used when the binary is driven by CLI flags instead
of a config file.

## Contents

- `config.go` - the `Config`, `Defaults`, and `ServerConfig` structs with
  TOML tags; the `Duration` wrapper type (with `UnmarshalText` /
  `MarshalText` for `time.ParseDuration` strings); and the built-in default
  constants (timeouts, max output bytes, keepalive, transport).
- `loader.go` - `Load` (read and parse a TOML file), `FromServer` (build a
  `Config` from a single `ServerConfig` for CLI single-server mode),
  `applyDefaults` (per-server inheritance from `[defaults]` then built-in
  defaults), `validate` (required fields, port range, auth method, transport
  value, command template placeholder), `ResolveSSHConfig` (fill missing
  fields from `~/.ssh/config`), `ExpandHome`, and accessor methods on
  `Config` and `ServerConfig`.
- `loader_test.go` - unit tests for loading, default application, and
  validation.

## Integration with the project

- Consumed by `cmd/ssh-mcp` (`loadConfig` calls `Load` or `FromServer`).
- Consumed by `internal/mcp` (`mcp.New` takes a `*config.Config`) and by
  `internal/ssh` (`ConnectionManager` reads server configs and policies).
- Depends on `internal/sshconfig` (for `ResolveSSHConfig`) and on
  `github.com/pelletier/go-toml/v2` for TOML parsing.

## Notes

- `Duration` fields accept Go duration strings such as `30s`, `5m`, `1h30m`.
- Validation requires each server to have a unique `name`, a `host`, a
  `username`, a `port` in 1-65535, and at least one authentication method
  (`password`, `private_key`, `agent`, or `try_keyboard`).
- `transport` must be `exec` or `shell`; anything else is rejected.
- `command_template`, when set, must contain a `<command>` or
  `<quotedCommand>` placeholder.
- `PTY` is a `*bool` so the distinction between "unset" and "explicitly
  false" is preserved during default application.
