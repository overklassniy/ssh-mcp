# Security policy

## Supported versions

ssh-mcp is a single-line project: only the latest release receives
security fixes. There are no separate maintenance branches.

| Version | Supported |
| --- | --- |
| Latest release | Yes |
| Older releases | No |
| `:dev` image / `main` branch | Best effort, not for production |

## Reporting a vulnerability

Do not open a public GitHub issue for a security vulnerability.

To report a vulnerability, use one of these channels:

1. **GitHub private security advisory** – go to
   [Security > Advisories](https://github.com/overklassniy/ssh-mcp/security/advisories/new)
   and create a private report. This is the preferred channel.
2. **Email** – if you cannot use the advisory flow, contact the
   maintainer through the GitHub profile listed on the repository page.

Please include:

- A description of the vulnerability and its impact.
- The affected version or commit.
- Steps to reproduce, including config (with secrets redacted).
- Any suggested fix or mitigation.

You will receive an acknowledgment within a reasonable timeframe. Please
do not disclose the vulnerability publicly until a fix has been released
or you have been asked to do so.

## Scope

This policy covers the `ssh-mcp` server, its configuration parsing, and
its SSH/SFTP handling. It does not cover vulnerabilities in the
underlying `golang.org/x/crypto/ssh` library or the Go runtime; report
those to the respective upstream projects.

## Known security trade-offs

ssh-mcp does not verify remote host keys (`InsecureIgnoreHostKey`). This
is intentional: the server is designed to be pointed at operator-owned
machines where host key management is handled out of band, and the
primary security boundary is the command policy and path restrictions,
not host key verification. Reports about the missing host key check will
be treated as a known design trade-off, not a new vulnerability.

Security is enforced through two operator-configured mechanisms,
documented in the README:

- **Command policy** – `whitelist`/`blacklist` regex patterns gate every
  command, including the probe commands used by `server-status`.
- **Path restrictions** – `allowed_local_paths` and
  `allowed_remote_paths` confine SFTP operations; remote paths must be
  absolute POSIX paths.

Sensitive values (password, passphrase, 2FA code) are passed via
environment variables and are never accepted on the command line.
