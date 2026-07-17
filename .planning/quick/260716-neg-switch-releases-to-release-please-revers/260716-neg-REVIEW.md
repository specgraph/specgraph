---
phase: 260716-neg-switch-releases-to-release-please-revers
reviewed: 2026-07-16T21:54:23Z
depth: quick
files_reviewed: 9
files_reviewed_list:
  - .github/workflows/release.yml
  - .github/workflows/ci.yml
  - .goreleaser.yaml
  - release-please-config.json
  - .release-please-manifest.json
  - cog.toml
  - Taskfile.yml
  - CHANGELOG.md
  - RELEASING.md
findings:
  critical: 1
  warning: 2
  info: 1
  total: 4
status: issues_found
---

# Quick Task 260716-neg: Code Review Report

**Reviewed:** 2026-07-16T21:54:23Z
**Depth:** quick (rigorous pass on workflow logic; verified against upstream action sources — `googleapis/release-please-action`, `googleapis/release-please`, `googleapis/code-suggester`, `amannn/action-semantic-pull-request`, `actions/create-github-app-token`)
**Files Reviewed:** 9
**Status:** issues_found

## Summary

Gating logic, dangling references, SHA pins, and the `.goreleaser.yaml` release block all check out cleanly against the LOCKED decisions — I verified each of the three key action SHAs against the actual upstream tags (`45996ed1…` = `release-please-action@v5.0.0`, `48f256284…` = `amannn/action-semantic-pull-request@v6.1.1`, `bcd2ba49…` = `actions/create-github-app-token@v3`, all exact), confirmed `steps.release.outputs.version`/`tag_name` are real root-package outputs of the manifest-mode action (not dangling `steps.version.*`), confirmed every build/publish step in `release.yml` is gated on `steps.release.outputs.release_created`, confirmed the CHANGELOG.md insertion point lands correctly relative to the frozen historical block, and confirmed `release-please-config.json`'s `$schema` URL resolves (HTTP 200).

One finding is release-breaking: the App-token step's explicit `permission-*` allow-list omits `permission-issues: write`, but release-please's default behavior is to label the release PR (`autorelease: pending`), and that label call is not wrapped in a try/catch anywhere in `release-please`/`code-suggester` — it will 403 and crash the whole `release-please` step on the very first push to `main` that has release-worthy commits, well before any release is ever cut.

## Critical Issues

### CR-01: Missing `permission-issues: write` will crash release-please on its first PR-label attempt

**File:** `.github/workflows/release.yml:22-30`
**Issue:**
The `create-github-app-token` step explicitly restricts the App token to three permissions:
```yaml
permission-contents: write
permission-packages: write
permission-pull-requests: write
```
Per `actions/create-github-app-token`'s own docs: *"By default, the token inherits all of the installation's permissions"* — but the moment **any** `permission-*` input is set, the token is scoped down to **only** the permissions explicitly listed, regardless of what the App installation actually has. `issues` is not in this list.

`release-please-action`'s own README lists the required workflow permissions as `contents: write`, `issues: write`, `pull-requests: write` (not just contents+pull-requests). The reason: release-please labels every release PR it creates with `autorelease: pending` by default (`DEFAULT_LABELS = ['autorelease: pending']` in `manifest.ts`), and `skip-labeling` is not set to `true` in `release-please-config.json`, so labeling is not skipped.

I traced the call path in `release-please`/`code-suggester` source:
- `manifest.ts` → `createOrUpdatePullRequest` → `github.createPullRequest(...)` (passes `labels: pullRequest.labels`)
- `code-suggester`'s `src/index.ts` → `openPullRequest()` (creates the PR) then unconditionally `await addLabels(octokit, ..., options.labels)`
- `code-suggester`'s `src/github/labels.ts::addLabels` calls `octokit.issues.addLabels(...)` with **no try/catch**

A token missing `issues` write permission will get a 403 from `octokit.issues.addLabels`, which propagates uncaught all the way up through `manifest.ts` (also no try/catch around this call chain) and fails the `googleapis/release-please-action` step entirely. This isn't gated behind `release_created` — it happens on **every** push to `main` where release-please tries to open/update the release PR, i.e., before the first release can ever be cut.

Note this repo's config is *not* equivalent to the `seanb4t/engram` reference: engram's `create-github-app-token` step passes **no** `permission-*` inputs at all, so its token inherits the App installation's full permission set (whatever that includes). SpecGraph's step explicitly narrows permissions, which is why this gap is exploitable here even though the pattern was copied from a working reference.

**Fix:** Either grant `issues: write` explicitly (requires the RELEASE_APP GitHub App installation to actually have `Issues: write` granted, or minting will itself fail per `create-github-app-token`'s own warning), or — simpler and doesn't depend on out-of-band App permission changes — tell release-please-action to skip labeling entirely:
```yaml
      - name: Run release-please
        id: release
        uses: googleapis/release-please-action@45996ed1f6d02564a971a2fa1b5860e934307cf7 # v5.0.0
        with:
          token: ${{ steps.app-token.outputs.token }}
          config-file: release-please-config.json
          manifest-file: .release-please-manifest.json
          skip-labeling: true
```

## Warnings

### WR-01: `pull_request` trigger doesn't include `edited`, so retitling a PR after opening skips commit-lint

**File:** `.github/workflows/ci.yml:11, 235-244`
**Issue:** `on: pull_request:` (top of `ci.yml`, applies to all jobs including the new `commit-lint` job) doesn't specify `types:`, so it defaults to `[opened, synchronize, reopened]`. `edited` is not included. If a contributor opens a PR with a valid conventional-commit title, gets a green `commit-lint` check, then edits *only the title* (no new commits, so `synchronize` doesn't fire), the check never re-runs against the new title. RELEASING.md calls PR-title linting "the authoritative gate" for what release-please parses on squash-merge — this gap means a title can be silently changed to something invalid (or a different bump class) after the gate has already passed.
**Fix:** Add an `edited` type either globally or scoped to the job trigger, e.g.:
```yaml
on:
  pull_request:
    types: [opened, synchronize, reopened, edited]
```

### WR-02: `ci.yml`'s `paths-ignore` can let a doc-only PR skip `commit-lint` entirely

**File:** `.github/workflows/ci.yml:11-15`
**Issue:** `paths-ignore: [docs/**, **.md, !CLAUDE.md]` applies to the whole workflow, including the new `commit-lint` job. A PR that only touches Markdown (e.g. an update to `RELEASING.md` itself) never triggers `ci.yml`, so `commit-lint` never runs and its title is never validated before squash-merge. This doesn't crash anything (release-please just won't bump on an unrecognized prefix), but it silently defeats the "authoritative gate" claim in `RELEASING.md` for a category of PRs this very migration introduces (`RELEASING.md` is itself a new doc file).
**Fix:** If title-linting should be unconditional, put `commit-lint` in a separate workflow with no `paths-ignore`, or accept and document the exception explicitly in `RELEASING.md`.

## Info

### IN-01: `concurrency.group: release-${{ github.ref }}` is a no-op template — always evaluates to the same value

**File:** `.github/workflows/release.yml:9-11`
**Issue:** Since the workflow only triggers on `push: branches: [main]`, `github.ref` is always `refs/heads/main` for every run of this workflow — `release-${{ github.ref }}` is equivalent to the hardcoded string `release` used previously. Not incorrect, just misleading (implies per-ref serialization that can never actually branch).
**Fix:** Either revert to the literal `group: release` for clarity, or leave as-is with a one-line comment noting it's intentionally constant (harmless either way, given `cancel-in-progress: false` correctly serializes overlapping runs).

---

_Reviewed: 2026-07-16T21:54:23Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: quick_
