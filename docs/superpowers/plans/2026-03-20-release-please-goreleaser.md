# Release Infrastructure Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Set up release-please + goreleaser for automated versioning, changelog, cross-platform binary distribution, Homebrew tap, and Docker image publishing.

**Architecture:** release-please creates release PRs on push to main; merging creates a tag; goreleaser triggers on the tag to build binaries, publish to Homebrew tap, and push Docker image to GHCR. cocogito (already in place) validates commit messages.

**Tech Stack:** GitHub Actions, release-please, goreleaser, Docker, Homebrew

**Spec:** `docs/superpowers/specs/2026-03-20-release-please-goreleaser-design.md`

**Prerequisites (manual, before executing):**

1. Create `specgraph/homebrew-tap` repo in the org (empty, public)
2. Create `HOMEBREW_TAP_TOKEN` fine-grained PAT with write access to `specgraph/homebrew-tap`, store as repo secret on `specgraph/specgraph`
3. Enable branch protection on `main` with required status checks

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `.github/workflows/release.yml` | release-please action |
| `.github/workflows/goreleaser.yml` | goreleaser action on tag push |
| `.goreleaser.yaml` | Build, archive, brew, docker config |
| `release-please-config.json` | release-please settings |
| `.release-please-manifest.json` | Version tracking |

### Modified Files

| File | Changes |
|------|---------|
| `cmd/specgraph/main.go` | Add version/commit/date vars, wire `rootCmd.Version` |

### Existing Files (no changes)

| File | Notes |
|------|-------|
| `cog.toml` | Already complete |
| `Dockerfile` | Already exists, goreleaser references it |

---

## Chunk 1: Config Files and Workflows

### Task 1: Create release-please Config

**Files:**

- Create: `release-please-config.json`
- Create: `.release-please-manifest.json`

- [ ] **Step 1: Create release-please-config.json**

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

- [ ] **Step 2: Create .release-please-manifest.json**

```json
{
  ".": "0.0.0"
}
```

- [ ] **Step 3: Commit**

```bash
jj commit -m "ci: add release-please config and manifest"
```

### Task 2: Create release-please Workflow

**Files:**

- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Create the workflow**

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

- [ ] **Step 2: Verify YAML syntax**

```bash
yamlfmt -lint .github/workflows/release.yml
```

Expected: No issues

- [ ] **Step 3: Commit**

```bash
jj commit -m "ci: add release-please workflow"
```

### Task 3: Create goreleaser Config

**Files:**

- Create: `.goreleaser.yaml`

- [ ] **Step 1: Create .goreleaser.yaml**

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

- [ ] **Step 2: Validate goreleaser config**

```bash
goreleaser check
```

Expected: `0 errors, 0 warnings` (or install goreleaser first: `go install github.com/goreleaser/goreleaser/v2@latest`)

- [ ] **Step 3: Commit**

```bash
jj commit -m "ci: add goreleaser config"
```

### Task 4: Create goreleaser Workflow

**Files:**

- Create: `.github/workflows/goreleaser.yml`

- [ ] **Step 1: Create the workflow**

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

- [ ] **Step 2: Verify YAML syntax**

```bash
yamlfmt -lint .github/workflows/goreleaser.yml
```

- [ ] **Step 3: Commit**

```bash
jj commit -m "ci: add goreleaser workflow"
```

---

## Chunk 2: Version Injection and Verification

### Task 5: Add Version Variables to CLI

**Files:**

- Modify: `cmd/specgraph/main.go`

- [ ] **Step 1: Add version variables and wire to rootCmd**

Add after the `package main` import block:

```go
// Set by goreleaser ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)
```

Add in the `init()` function (or right after `rootCmd` declaration):

```go
rootCmd.Version = version
```

This enables `specgraph --version` to print the version. Cobra formats it as `specgraph version <version>`.

- [ ] **Step 2: Verify build and --version**

```bash
go build -o specgraph ./cmd/specgraph && ./specgraph --version
```

Expected: `specgraph version dev`

- [ ] **Step 3: Test with ldflags**

```bash
go build -ldflags "-X github.com/specgraph/specgraph/cmd/specgraph.version=0.1.0-test" -o specgraph ./cmd/specgraph && ./specgraph --version
```

Expected: `specgraph version 0.1.0-test`

- [ ] **Step 4: Clean up test binary**

```bash
rm -f specgraph
```

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat: add --version flag with goreleaser ldflags injection"
```

### Task 6: Local Goreleaser Build Test

- [ ] **Step 1: Run goreleaser snapshot build**

```bash
goreleaser build --snapshot --clean
```

Expected: Builds 4 binaries in `dist/` (linux/darwin × amd64/arm64). No publishing.

- [ ] **Step 2: Verify one of the snapshot binaries**

```bash
./dist/specgraph_linux_amd64_v1/specgraph --version 2>/dev/null || ./dist/specgraph_darwin_arm64/specgraph --version
```

Expected: Prints version with snapshot suffix

- [ ] **Step 3: Clean up**

```bash
rm -rf dist/
```

- [ ] **Step 4: Test Docker build**

```bash
docker build -t specgraph-test . && docker run --rm specgraph-test --version
```

Expected: Prints `specgraph version dev` (no ldflags in plain docker build)

```bash
docker rmi specgraph-test
```

### Task 7: Full Verification

- [ ] **Step 1: Run task check**

```bash
task check
```

Expected: PASS (fmt, lint, build, unit tests)

- [ ] **Step 2: Run task pr-prep**

```bash
task pr-prep
```

Expected: PASS (all tests including integration and e2e)

- [ ] **Step 3: Final commit if any fixups needed**

```bash
jj commit -m "fix: address lint/test issues from release infrastructure"
```
