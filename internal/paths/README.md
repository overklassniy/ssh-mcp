# internal/paths

Local and remote path validation against configured allowed-path lists.

## Purpose

This package prevents path traversal attacks on both the local filesystem and
the remote SFTP server. It resolves and normalizes paths, then checks that
they fall within one of the configured allowed roots before any file
operation is performed.

## Contents

- `paths.go` – `ValidateLocal` (resolves a local path to an absolute form,
  requires the parent directory to exist for writes, and checks it is within
  an allowed root), `ValidateRemote` (requires an absolute POSIX path and
  checks it against allowed remote roots, accepting any absolute path when
  no roots are configured), `ResolveAllowedLocalRoots` (resolves and
  deduplicates allowed local roots, always including the current working
  directory as the first root), `Cwd` (returns the current working
  directory), and the internal `isWithinRoot` helper.
- `paths_test.go` – unit tests for local and remote validation, traversal
  rejection, and root resolution.

## Integration with the project

- Used by `internal/ssh` in `sftp.go` to validate local and remote paths
  before every SFTP upload, download, directory sync, and listing operation.
- The allowed roots come from `ServerConfig.AllowedLocalPaths` and
  `ServerConfig.AllowedRemotePaths` (see `internal/config`).

## Notes

- Local paths use `path/filepath` (OS-specific semantics, case-insensitive
  comparison on Windows); remote paths use POSIX `path` and must be
  absolute.
- Symlinks are intentionally not expanded to avoid Windows 8.3 short-name
  path issues; `filepath.Clean` is used instead.
- Null bytes in a path are rejected outright.
- When `allowedRoots` is empty for `ValidateRemote`, any absolute POSIX path
  is accepted. The connection manager logs a warning in this case so the
  operator is aware that remote path restriction is disabled.
