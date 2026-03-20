# Release Infrastructure: release-please + goreleaser + cocogito

**Status:** Approved
**Date:** 2026-03-20
**Bead:** spgr-sdw

## Problem

SpecGraph has no release infrastructure. Approaching 0.1.0, we need automated versioning, changelog generation, cross-platform binary distribution, a Homebrew tap, and a Docker image.

## Decision

Use release-please for versioning and changelog, goreleaser for builds and distribution, and cocogito (already in place) for commit validation.

## Components

### release-please

GitHub Action that runs on every push to main. Reads conventional commit messages and opens a release PR with a version bump and generated `CHANGELOG.md`. When the release PR is merged, release-please creates a GitHub Release with a git tag (e.g., `v0.1.0`).

Configuration:

- `release-type: go` — updates version references for Go modules
- Initial version: `0.0.0` in manifest, so first release becomes `0.1.0`
- Changelog sections mapped to match the commit types in the existing `cog.toml`

### goreleaser

Triggered by tag push (from release-please). Builds cross-platform binaries, publishes distribution artifacts.

**Build targets:**

- `linux/amd64`, `linux/arm64`
- `darwin/amd64`, `darwin/arm64`

**Distribution channels:**

1. **GitHub Release** — tarballs + checksums attached to the release
2. **Homebrew tap** — formula pushed to `specgraph/homebrew-tap` repo
3. **Docker image** — built from existing `Dockerfile`, pushed to `ghcr.io/specgraph/specgraph`

**Binary:** `specgraph` from `./cmd/specgraph`

### cocogito

Already in place. `cog.toml` exists with `ignore_merge_commits = true` and a full `[commit_types]` table. Enforced via lefthook `commit-msg` hook. No changes needed — cog validates commits, release-please handles versioning and changelog.

## Workflow

```text
push to main
  → release-please opens/updates release PR
  → PR contains: version bump + CHANGELOG.md update
  → merge release PR when ready
  → release-please creates GitHub Release + tag (v0.x.y)
  → tag push triggers goreleaser
    → cross-compile binaries (4 targets)
    → upload to GitHub Release
    → push Homebrew formula to specgraph/homebrew-tap
    → build + push Docker image to ghcr.io/specgraph/specgraph
```

**Safety:** Branch protection rules on `main` (required status checks) ensure goreleaser only runs on code that passed CI. The tag is created by merging the release PR, which requires CI to pass.

## Files

### New files

| File | Purpose |
|------|---------|
| `.github/workflows/release.yml` | release-please action (push to main trigger) |
| `.github/workflows/goreleaser.yml` | goreleaser action (tag push trigger) |
| `.goreleaser.yaml` | goreleaser build/archive/brew/docker config |
| `.release-please-manifest.json` | Version tracking (`{"." : "0.0.0"}`) |
| `release-please-config.json` | release-please config (release-type, changelog sections) |

### Existing files (no changes)

| File | Status |
|------|--------|
| `cog.toml` | Already complete — commit types, ignore_merge_commits |
| `Dockerfile` | Already exists — golang:1.26-alpine builder, alpine:3.23 runtime, EXPOSE 9090, CMD serve |

## File Details

### .github/workflows/release.yml

```yaml
# SPDX-License-Identifier: MIT
# Copyright 2026 Sean Brandt
name: Release Please
on:
  push:
    branches: [main]
permissions:
  contents: write
  pull-requests: write
jobs:
  release-please:
    runs-on: ubuntu-latest
    steps:
      - uses: googleapis/release-please-action@v4
        with:
          token: ${{ secrets.GITHUB_TOKEN }}
```

release-please reads `release-please-config.json` and `.release-please-manifest.json` automatically.

### .github/workflows/goreleaser.yml

```yaml
# SPDX-License-Identifier: MIT
# Copyright 2026 Sean Brandt
name: Release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
  packages: write
jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
```

### .goreleaser.yaml

```yaml
# SPDX-License-Identifier: MIT
# Copyright 2026 Sean Brandt
version: 2

builds:
  - id: specgraph
    main: ./cmd/specgraph
    binary: specgraph
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w
      - -X github.com/specgraph/specgraph/cmd/specgraph.version={{.Version}}
      - -X github.com/specgraph/specgraph/cmd/specgraph.commit={{.ShortCommit}}
      - -X github.com/specgraph/specgraph/cmd/specgraph.date={{.Date}}

archives:
  - id: default
    format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - README.md
      - LICENSE

checksum:
  name_template: checksums.txt
  algorithm: sha256

brews:
  - name: specgraph
    repository:
      owner: specgraph
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    homepage: https://specgraph.io
    description: Live spec-driven development framework
    license: MIT
    install: |
      bin.install "specgraph"

dockers:
  - image_templates:
      - "ghcr.io/specgraph/specgraph:{{ .Version }}"
      - "ghcr.io/specgraph/specgraph:latest"
    dockerfile: Dockerfile
    use: buildx
    build_flag_templates:
      - "--platform=linux/amd64"
      - "--label=org.opencontainers.image.title={{ .ProjectName }}"
      - "--label=org.opencontainers.image.version={{ .Version }}"
      - "--label=org.opencontainers.image.source=https://github.com/specgraph/specgraph"

changelog:
  use: github
  groups:
    - title: Features
      regexp: '^feat'
    - title: Bug Fixes
      regexp: '^fix'
    - title: Documentation
      regexp: '^docs'
    - title: Performance
      regexp: '^perf'
    - title: Other
      order: 999
```

Note: goreleaser's `changelog` section is used when goreleaser creates the release notes. Since release-please creates the release first and goreleaser appends artifacts, goreleaser's changelog is secondary — release-please's changelog in `CHANGELOG.md` is the primary record.

### release-please-config.json

```json
{
  "packages": {
    ".": {
      "release-type": "go",
      "bump-minor-pre-major": true,
      "bump-patch-for-minor-pre-major": true,
      "changelog-sections": [
        {"type": "feat", "section": "Features"},
        {"type": "fix", "section": "Bug Fixes"},
        {"type": "docs", "section": "Documentation"},
        {"type": "perf", "section": "Performance"},
        {"type": "refactor", "section": "Code Refactoring"},
        {"type": "test", "section": "Tests"},
        {"type": "ci", "section": "CI"},
        {"type": "build", "section": "Build"},
        {"type": "chore", "section": "Miscellaneous"}
      ]
    }
  }
}
```

`changelog-sections` maps all commit types allowed by `cog.toml` to changelog headings.

### .release-please-manifest.json

```json
{
  ".": "0.0.0"
}
```

### Version injection

The `ldflags` in `.goreleaser.yaml` inject version info into the binary. The CLI needs corresponding variables. Add to `cmd/specgraph/main.go`:

```go
var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)
```

And wire into the root command's `Version` field so `specgraph --version` prints the release version.

## Prerequisites

1. **Create `specgraph/homebrew-tap` repo** in the org (empty, public)
2. **Create `HOMEBREW_TAP_TOKEN` secret** — fine-grained PAT with write access to `specgraph/homebrew-tap`. Store as a repository secret on `specgraph/specgraph`.
3. **Branch protection on `main`** — require status checks to pass before merge. This ensures goreleaser only publishes code that passed CI.

## Testing

- **release-please:** Verify the workflow runs on push to main and opens a release PR. Merge it and confirm the tag + release are created.
- **goreleaser:** Run `goreleaser check` locally to validate `.goreleaser.yaml`. Run `goreleaser build --snapshot --clean` to test builds without publishing.
- **Docker:** Build locally with `docker build -t specgraph .` and verify `docker run specgraph --help` works.
- **Version:** After goreleaser build, verify `./specgraph --version` prints the injected version.

## Alternatives Considered

### GoReleaser only (no release-please)

goreleaser can generate changelogs, but it doesn't manage version bumping or create release PRs. You'd need to manually create and push tags. release-please automates this.

### Semantic-release instead of release-please

More complex, requires Node.js. release-please is simpler, Go-native, and well-maintained by Google.

### cog bump instead of release-please

cog can bump versions and generate changelogs, but it doesn't create release PRs or integrate with GitHub Releases as cleanly. Using release-please for release management and cog for commit validation gives each tool one clear job.
