# testdata

Test fixtures used by the package tests.

## Purpose

This folder holds static fixture files consumed by the unit and integration
tests under `internal/`. Keeping fixtures separate from the test source makes
them easy to reference via relative paths and keeps test inputs visible.

## Contents

- `sample.toml` - a sample ssh-mcp TOML configuration used by the config
  loader tests. It contains a `[defaults]` section (timeouts, max output
  bytes, keepalive, PTY, transport) and two `[[server]]` entries: a `dev`
  server in exec mode with a command whitelist and allowed remote paths, and
  a `bastion` server in shell mode with a custom shell-ready timeout.

## Integration with the project

- Read by `internal/config/loader_test.go` to exercise TOML loading, default
  application, and validation.
- The structure mirrors the schema defined in `internal/config/config.go`.

## Notes

- The `password` values in `sample.toml` (`secret`, `pwd123456`) are
  fixtures only and must never be used against real hosts.
- Do not add binary or large fixtures here without a clear test need; keep
  fixtures small and text-based so they remain reviewable.
