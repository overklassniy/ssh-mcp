# .github

GitHub configuration and community health files for the ssh-mcp
repository.

## Purpose

This folder holds repository-level GitHub configuration: CI/CD
workflows, issue templates, a pull request template, and Dependabot
configuration. It is the integration point between the project and
GitHub's automation and collaboration features.

## Contents

- `ISSUE_TEMPLATE/` - issue forms and templates used when a contributor
  opens a new issue.
  - `bug_report.yml` - structured bug report form (client, platform,
    version, install method, config, reproduction steps, logs).
  - `feature_request.yml` - structured feature request form (problem,
    proposed solution, alternatives).
  - `config_support.md` - markdown template for configuration and
    authentication questions.
  - `config.yml` - issue template chooser configuration; disables blank
    issues and links to the security policy and documentation.
- `PULL_REQUEST_TEMPLATE.md` - template pre-filled when a contributor
  opens a pull request. Mirrors the Conventional Commits and
  issue-referencing rules from `CONTRIBUTING.md`.
- `dependabot.yml` - Dependabot configuration for weekly Go module
  updates at the repository root.
- `workflows/` - GitHub Actions workflows. See
  [workflows/README.md](./workflows/README.md) for details on the
  release, dev-image, and wiki-sync pipelines.

## Integration with the project

- Issue templates reference the supported MCP clients (Claude Desktop,
  Claude Code, Cursor, Devin, VS Code, Cline) and platforms (Windows,
  macOS, Linux) documented in the root `README.md`.
- The PR template and `CONTRIBUTING.md` (at the repository root) share
  the same Conventional Commits and issue-referencing contract.
- `dependabot.yml` targets the root Go module declared in `go.mod`.
- The `workflows/` folder is documented separately in its own README.

## Conventions

- Issue forms use the GitHub issue-forms YAML schema (verified against
  GitHub Docs). Field types are limited to `markdown`, `input`,
  `textarea`, `dropdown`, `checkboxes`, and `upload`.
- All template copy is in English, plain text only, no emojis or
  decorative characters, matching the project documentation rules.
- Secrets are never requested in templates; contributors are reminded to
  redact passwords, passphrases, keys, and 2FA codes.
