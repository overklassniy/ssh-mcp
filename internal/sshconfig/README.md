# internal/sshconfig

OpenSSH-style `~/.ssh/config` parser and host alias resolver.

## Purpose

This package parses OpenSSH config files and resolves a host alias to its
connection parameters (`HostName`, `User`, `Port`, `IdentityFile`). It allows
ssh-mcp to accept the same host aliases a user types at the `ssh` command
line, instead of requiring fully-qualified connection details in the TOML
config or CLI flags.

## Contents

- `parser.go` - the `Entry` struct (resolved parameters for one alias);
  `Lookup` (finds the config for an alias, defaulting to `~/.ssh/config`);
  `parseFile` (tokenizes a config file, following `Include` directives with
  cycle detection); `matchHost` (first-match-wins resolution across all
  matching blocks); `blockMatches` and `patternMatches` (wildcard and
  negated-pattern matching); `expandInclude` (resolves `Include` patterns
  relative to the including file, with tilde and glob expansion);
  `expandTilde`; `stripComment`; and `homeDir`.
- `parser_test.go` - unit tests for parsing, includes, wildcards, negation,
  and first-match-wins semantics.

## Integration with the project

- Used by `internal/config.ResolveSSHConfig` to fill in missing server
  fields from the user's SSH config.
- Used by `cmd/ssh-mcp` in single-server mode to resolve a host alias before
  building the `ServerConfig`.

## Notes

- `Include` directives are followed recursively, with a `visited` set keyed
  on the real (symlink-resolved) path to prevent infinite cycles.
- First-match-wins semantics match OpenSSH: for each field, the first value
  seen across all matching blocks is kept.
- Host patterns support `*` and `?` wildcards and `!` negation; a negated
  match causes the block to be rejected immediately.
- A missing default config file (`~/.ssh/config`) returns `nil, nil` (no
  error), so users without an SSH config are not penalized. An explicit
  `configPath` that does not exist returns an error.
- Comments starting with `#` (at the start of a line or preceded by
  whitespace) are stripped before parsing.
