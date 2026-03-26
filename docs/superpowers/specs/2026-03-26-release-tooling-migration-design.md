# Release Tooling Migration: git-cliff + goreleaser v2

**Date**: 2026-03-26
**Status**: Approved
**Supersedes**: 2026-03-20-release-please-goreleaser-design.md

## Problem

The current release-please + goreleaser pipeline is fragile and high-ceremony:

- **6 fix PRs** were needed to coordinate release-please and goreleaser ownership of tags, drafts, and releases.
- **Duplicate changelogs** — release-please generates CHANGELOG.md, goreleaser generates GitHub release notes. Different formats, duplicate entries.
- **Release PR dance** — manifest file churn (`.release-please-manifest.json` committed on every release), extra PRs, `Release-As` escape hatches.
- **PAT required** — `RELEASE_PLEASE_TOKEN` needed to trigger downstream CI from release PRs.
- v0.2.0 and v0.2.1 releases both got stuck as drafts due to handoff issues.

## Decision

Replace release-please with **git-cliff** for version computation and changelog generation. Migrate **goreleaser** to v2 APIs. Use `workflow_dispatch` (manual trigger) instead of automatic releases on every push to main.

### Tool Responsibilities

| Tool | Owns |
|------|------|
| **git-cliff** | Version computation (semver from conventional commits), CHANGELOG.md generation, git tag creation, GitHub release creation (with changelog body) |
| **goreleaser v2** | Binary builds, archives, Docker images, Homebrew formula, cosign signing, SBOM generation, provenance attestation |

goreleaser no longer touches changelogs or creates GitHub releases. It appends artifacts to the release git-cliff created.

## Workflow Architecture

### Trigger

`workflow_dispatch` only. No automatic releases on push to main. The operator decides when to cut a release.

### Pipeline

```text
workflow_dispatch
    │
    ▼
┌─────────────────────────────┐
│  Job 1: release             │
│  1. Checkout (full history) │
│  2. Install git-cliff       │
│  3. Compute next version    │
│  4. Generate CHANGELOG.md   │
│  5. Sed version in site docs│
│  6. Commit + push           │
│  7. Create + push tag       │
│  8. Create GitHub release   │
│  9. Output: new_tag, version│
└─────────────┬───────────────┘
              │ if new_tag != ''
              ▼
┌─────────────────────────────┐
│  Job 2: goreleaser          │
│  1. Checkout at tag SHA     │
│  2. Build web UI (pnpm)     │
│  3. Setup Go + cache        │
│  4. goreleaser release      │
│     - binaries (4 targets)  │
│     - Docker images (2 arch)│
│     - Docker manifest       │
│     - Homebrew formula      │
│     - cosign signing        │
│     - SBOM generation       │
│     - Provenance attestation│
│  5. Trivy image scan        │
└─────────────────────────────┘
```

### Job Output Mechanism

Job 1 sets GitHub Actions outputs via `echo "new_tag=v0.3.0" >> "$GITHUB_OUTPUT"`:

- `new_tag` — the tag name (e.g., `v0.3.0`), or empty string if no releasable commits
- `version` — the version without `v` prefix (e.g., `0.3.0`)

Job 2 consumes them via `needs.release.outputs.new_tag`. Job 2 checks out at the tag SHA (not HEAD, since Job 1 pushed a commit + tag after HEAD).

### Partial Failure Handling

If goreleaser fails after git-cliff succeeds, the tag and GitHub release exist but lack artifacts. To recover: re-run the workflow — Job 1 will detect no new releasable commits since the tag already exists, so it exits early. Instead, re-trigger goreleaser directly via `gh workflow run release.yml` or manually run goreleaser against the existing tag.

### Version Source of Truth

Git tags only. No manifest file, no version file committed to the repo. The binary gets its version from goreleaser ldflags (existing behavior).

## git-cliff Configuration

### `cliff.toml`

**Conventional commits parsing:**

- `feat` → minor bump (pre-1.0), minor (post-1.0)
- `fix` → minor bump (pre-1.0), patch (post-1.0)
- `feat!` / `BREAKING CHANGE` → minor bump (pre-1.0), major (post-1.0)

**Changelog sections (grouped):**

- Features (`feat`)
- Bug Fixes (`fix`)
- Performance (`perf`)
- Code Refactoring (`refactor`)
- Tests (`test`)
- CI (`ci`)
- Build (`build`)
- Miscellaneous (everything else that passes the filter)

**Filtered out (not shown in changelog):**

- `chore` commits
- `docs` commits
- Merge commits
- Old `chore(main): release` commits from release-please

**Formatting:**

- Scope rendered as bold prefix: `feat(web): add graph page` → "**web:** add graph page"
- Commit SHA links to GitHub
- PR numbers extracted from squash-merge messages (`(#123)`) and linked

**Tag pattern:** `v*` prefix convention.

**Changelog generation mode:** Full regeneration from all tags matching `v*`. Each release rewrites CHANGELOG.md from complete git history. The old release-please format will be replaced with a consistent git-cliff format across all versions.

**Release commit message:** `chore(release): vX.Y.Z` — matches the `chore` filter so the release commit itself never appears in the changelog.

## goreleaser v2 Migration

### Deprecated → New API Keys

| Deprecated | New | Notes |
|---|---|---|
| `archives[].format` | `archives[].formats` | Plural, takes a list |
| `brews` | `homebrew_casks` | Renamed section |
| `dockers` + `docker_manifests` | `dockers_v2` | Unified multi-platform |
| `release.use_existing_draft: true` | `release.mode: append` | Append artifacts to git-cliff's release |
| `changelog.use: github` | `changelog.disable: true` | git-cliff owns changelog |

### goreleaser Role (Narrowed)

Build binaries, package archives, build+push Docker images, sign, generate SBOMs, attest provenance, push Homebrew formula. Does not create or manage GitHub releases or changelogs.

## Site Docs Version Handling

Replace hardcoded version strings and `<!-- x-release-please-version -->` markers with `{{VERSION}}` placeholders. The release job seds them with the computed version at commit time.

**Affected files:**

- `site/docs/quickstart.md` — download link, docker pull command
- `site/docs/index.md` — footer version

## File Changes

### Delete

- `release-please-config.json`
- `.release-please-manifest.json`

### Add

- `cliff.toml` — git-cliff configuration

### Modify

- `.github/workflows/release.yml` — rewrite: workflow_dispatch trigger, git-cliff release job, goreleaser v2 job
- `.goreleaser.yaml` — migrate to v2 API keys, disable changelog, set `release.mode: append`
- `site/docs/quickstart.md` — replace version with `{{VERSION}}` placeholder
- `site/docs/index.md` — replace version with `{{VERSION}}` placeholder

### Secrets

- `GITHUB_TOKEN` — keep (already exists)
- `HOMEBREW_TAP_TOKEN` — keep
- `RELEASE_PLEASE_TOKEN` — becomes unused (clean up after migration)

## Migration Strategy

1. Implement all changes in a single PR
2. Merge to main
3. Manually trigger `workflow_dispatch` to test the new pipeline
4. Verify: CHANGELOG.md updated, tag created, GitHub release with changelog, goreleaser artifacts attached, Docker images pushed, Homebrew formula updated
5. Clean up `RELEASE_PLEASE_TOKEN` secret after confirming the new pipeline works

## Risks

- **Existing release history**: git-cliff can regenerate the full CHANGELOG.md from git history, but formatting will differ from release-please output. This is acceptable — the new format will be consistent going forward.
- **First release after migration**: The workflow_dispatch trigger means we control when this fires. Low risk.
- **goreleaser v2 API changes**: Well-documented migration paths exist. The changes are mechanical renames, not behavioral.
