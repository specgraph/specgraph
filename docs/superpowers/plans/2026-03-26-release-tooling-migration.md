# Release Tooling Migration Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace release-please with git-cliff + goreleaser v2 for a simpler, workflow_dispatch-triggered release pipeline.

**Architecture:** git-cliff computes next semver from conventional commits, generates CHANGELOG.md, creates the git tag and GitHub release. goreleaser v2 builds binaries, Docker images, Homebrew formula, signs, and attests — appending artifacts to the release git-cliff created. Manual trigger only (workflow_dispatch).

**Tech Stack:** git-cliff (Tera templates, conventional commits), goreleaser v2 (dockers_v2, homebrew_casks), GitHub Actions

**Spec:** `docs/superpowers/specs/2026-03-26-release-tooling-migration-design.md`

---

## Chunk 1: git-cliff Configuration + Delete Release-Please Files

### Task 1: Create cliff.toml

**Files:**

- Create: `cliff.toml`

- [ ] **Step 1: Create `cliff.toml`**

```toml
# SPDX-License-Identifier: MIT
# Copyright 2026 Sean Brandt

[remote.github]
owner = "specgraph"
repo = "specgraph"

[changelog]
header = """# Changelog\n
All notable changes to this project will be documented in this file.\n"""
body = """
{%- macro remote_url() -%}
  https://github.com/{{ remote.github.owner }}/{{ remote.github.repo }}
{%- endmacro -%}

{% if version -%}
    ## [{{ version | trim_start_matches(pat="v") }}]({{ self::remote_url() }}/compare/{{ previous.version }}...{{ version }}) - {{ timestamp | date(format="%Y-%m-%d") }}
{% else -%}
    ## [Unreleased]
{% endif -%}

{% for group, commits in commits | group_by(attribute="group") %}
    ### {{ group | striptags | trim | upper_first }}
    {% for commit in commits %}
        - {% if commit.scope %}**{{ commit.scope }}:** {% endif %}\
            {% if commit.breaking %}[**breaking**] {% endif %}\
            {{ commit.message | split(pat="\n") | first | upper_first | trim }}\
            {% if commit.remote.pr_number %} ([#{{ commit.remote.pr_number }}]({{ self::remote_url() }}/pull/{{ commit.remote.pr_number }})){% endif %}\
            {%- if commit.id %} ([{{ commit.id | truncate(length=7, end="") }}]({{ self::remote_url() }}/commit/{{ commit.id }})){% endif %}
    {%- endfor %}
{% endfor %}\n
"""
footer = ""
trim = true

[git]
conventional_commits = true
filter_unconventional = true
split_commits = false
commit_parsers = [
    { message = "^feat", group = "Features" },
    { message = "^fix", group = "Bug Fixes" },
    { message = "^perf", group = "Performance" },
    { message = "^refactor", group = "Code Refactoring" },
    { message = "^test", group = "Tests" },
    { message = "^ci", group = "CI" },
    { message = "^build", group = "Build" },
    { message = "^chore\\(release\\)", skip = true },
    { message = "^chore\\(main\\): release", skip = true },
    { message = "^chore", skip = true },
    { message = "^docs", skip = true },
    { message = "^Merge", skip = true },
    { message = ".*", group = "Miscellaneous" },
]
protect_breaking_commits = true
filter_commits = false
tag_pattern = "v[0-9].*"
sort_commits = "newest"

[bump]
# Pre-1.0: feat/fix both bump minor. Post-1.0: feat=minor, fix=patch.
# Breaking changes bump minor pre-1.0, major post-1.0.
# This is the default semver behavior in git-cliff.
```

- [ ] **Step 2: Verify git-cliff can parse the repo history**

Run locally (requires git-cliff installed):

```bash
git cliff --config cliff.toml --dry-run
```

Expected: Changelog output grouped by version with Features, Bug Fixes, etc. No `chore` or `docs` commits shown.

If git-cliff is not installed locally, skip this step — CI will validate.

- [ ] **Step 3: Commit**

```bash
jj --no-pager describe -m "build: add cliff.toml for git-cliff changelog generation"
```

### Task 2: Delete release-please configuration files

**Files:**

- Delete: `release-please-config.json`
- Delete: `.release-please-manifest.json`

- [ ] **Step 1: Delete the files**

```bash
rm release-please-config.json .release-please-manifest.json
```

- [ ] **Step 2: Verify they're gone**

```bash
jj --no-pager status
```

Expected: Both files show as deleted.

- [ ] **Step 3: Commit**

```bash
jj --no-pager describe -m "build: remove release-please config and manifest files"
```

Note: Squash tasks 1 and 2 together before PR if desired — they're logically one change.

---

## Chunk 2: goreleaser v2 API Migration

### Task 3: Migrate `.goreleaser.yaml` to v2 APIs

**Files:**

- Modify: `.goreleaser.yaml`

The current config uses deprecated v1 keys. Here is the full replacement:

- [ ] **Step 1: Replace `.goreleaser.yaml` with v2 config**

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
archives:
  - id: default
    formats:
      - tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - README.md
      - LICENSE
checksum:
  name_template: checksums.txt
  algorithm: sha256
homebrew_casks:
  - name: specgraph
    repository:
      owner: specgraph
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    homepage: https://specgraph.io
    description: Live spec-driven development framework
    license: MIT
dockers_v2:
  - images:
      - "ghcr.io/specgraph/specgraph"
    tags:
      - "{{ .Version }}"
      - latest
    dockerfile: Dockerfile
    labels:
      "org.opencontainers.image.title": "{{ .ProjectName }}"
      "org.opencontainers.image.version": "{{ .Version }}"
      "org.opencontainers.image.source": "https://github.com/specgraph/specgraph"
source:
  enabled: true
  name_template: "{{ .ProjectName }}.src"
sboms:
  - artifacts: archive
  - id: source
    artifacts: source
signs:
  - cmd: cosign
    signature: "${artifact}.sigstore.json"
    args:
      - sign-blob
      - "--bundle=${signature}"
      - "${artifact}"
      - "--yes"
    artifacts: checksum
    output: true
docker_signs:
  - cmd: cosign
    output: true
    artifacts: all
    args:
      - "sign"
      - "${artifact}@${digest}"
      - "--yes"
release:
  mode: append
changelog:
  disable: true
```

Key changes from current config:

- `archives[].format` → `archives[].formats` (list)
- `brews` → `homebrew_casks` (removed `install` block — casks use binary directly)
- `dockers` + `docker_manifests` → `dockers_v2` (single entry, multi-arch handled natively)
- `release.use_existing_draft: true` + `release.mode: keep-existing` → `release.mode: append`
- `changelog.use: github` → `changelog.disable: true`

- [ ] **Step 2: Validate config syntax**

```bash
goreleaser check --config .goreleaser.yaml
```

Expected: No errors. If goreleaser is not installed locally, skip — CI will validate.

- [ ] **Step 3: Commit**

```bash
jj --no-pager describe -m "build: migrate .goreleaser.yaml to v2 APIs (dockers_v2, homebrew_casks)"
```

---

## Chunk 3: Site Docs Version Placeholders

### Task 4: Replace hardcoded versions in site docs

**Files:**

- Modify: `site/docs/quickstart.md` (lines 21-40)
- Modify: `site/docs/index.md` (line 90)

- [ ] **Step 1: Update `site/docs/quickstart.md`**

Remove the `x-release-please-start-version` and `x-release-please-end` markers (lines 21 and 40). Replace the hardcoded version in the GitHub releases link with a `/releases/latest` URL. Replace the pinned Docker tag (`0.2.1`) with `latest`. This eliminates the need for sed in CI and avoids stale version references.

- [ ] **Step 2: Update `site/docs/index.md`**

Replace line 90 (`v0.2.1 <!-- x-release-please-version -->`) with:

```markdown
See the [changelog](changelog.md) for the latest release.
```

- [ ] **Step 3: Verify no remaining release-please markers**

```bash
grep -r "x-release-please" site/docs/
```

Expected: No matches.

- [ ] **Step 4: Commit**

```bash
jj --no-pager describe -m "docs: remove hardcoded versions and release-please markers from site docs"
```

---

## Chunk 4: Rewrite Release Workflow

### Task 5: Rewrite `.github/workflows/release.yml`

**Files:**

- Modify: `.github/workflows/release.yml`

This is the core change. The new workflow has two jobs: `release` (git-cliff) and `goreleaser`.

- [ ] **Step 1: Replace `.github/workflows/release.yml`**

```yaml
# SPDX-License-Identifier: MIT
# Copyright 2026 Sean Brandt
name: Release
on:
  workflow_dispatch: {}
permissions: {}
concurrency:
  group: release
  cancel-in-progress: false
jobs:
  release:
    permissions:
      contents: write
    runs-on: namespace-profile-linux-amd64-4x8
    outputs:
      new_tag: ${{ steps.cliff.outputs.tag }}
      version: ${{ steps.cliff.outputs.version }}
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6
        with:
          fetch-depth: 0
      - name: Install git-cliff
        uses: orhun/git-cliff-action@4a4a951bc43fcfc7e9867e009e6e42d1672e522e # v4
        id: cliff
        with:
          args: --bump
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          OUTPUT: CHANGELOG.md
      - name: Check if version bumped
        id: check
        run: |
          TAG="${{ steps.cliff.outputs.tag }}"
          if [ -z "$TAG" ]; then
            echo "No releasable commits since last tag. Exiting."
            echo "new_tag=" >> "$GITHUB_OUTPUT"
            exit 0
          fi
          VERSION="${TAG#v}"
          echo "new_tag=$TAG" >> "$GITHUB_OUTPUT"
          echo "version=$VERSION" >> "$GITHUB_OUTPUT"
      - name: Commit changelog
        if: steps.check.outputs.new_tag != ''
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add CHANGELOG.md
          git commit -m "chore(release): ${{ steps.check.outputs.new_tag }}"
          git push
      - name: Create and push tag
        if: steps.check.outputs.new_tag != ''
        run: |
          git tag "$TAG"
          git push origin "$TAG"
        env:
          TAG: ${{ steps.check.outputs.new_tag }}
      - name: Create GitHub release
        if: steps.check.outputs.new_tag != ''
        run: |
          gh release create "$TAG" \
            --title "$TAG" \
            --notes-file <(git cliff --latest --strip header) \
            --verify-tag
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          TAG: ${{ steps.check.outputs.new_tag }}
  goreleaser:
    permissions:
      contents: write
      packages: write
      id-token: write
      attestations: write
    needs: release
    if: needs.release.outputs.new_tag != ''
    runs-on: namespace-profile-linux-amd64-4x8
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6
        with:
          fetch-depth: 0
          ref: ${{ needs.release.outputs.new_tag }}
      - uses: sigstore/cosign-installer@ba7bc0a3fef59531c69a25acd34668d6d3fe6f22 # v4.1.0
      - uses: anchore/sbom-action/download-syft@e22c389904149dbc22b58101806040fa8d37a610 # v0
      - uses: namespacelabs/nscloud-setup-buildx-action@d059ed7184f0bc7c8b27e8810cea153d02bcc6dd # v0.0.23
      - uses: docker/login-action@b45d80f862d83dbcd57f89517bcf500b2ab88fb2 # v4
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: actions/setup-node@49933ea5288caeca8642d1e84afbd3f7d6820020 # v4
        with:
          node-version: "22"
      - uses: pnpm/action-setup@02f6c237bd2518259fed6c71566509edfb3f2b74 # v4
        with:
          run_install: false
      - name: Build web UI
        working-directory: web
        run: pnpm install --frozen-lockfile && pnpm build
      - uses: actions/setup-go@4b73464bb391d4059bd26b0524d20df3927bd417 # v6
        with:
          go-version-file: go.mod
          cache: false
      - name: Configure Namespace cache
        uses: namespacelabs/nscloud-cache-action@a90bb5d4b27522ce881c6e98eebd7d7e6d1653f9 # v1.4.2
        with:
          cache: go
      - uses: goreleaser/goreleaser-action@ec59f474b9834571250b370d4735c50f8e2d1e29 # v7
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
      - name: Scan Docker image
        uses: aquasecurity/trivy-action@57a97c7e7821a5776cebc9bb87c984fa69cba8f1 # v0.35.0
        with:
          image-ref: "ghcr.io/specgraph/specgraph:${{ needs.release.outputs.version }}"
          format: table
          exit-code: "1"
          severity: CRITICAL,HIGH
      - name: Attest binary provenance
        uses: actions/attest-build-provenance@a2bbfa25375fe432b6a289bc6b6cd05ecd0c4c32 # v4
        with:
          subject-checksums: ./dist/checksums.txt
      - name: Attest Docker image provenance
        uses: actions/attest-build-provenance@a2bbfa25375fe432b6a289bc6b6cd05ecd0c4c32 # v4
        with:
          subject-checksums: ./dist/digests.txt
```

Key differences from current workflow:

- Trigger: `workflow_dispatch` only (was `push: branches: [main]`)
- Job 1 (`release`): git-cliff replaces release-please. Computes version, generates CHANGELOG.md, commits, tags, creates GitHub release.
- Job 2 (`goreleaser`): Same artifact pipeline, but checks out at tag SHA and uses `release.mode: append`.
- No `RELEASE_PLEASE_TOKEN` needed.
- Trivy scan simplified: `dockers_v2` produces a single multi-arch manifest, so one scan suffices.
- git-cliff-action pin: `v4` at `4a4a951bc43fcfc7e9867e009e6e42d1672e522e` — verify this is current before merging.

- [ ] **Step 2: Verify the git-cliff-action pin**

```bash
gh api repos/orhun/git-cliff-action/git/ref/tags/v4 --jq '.object.sha'
```

If the SHA differs from `4a4a951bc43fcfc7e9867e009e6e42d1672e522e`, update the pin in the workflow file. If the tag is annotated (returns a tag object), dereference it:

```bash
gh api repos/orhun/git-cliff-action/git/tags/<sha> --jq '.object.sha'
```

- [ ] **Step 3: Commit**

```bash
jj --no-pager describe -m "ci: rewrite release workflow with git-cliff + goreleaser v2"
```

---

## Chunk 5: Final Verification + PR

### Task 6: Squash, verify, and create PR

- [ ] **Step 1: Squash all changes into one commit**

```bash
jj --no-pager squash --into <first-change-id> -m "build: migrate release pipeline from release-please to git-cliff + goreleaser v2"
```

Or if working on separate jj changes, squash them together:

```bash
jj --no-pager new <first> <last>
jj --no-pager describe -m "build: migrate release pipeline from release-please to git-cliff + goreleaser v2"
```

- [ ] **Step 2: Verify no release-please references remain**

```bash
grep -r "release-please" --include="*.yml" --include="*.yaml" --include="*.json" --include="*.md" . | grep -v CHANGELOG.md | grep -v docs/superpowers/
```

Expected: No matches (CHANGELOG.md and spec docs may reference it historically — that's fine).

- [ ] **Step 3: Verify goreleaser config**

```bash
# If goreleaser installed locally:
goreleaser check --config .goreleaser.yaml
```

- [ ] **Step 4: Verify git-cliff config**

```bash
# If git-cliff installed locally:
git cliff --config cliff.toml --dry-run | head -50
```

- [ ] **Step 5: Run task check**

```bash
task check
```

Expected: All checks pass (fmt, lint, build, test).

- [ ] **Step 6: Create bookmark and push**

```bash
jj --no-pager bookmark set build/release-migration -r @
jj --no-pager git push --bookmark build/release-migration
```

- [ ] **Step 7: Create PR**

```bash
gh pr create \
  --title "build: migrate release pipeline from release-please to git-cliff + goreleaser v2" \
  --body "$(cat <<'EOF'
## Summary
- Replace release-please with git-cliff for version computation and changelog generation
- Migrate goreleaser to v2 APIs (dockers_v2, homebrew_casks, formats)
- Switch release trigger from push-to-main to workflow_dispatch (manual)
- Remove release-please config files and manifest
- Remove hardcoded version strings from site docs

Closes #561

## Test plan
- [ ] Merge to main
- [ ] Trigger workflow_dispatch on the Release workflow
- [ ] Verify: CHANGELOG.md updated, tag created, GitHub release with changelog body
- [ ] Verify: goreleaser artifacts attached (binaries, Docker, Homebrew)
- [ ] Clean up RELEASE_PLEASE_TOKEN secret after confirming

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Post-Merge Verification (Manual)

After the PR merges:

1. Go to Actions → Release → Run workflow (workflow_dispatch)
2. Watch Job 1 (release): should compute next version, generate CHANGELOG.md, commit, tag, create GitHub release
3. Watch Job 2 (goreleaser): should build binaries, Docker images, sign, attest, push Homebrew formula
4. Verify the GitHub release has both changelog body and attached artifacts
5. Verify `brew install specgraph/tap/specgraph` works
6. Verify `docker pull ghcr.io/specgraph/specgraph:latest` works
7. Delete the `RELEASE_PLEASE_TOKEN` secret from repo settings
