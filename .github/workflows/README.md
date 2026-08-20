# .github/workflows

GitHub Actions workflows for building, releasing, publishing, and
documenting ssh-mcp.

## Purpose

This folder holds the CI/CD pipelines that automate binary releases,
container image publishing, and wiki documentation sync. Together they
cover five cadences: continuous integration on every push and pull
request, stable releases triggered by git tags, continuous dev images
triggered by pushes to the master branch, wiki documentation mirroring
triggered by changes to the docs/ folder, and Docker Hub overview sync
triggered by changes to DOCKERHUB.md.

## Contents

- `ci.yml` – continuous integration pipeline. Runs `go build`, `go vet`,
  and `go test -short` on every push to master and every pull request
  targeting master. Validates Dependabot dependency bumps and feature
  branches before merge.
- `release.yml` – tag-triggered release pipeline. Runs goreleaser to build
  cross-platform binaries and create a draft GitHub Release, then builds a
  multi-arch Docker image (linux/amd64, linux/arm64) and pushes it to GHCR
  and Docker Hub with semver tags (:version, :major, :major.minor, :latest).
- `dev-image.yml` – master-branch dev image pipeline. Builds and pushes a
  multi-arch Docker image tagged :dev to both registries. No GitHub Release
  is produced. The image embeds a version string of the form
  `dev-<short-sha>`.
- `wiki-sync.yml` – master-branch documentation sync pipeline. Mirrors the
  contents of the `docs/` folder to the repository's GitHub Wiki using
  `Andrew-Chen-Wang/github-wiki-action`. Runs on any push to master that
  touches `docs/**` or the workflow file itself, and on manual dispatch.
- `dockerhub-sync.yml` – master-branch Docker Hub overview sync pipeline.
  Pushes the root `DOCKERHUB.md` to the Docker Hub repository overview
  (the "Overview" tab) and sets the short description using
  `peter-evans/dockerhub-description@v5.0.0`. Runs on any push to master
  that touches `DOCKERHUB.md` or the workflow file itself, and on manual
  dispatch. Does not sync the logo (Docker Hub has no logo upload API;
  see `assets/readme/README.md`).

## Triggers

| Workflow | Trigger | Produces |
| --- | --- | --- |
| `ci.yml` | Push to `master` or pull request targeting `master` | Build, vet, and test results |
| `release.yml` | Push of a `v*` tag (e.g. `v1.0.0`) | Draft GitHub Release + `:version`, `:major`, `:major.minor`, `:latest` images |
| `dev-image.yml` | Push to `master` branch | `:dev` image only |
| `wiki-sync.yml` | Push to `master` touching `docs/**` | Updated GitHub Wiki mirroring `docs/` |
| `dockerhub-sync.yml` | Push to `master` touching `DOCKERHUB.md` | Updated Docker Hub overview and short description |

## Required secrets

GHCR authentication uses the automatic `GITHUB_TOKEN` provided by GitHub
Actions, so no extra secret is needed for GHCR pushes.

Docker Hub requires two repository secrets (add them under Settings >
Secrets and variables > Actions):

- `DOCKERHUB_USERNAME` – the Docker Hub account name to push as.
- `DOCKERHUB_TOKEN` – a Docker Hub access token (not the account password).

If the Docker Hub secrets are missing, the Docker Hub login step fails
gracefully (`continue-on-error: true`) and the GHCR push still succeeds.
Only the Docker Hub mirror is skipped.

## Registries

Images are published to both registries for discoverability:

- GHCR (primary): `ghcr.io/overklassniy/ssh-mcp`
- Docker Hub (mirror): `overklassniy/ssh-mcp`

The Docker Hub image name can be overridden via the `DOCKERHUB_IMAGE`
repository variable if a different namespace is used.

## Wiki sync

`wiki-sync.yml` mirrors the `docs/` folder to the repository's GitHub
Wiki. The `docs/` folder is the single source of truth for user
documentation; the wiki is a rendered mirror.

- Action: `Andrew-Chen-Wang/github-wiki-action@v5` with `strategy: clone`
  (appends a commit to the wiki repo history, no force-push) and
  `preprocess: true` (rewrites relative links to wiki bare links and
  repo-relative links to `blob/`/`raw/` URLs pinned to the pushed commit).
- Authentication: the automatic `GITHUB_TOKEN` with
  `permissions: contents: write`. Same-repo wiki pushes do not require a
  PAT. If the repository's default workflow permissions are set to
  read-only (Settings > Actions > General > Workflow permissions), the
  push fails; either allow workflows to override the default or set the
  repo default to read-write.
- `docs/README.md` is excluded from the sync via the `ignore` input,
  because it only describes the folder locally and is not wiki content.
- `docs/Home.md` becomes the wiki landing page (GitHub wikis use
  `Home.md`, not `README.md`, for the homepage).
- `docs/_Sidebar.md` controls the wiki left navigation shown on every
  page.

### One-time manual prerequisite

Before the first run, create a dummy page in the wiki via the GitHub
web UI (repo > Wiki > Create the first page). This initializes the
`.wiki.git` backend that the workflow pushes to. Without it, the first
run fails because the wiki repo does not exist yet.

### Editing workflow

Edit documentation in `docs/`, never in the wiki UI. Edits made in the
wiki UI are overwritten on the next push to `master` that touches
`docs/**`. To pull wiki-side edits back into `docs/`, use the action's
`direction: pull` mode in a separate workflow (not configured here).

## Integration with the project

- `release.yml` and `dev-image.yml` build from the root `Dockerfile`, a
  multi-stage distroless build described in the root project
  documentation.
- `release.yml` uses the existing `goreleaser.yml` configuration for
  binary builds, so the two distribution paths (binaries and containers)
  share the same version string and build flags.
- The `VERSION` build arg is passed through to the Dockerfile and injected
  into the binary via `-ldflags`, matching the goreleaser ldflags.
- `wiki-sync.yml` reads from the `docs/` folder, whose contents and
  purpose are described in `docs/README.md`.
- `dockerhub-sync.yml` reads the root `DOCKERHUB.md`, a Docker
  Hub-adapted mirror of the GitHub README with absolute URLs and a
  Docker-focused quickstart. It does not read the GitHub `README.md`
  directly, because that file uses HTML align blocks and relative links
  that Docker Hub's Markdown renderer handles inconsistently. The
  Docker Hub repository logo is uploaded manually once (see
  `assets/readme/README.md`); the workflow only syncs the text overview
  and short description.

## Notes

- Multi-arch builds use QEMU + Docker Buildx. The `cache-from`/`cache-to`
  `type=gha` setting stores build cache in GitHub Actions cache to speed
  up subsequent runs.
- The dev image workflow does not depend on goreleaser, so it runs quickly
  and independently of the release pipeline.
- Goreleaser's `before.hooks` run `go mod tidy`; ensure `go.sum` is
  committed clean before tagging, or the hook may report a dirty tree.
- The wiki sync workflow uses `disable-empty-commits: true`, so a run
  that would produce no wiki changes does not create an empty commit.
