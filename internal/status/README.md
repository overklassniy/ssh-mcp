# internal/status

Remote system status collection via a single batched SSH command.

## Purpose

This package collects system status information (hostname, OS, CPU, memory,
disk, GPUs, processes, services) from a remote server. Rather than issuing
many separate SSH commands, it joins all probes into one remote script
delimited by random marker lines, runs it once, and parses the output into a
`ServerStatus` struct. This mirrors the original TypeScript
status-collector implementation.

## Contents

- `collector.go` - the `ServerStatus` struct (JSON-tagged fields for all
  collected metrics); the `CommandRunner` function type (runs a command on a
  named server and returns stdout); `CollectSystemStatus` (filters probes
  through the policy, builds the batched script, runs it, and parses the
  output); `buildProbeCommands` (the map of field names to probe commands);
  `buildStatusScript` (joins probes with marker lines); and
  `parseStatusScriptOutput` (extracts field values from the marker-delimited
  output).
- `collector_test.go` - unit tests for script building, output parsing, and
  policy filtering.

## Integration with the project

- Invoked by `internal/mcp/tools/serverstatus.go`, which passes the
  connection manager's command runner and the server's policy to
  `CollectSystemStatus`.
- Depends on `internal/policy`: each probe command is validated against the
  server's command policy before it is included in the batched script, so a
  restrictive whitelist cannot be bypassed by the status tool.

## Notes

- A random marker prefix is generated per call to avoid collisions with
  command output.
- If the batched command fails, every field is left unset and `reachable` is
  still reported as `true`, matching the original behavior; the caller is
  expected to interpret a mostly-empty status as a probe failure.
- The probe commands assume a Linux remote with common utilities available
  (`ip`, `lscpu` or `/proc/cpuinfo`, `nvidia-smi` for GPUs, `systemctl` or
  `/etc/init.d` for services, `free`, `df`, `ps`, `top`, `uname`). Missing
  utilities produce empty values rather than errors because each probe is
  wrapped to suppress stderr.
- Loopback addresses (`127.x`) are filtered out of the collected IP
  addresses.
