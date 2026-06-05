# Release Pipeline: Single-Job, GoReleaser-Owns-Release

**Date**: 2026-06-05
**Status**: Approved
**Bead**: spgr-7r6g
**Supersedes**: 2026-03-26-release-tooling-migration-design.md

## Problem

Every **published** release since **v0.3.7** has had **empty release notes**. The
real notes exist, but only on an orphaned **draft** release that never gets
published. Releases v0.3.7, v0.4.0, v0.5.0, v0.6.0, and v0.7.0 are all affected;
v0.3.6 and earlier were correct.

### Root cause

The current pipeline (`.github/workflows/release.yml`) has **two independent
release-creation paths for the same tag** that do not coordinate:

1. **`release` job** runs `gh release create "$TAG" --notes "$NOTES" --draft` —
   a draft carrying the git-cliff changelog body.
2. **`goreleaser` job** runs `goreleaser release --clean`. GoReleaser looks up
   the existing release for the tag via the GitHub *get-release-by-tag* API,
   **which does not return draft releases**. Concluding none exists, GoReleaser
   **creates its own release** with an empty body (`changelog.disable: true`,
   no notes supplied) and uploads the assets there. It publishes immediately
   (`release.draft` defaults to false).
3. The final `gh release edit "$TAG" --draft=false` targets the
   already-published GoReleaser release — a no-op. The notes-bearing draft is
   orphaned forever.

The defect was introduced by the 2026-03-26 migration. Its "Deprecated → New API
Keys" table (line 139) claimed:

| Deprecated                          | New                    |
|-------------------------------------|------------------------|
| `release.use_existing_draft: true`  | `release.mode: append` |

**This equivalence is false.** The two options are unrelated:

- `use_existing_draft: true` — makes GoReleaser **find and adopt an existing
  draft** as its target release. This is what implements "append to git-cliff's
  release." It is current (GoReleaser v2.5+), not deprecated.
- `mode: append` — only governs **artifact handling** (append vs replace files)
  *within whatever release GoReleaser is already targeting*. It does **not**
  make GoReleaser locate a draft.

Removing `use_existing_draft` based on this false equivalence severed the link
between the draft and GoReleaser. The migration's documented intent — "goreleaser
appends artifacts to the release git-cliff created" — was never actually achieved.

## Decision

Rather than patch the two-job draft architecture (which remains fragile —
ordering, token handoff, draft visibility, orphan cleanup), **adopt the
single-job model proven in the holomush repo**: one release creator, no drafts,
no cross-job handoff.

### Tool responsibilities (new)

| Tool | Owns |
|------|------|
| **cog** (cocogitto) | Version computation from conventional commits; creates a **local** `v*` tag only (no bump commit, no in-repo changelog) |
| **GoReleaser v2** | Creates the **remote** tag (via `target_commitish`) **and the GitHub Release with its native changelog notes**; binary builds, archives, Docker images, Homebrew cask, cosign signing, SBOMs |

GoReleaser is the **single** owner of the GitHub Release. There is no draft, no
`gh release create`, no `gh release edit`, no `use_existing_draft`, no
`mode: append`, and no git-cliff. Because GoReleaser internally uploads to a
draft and flips to published only after all assets are attached, there is never
a notes-without-binaries window — and a second release path cannot exist.

### What changes from today

| Concern | Today (broken) | New (holomush model) |
|---------|----------------|----------------------|
| Version derivation | git-cliff `--bump` | **cog** `bump --auto --dry-run` (guard) + `bump --auto --disable-bump-commit` (local tag) |
| Tag | git-cliff creates + pushes remote tag | cog creates **local** tag; GoReleaser creates remote tag via `target_commitish: "{{ .Commit }}"` |
| Committed `CHANGELOG.md` | regenerated + pushed every release | **frozen** — no longer updated (existing file kept as historical record) |
| GitHub Release | draft (git-cliff) + empty published (GoReleaser) | **single** release created by GoReleaser |
| Release notes | git-cliff body, stranded in draft | **GoReleaser native changelog** (grouped, filtered) |
| Jobs | `release` + `goreleaser` (2 jobs) | **1 job** (`goreleaser`) + downstream attest/scan |
| git-cliff | `cliff.toml` + `orhun/git-cliff-action` | **removed** |

### Accepted trade-offs

- **No in-repo `CHANGELOG.md` going forward.** Notes live on the GitHub Release.
  The existing `CHANGELOG.md` is frozen (kept, not deleted, to preserve history)
  with a header note pointing readers to GitHub Releases. Rationale: the
  committed changelog was a source of merge churn and a second format to
  maintain; the GitHub Release is the canonical notes surface.
- **Plainer notes format.** GoReleaser's native changelog (grouped by
  conventional-commit type, with exclude filters) is less ornate than
  git-cliff's bold-scope + PR/commit-link template. Accepted in exchange for a
  structurally sound, single-owner pipeline.

## Target Configuration

### `cog.toml` (rewrite)

cog becomes the release engine. Base on the holomush-proven cog-7 form (verified
against the installed `cog 7.0.0`):

```toml
# cog is the release engine: derives next semver from conventional commits and
# creates a TAG ONLY (no bump commit, no in-repo CHANGELOG.md). GitHub Release
# notes are generated by GoReleaser (see .goreleaser.yaml). cog is also used for
# commit-msg validation (lefthook commit-msg hook: `cog verify`).
tag_prefix = "v"
disable_changelog = true
branch_whitelist = ["main"]
ignore_merge_commits = true
```

Notes:

- Drop the `[settings]` wrapper and the redundant `[commit_types]` block. All
  listed types (feat, fix, docs, style, refactor, perf, test, build, ci, chore,
  revert) are cog's built-in defaults. Per the cog-7 footgun, an empty
  `feat = {}` entry can *disable* a type — so do not re-list defaults. Only add a
  `[commit_types]` entry (with a **non-empty** table) to introduce a *new* type.
- `branch_whitelist = ["main"]` is defense-in-depth: a stray `workflow_dispatch`
  from a feature branch cannot cut a release.
- The existing lefthook `commit-msg` hook (`cog verify --file {1}`) keeps working
  — `cog verify` does not require version-control settings.

### `.goreleaser.yaml` (modify `release:` + `changelog:`)

```yaml
release:
  github:
    owner: specgraph
    name: specgraph
  draft: false
  prerelease: auto
  # cog creates the v* tag LOCALLY (cog bump --disable-bump-commit) and does NOT
  # push it before GoReleaser runs. target_commitish tells the Releases API which
  # commit to create the tag from, so GoReleaser creates the remote tag as part
  # of publishing — atomically. Without this, `goreleaser release` against a
  # local-only tag fails at the API call.
  target_commitish: "{{ .Commit }}"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
      - "^ci:"
      - "^build:"
      - "Merge pull request"
      - "Merge branch"
```

Everything else in `.goreleaser.yaml` (builds, archives, checksum,
homebrew_casks, dockers_v2, source, sboms, signs, docker_signs) is unchanged.

### `.github/workflows/release.yml` (rewrite to single job)

`workflow_dispatch` only, with an optional `expected_increment` guard input.
Single `goreleaser` job:

1. Checkout (`fetch-depth: 0`, `persist-credentials: false`).
2. Mint GitHub App token (`RELEASE_APP_ID` / `RELEASE_APP_PRIVATE_KEY`),
   scoped to `contents: write` + `packages: write`.
3. Install cog (pinned binary).
4. Configure git identity for tagging.
5. Preview + guard version bump (`cog bump --auto --dry-run`; if
   `expected_increment != auto`, assert the computed bump kind matches).
6. Cut tag: `cog bump --auto --disable-bump-commit` (local tag only).
7. Resolve version from the tag; validate it is `v`-prefixed semver.
8. Set up Go + module cache.
9. Build web UI (`pnpm install --frozen-lockfile && pnpm build` in `web/`).
10. Login to GHCR; install cosign + syft.
11. `goreleaser release --clean` with `GITHUB_TOKEN` = the **app token**.
12. Trivy image scan; binary + container provenance attestation.

Removed: the entire `release` job, git-cliff install/run, CHANGELOG.md
commit/push, `gh release create --draft`, `gh release edit --draft=false`, and
the cross-job `needs` plumbing.

### `cliff.toml` (delete)

git-cliff is no longer used.

### `Taskfile.yml` (add `release:cut`)

```yaml
  release:cut:
    desc: "Cut a release via GitHub Actions (cog derives version + tag). Args: increment=auto|major|minor|patch"
    cmds:
      - gh workflow run release.yml -f expected_increment={{.increment | default "auto"}}
```

`release:check` and `release:snapshot` are unchanged.

## Recovery / Cleanup (one-time, out of band)

Independent of the pipeline rewrite:

- **Backfill v0.7.0 notes** onto the published release from the orphaned draft
  body (or regenerate). Optionally backfill v0.3.7–v0.6.0.
- **Delete the orphaned draft releases** for v0.3.7, v0.4.0, v0.5.0, v0.6.0,
  v0.7.0 to stop them accumulating.

These are manual `gh release` operations, not part of the workflow.

## Migration Strategy

1. Land all config + workflow changes in a single PR (docs + implementation).
2. Verify locally: `goreleaser check`, `cog bump --auto --dry-run`, `actionlint`.
3. After merge, cut a test release via `task release:cut` (or
   `gh workflow run release.yml`).
4. Verify: single published release, populated notes, all assets attached,
   Docker images pushed, Homebrew cask updated, signatures present.

## Risks

- **cog version drift in CI.** The pinned cog binary in CI must match the cog-7
  config semantics. Mitigation: pin the cog version in the install step (as
  holomush does) and verify with a dry-run step before the tag is cut.
- **App-token scope.** The token must carry `contents: write` (release + remote
  tag) and `packages: write` (GHCR). Mitigation: scope explicitly when minting.
- **First release after migration.** `workflow_dispatch`-only means the operator
  controls timing. Low risk; recoverable by re-running.
