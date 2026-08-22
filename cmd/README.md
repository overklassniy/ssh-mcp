# cmd

Parent folder for ssh-mcp command binaries.

## Purpose

This folder holds the entry-point packages that compile into executables.
The Go convention is to place each binary's `main` package in its own
subdirectory under `cmd/`, so the binary name matches the subdirectory name.

## Contents

- `ssh-mcp/` – the `ssh-mcp` binary entry point. Parses CLI flags (or a TOML
  config file), loads configuration, and starts the MCP stdio server; also
  provides the `install` and `snippet` subcommands. `install` registers
  ssh-mcp in a supported AI client's MCP config; `snippet` prints a
  ready-to-paste config entry without modifying files.

## Integration with the project

- The `ssh-mcp` binary is built from `cmd/ssh-mcp` by the root `Makefile`
  and `goreleaser.yml`.
- The same binary is packaged into `.mcpb` bundles by the `make mcpb` target
  (see `mcpb/`) and registered into AI clients by the `install` subcommand
  (see `internal/install/`).
- The entry point imports internal packages under `internal/` for
  configuration, MCP server wiring, SSH config parsing, and install logic.

## Notes

- Only `main` packages belong here. All reusable logic lives under
  `internal/`.
- Adding a new binary means adding a new subdirectory under `cmd/` with its
  own `main` package.
