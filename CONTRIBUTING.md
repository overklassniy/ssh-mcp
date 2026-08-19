# Contributing to ssh-mcp

Thanks for your interest in contributing to ssh-mcp. This document
describes how to build, test, and submit changes.

## Before you start

- Search the existing issues and pull requests to avoid duplicate work.
- For bugs and features, open an issue first so the approach can be
  discussed before you invest time in code.
- For security vulnerabilities, do not open a public issue. See
  [SECURITY.md](./SECURITY.md) for the reporting process.

## Build and test

ssh-mcp is written in Go and requires Go 1.26 or newer.

```sh
git clone https://github.com/overklassniy/ssh-mcp.git
cd ssh-mcp
make build
```

Run the tests and vet:

```sh
make test
go vet ./...
```

See the [Makefile](./Makefile) for the full list of targets.

## Branches and commits

- Open pull requests against the `main` branch.
- Keep branches focused; one logical change per PR.
- Commit messages must follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).
  Use imperative mood ("add" not "added"), keep the subject under 72
  characters, and add a blank line between the subject and the body.
- Every commit that touches project code must reference the associated
  issue in a footer: `Refs #N` (reference), `Fixes #N` (reference and
  close), or `Closes #N` (reference and close).

Example:

```
feat(tools): add retry flag to execute-command

Adds an optional retry count so transient SSH failures can be retried
without the agent re-issuing the tool call.

Refs #42
```

## Documentation

User documentation lives in the [docs/](./docs) folder and is mirrored
to the GitHub Wiki by the `wiki-sync` workflow. Always edit files in
`docs/`, never in the wiki UI, because wiki-side edits are overwritten
on the next push to `main` that touches `docs/**`.

When a change affects user-facing behavior, update the relevant doc
pages (Installation, Configuration, Tools, Architecture,
Troubleshooting) in the same PR. Incomplete documentation is treated as
a defect.

## Comments and language

- All code comments and documentation must be in English.
- Use plain text only: no emojis, no decorative separators, no
  non-standard Unicode formatting characters.
- Go doc comments use `//` placed directly above the declaration, start
  with the name of the element being documented, and follow standard Go
  doc conventions (no JSDoc or Python-style docstrings).

## Pull requests

- Fill in the pull request template.
- Link the related issue in the PR description (`Refs #N` / `Fixes #N`).
- Make sure `make build` and the test command pass before requesting
  review.
- Keep the PR focused; avoid mixing unrelated changes.

## Security

- Never commit secrets, passwords, passphrases, private keys, or tokens.
- ssh-mcp passes sensitive values through environment variables; do not
  add code that logs or persists them.
- See [SECURITY.md](./SECURITY.md) for vulnerability reporting.