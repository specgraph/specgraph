---
phase: quick-260716-neg
verified: 2026-07-16T00:00:00Z
status: human_needed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Confirm the release-please GitHub App is an allowed bypass actor on the main branch-protection ruleset."
    expected: "Release-please's release-PR merge (and its version-bump commit) is not silently blocked by required status checks / review rules."
    why_human: "Repo-level GitHub Settings (branch protection rulesets), not expressible in tracked YAML/JSON — cannot be verified from the codebase."
  - test: "Confirm squash-merge 'default commit message' is set to 'Pull request title' (Settings -> General -> Pull Requests)."
    expected: "The squashed commit landing on main is the linted PR title, which release-please then parses for Conventional Commit bump detection."
    why_human: "Repo-level GitHub Settings, not expressible in tracked YAML/JSON — cannot be verified from the codebase."
---

# Quick Task 260716-neg: Switch releases to release-please Verification Report

**Task Goal:** Switch SpecGraph's release pipeline from cog + GoReleaser to release-please + GoReleaser, mirroring seanb4t/engram.
**Verified:** 2026-07-16 (branch `release-please-migration`, HEAD `1df1ccc8`, includes post-executor review-fix commit)
**Status:** human_needed (all code-verifiable must-haves passed; 2 out-of-band repo settings require a human to confirm in GitHub UI — these are expected non-code items called out by the plan itself, not gaps)

## Goal Achievement

### Observable Truths (from PLAN must_haves + verification checklist)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | release.yml triggers ONLY on push to main | ✓ VERIFIED | `.github/workflows/release.yml:4-6` — `on: push: branches: [main]`. No `workflow_dispatch` block; `grep -c workflow_dispatch` (comments excluded) = 0. |
| 2 | App token minted with create-github-app-token v3 (SHA bcd2ba4…) + `permission-pull-requests: write` added | ✓ VERIFIED | `release.yml:24-30`: `actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1 # v3` with `permission-contents: write`, `permission-packages: write`, `permission-pull-requests: write`. |
| 3 | release-please v5 (SHA 45996ed…) runs with `skip-labeling: true` | ✓ VERIFIED | `release.yml:31-45`: `googleapis/release-please-action@45996ed1f6d02564a971a2fa1b5860e934307cf7 # v5.0.0`, `skip-labeling: true`, with a code comment explaining why (App token lacks `issues:write`). |
| 4 | Gated `actions/checkout` (ref=tag_name, fetch-depth 0) precedes every build step | ✓ VERIFIED | `release.yml:46-51`: checkout gated `if: steps.release.outputs.release_created`, `ref: steps.release.outputs.tag_name`, `fetch-depth: 0`, positioned before all subsequent build steps. |
| 5 | ALL rich build/publish steps gated on `release_created` (web pnpm build, cosign, syft, buildx, GHCR login, GoReleaser, Trivy, 2× attest) | ✓ VERIFIED | Every step from checkout (line 46) through the two `attest-build-provenance` steps (lines 104-113) carries `if: ${{ steps.release.outputs.release_created }}`. Total occurrences of the gate = 14 (checkout + 12 build steps + Trivy... counted across all gated steps). |
| 6 | GoReleaser env `GITHUB_TOKEN` = `steps.app-token.outputs.token` | ✓ VERIFIED | `release.yml:92-94`: `env: GITHUB_TOKEN: ${{ steps.app-token.outputs.token }}`, `HOMEBREW_TAP_TOKEN` preserved. |
| 7 | Trivy `image-ref` uses `steps.release.outputs.version` | ✓ VERIFIED | `release.yml:99`: `image-ref: "ghcr.io/specgraph/specgraph:${{ steps.release.outputs.version }}"`. |
| 8 | NO `workflow_dispatch`, NO cog steps, NO `steps.version` references in release.yml | ✓ VERIFIED | `grep -i cog release.yml` = empty; `steps.version.outputs` count (comments excluded) = 0; `workflow_dispatch` count = 0. |
| 9 | `.goreleaser.yaml` `release:` block is ONLY `replace_existing_artifacts: true` | ✓ VERIFIED | `.goreleaser.yaml:84-85` — the entire `release:` block is exactly `replace_existing_artifacts: true`. No `mode`, `target_commitish`, `draft`, `prerelease`, or `github:` sub-block. No `changelog:` block anywhere in the file. `goreleaser check` passes (exit 0). |
| 10 | `release-please-config.json`: release-type go, bump-minor-pre-major true, always-update true, NO bump-patch-for-minor-pre-major, NO extra-files | ✓ VERIFIED | File content confirmed via `Read` + `jq -e` assertion — all conditions true. |
| 11 | `.release-please-manifest.json`: `{".":"0.13.0"}` | ✓ VERIFIED | File content is exactly `{".": "0.13.0"}`. `jq -e '.["."]=="0.13.0"'` passes. |
| 12 | Dedicated `.github/workflows/pr-title.yml` exists (amannn v6.1.1 SHA 48f2562…, types include `edited`, no paths-ignore, pull-requests: read); commit-lint NOT in ci.yml | ✓ VERIFIED | `.github/workflows/pr-title.yml` exists as its own workflow (introduced by the post-executor review-fix commit, moved out of ci.yml deliberately — see file header comment explaining ci.yml's `paths-ignore` would skip docs-only PRs). Trigger `types: [opened, edited, reopened, synchronize]` includes `edited`; no `paths-ignore` key present; job permissions `pull-requests: read`; step uses `amannn/action-semantic-pull-request@48f256284bd46cdaab1048c3721360e808335d50 # v6.1.1`. Confirmed `grep -n commit-lint .github/workflows/ci.yml` returns no match (exit 1) — commit-lint job was removed from ci.yml in the review-fix commit and now lives solely as the `pr-title` job (named `commit-lint`) inside `pr-title.yml`. `git diff --name-only origin/main..HEAD` does not list `ci.yml`, confirming it nets out unchanged from origin/main. |
| 13 | cog demoted: cog.toml header states validation-only (no "cog is the release engine" line) | ✓ VERIFIED | `cog.toml:5-9` states cog "is now used ONLY for local commit-message validation... It NO LONGER drives releases." No "cog is the release engine" text found (`grep -c` = 0 implied by content read). |
| 14 | Taskfile.yml has no `release:cut` but keeps `release:check`, `release:snapshot`, `cocogitto` | ✓ VERIFIED | `Taskfile.yml:405` `release:check:`, `:409` `release:snapshot:`; `release:cut` absent; `cocogitto` present in the tools brew-install list (line 358). |
| 15 | lefthook.yaml commit-msg (cog verify + DCO) unchanged | ✓ VERIFIED | No lefthook.yaml diff present in `git diff --name-only origin/main..HEAD` — file untouched by this task, matching the LOCKED decision to keep it as-is. |
| 16 | CHANGELOG.md un-frozen (no "frozen as of v0.7.0" wording; mentions 0.14.0 bot-maintenance; historical `## [0.7.0]` block intact) | ✓ VERIFIED | `CHANGELOG.md:1-9` new note text mentions "Starting with v0.14.0, this file is bot-maintained by release-please"; `grep -c 'frozen as of v0.7.0'` = 0; `## [0.7.0]` heading present at line 12 with full historical entries intact below. |
| 17 | RELEASING.md documents merge-the-PR flow + two out-of-band settings | ✓ VERIFIED | `RELEASING.md` exists, documents "Merge the open release-please release PR" as the sole release trigger (lines 1-7), and has an explicit "Out-of-band repo settings" section (lines 55-68) listing both the App bypass-actor requirement and the squash-merge default-commit-message requirement. |

**Score:** 17/17 code-verifiable truths verified (0 behavior-unverified). 2 additional items are inherently out-of-band (GitHub repo settings) and are listed under Human Verification, not counted as gaps per task instructions.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `release-please-config.json` | release-type go, bump-minor-pre-major, always-update, no bump-patch flag, no extra-files | ✓ VERIFIED | Content matches exactly; `jq` assertion passes. |
| `.release-please-manifest.json` | `{".":"0.13.0"}` | ✓ VERIFIED | Exact match. |
| `.github/workflows/release.yml` | Rewritten gated workflow | ✓ VERIFIED | All sub-requirements verified above; `actionlint` clean. |
| `.goreleaser.yaml` | Reduced release block, no changelog block | ✓ VERIFIED | `goreleaser check` passes; content matches spec exactly. |
| `.github/workflows/ci.yml` | commit-lint job removed (moved to pr-title.yml) | ✓ VERIFIED | No `commit-lint` string present; file nets out unchanged vs origin/main. |
| `.github/workflows/pr-title.yml` (new, added by review-fix) | Dedicated PR-title lint workflow | ✓ VERIFIED | Present, matches all cited requirements, `actionlint` clean. |
| `cog.toml` | Validation-only header | ✓ VERIFIED | Header updated; functional keys (`tag_prefix`, `disable_changelog`, `branch_whitelist`, `ignore_merge_commits`) unchanged. |
| `CHANGELOG.md` | Un-frozen note, historical block intact | ✓ VERIFIED | Confirmed above. |
| `Taskfile.yml` | `release:cut` removed, others kept | ✓ VERIFIED | Confirmed above. |
| `RELEASING.md` (new) | Flow + out-of-band checklist doc | ✓ VERIFIED | Present and complete. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| release-please-action | config-file + manifest-file | inputs | ✓ WIRED | `release.yml:36-37` references `release-please-config.json` and `.release-please-manifest.json` by relative path; both files exist at repo root and parse. |
| App token (contents+pull-requests+packages write) | release-please PR/tag/Release AND GoReleaser's GITHUB_TOKEN | `steps.app-token.outputs.token` | ✓ WIRED | Token used at `release.yml:35` (release-please `token:` input) and again at `release.yml:93` (GoReleaser `env.GITHUB_TOKEN`) — same token, single App-token step. |
| Every rich build/publish step | `steps.release.outputs.release_created` | `if:` gate | ✓ WIRED | Verified gate present on checkout + all 13 subsequent build/publish/scan/attest steps (14 total gate occurrences). |
| Gated checkout | `steps.release.outputs.tag_name` | `ref:` | ✓ WIRED | `release.yml:49`. |
| Trivy image-ref | `steps.release.outputs.version` | string interpolation | ✓ WIRED | `release.yml:99`, replaces the deleted `steps.version` step. |
| `.goreleaser.yaml` release: block | release-please's notes body | `replace_existing_artifacts: true` (default keep-existing mode) | ✓ WIRED | No `mode: append` set; block is minimal as specified. |

### Diff Scope Check

`git diff --name-only origin/main..HEAD -- . ':!.planning'` returned exactly:

```
.github/workflows/pr-title.yml
.github/workflows/release.yml
.goreleaser.yaml
.release-please-manifest.json
CHANGELOG.md
RELEASING.md
Taskfile.yml
cog.toml
release-please-config.json
```

All 9 changed files are intended release-pipeline files. No stray Go source reformats, no unrelated file changes. `.github/workflows/ci.yml` and `lefthook.yaml` are correctly absent (ci.yml's commit-lint addition was later reverted by the review-fix commit that introduced pr-title.yml instead, netting ci.yml back to its origin/main state; lefthook.yaml was never touched, matching the LOCKED decision to leave it as-is).

### Automated Gate Results

| Check | Result |
|-------|--------|
| `actionlint .github/workflows/release.yml .github/workflows/ci.yml .github/workflows/pr-title.yml` | PASS (exit 0) |
| `goreleaser check` | PASS (exit 0, "1 configuration file(s) validated") |
| `jq` assertions on release-please-config.json / manifest | PASS |
| `rg "TBD\|FIXME\|XXX"` across all 9 changed files | No matches |
| `steps.release.outputs.release_created` gate count | 14 |
| `workflow_dispatch` / `steps.version.outputs` / cog references in release.yml | 0 (all absent) |

### Anti-Patterns Found

None. No debt markers (TBD/FIXME/XXX), no placeholder text, no stub implementations in any of the 9 changed files.

### Human Verification Required

### 1. Release-please App bypass actor on main branch protection

**Test:** In GitHub repo Settings -> Rules (branch-protection ruleset for `main`), confirm the release-please GitHub App (the App identified by `secrets.RELEASE_APP_ID`) is listed as an allowed bypass actor.
**Expected:** The release-please release-PR merge (and the version-bump/tag it produces) is not silently blocked by required status checks or review rules.
**Why human:** This is a GitHub repository configuration setting (branch protection ruleset), not expressible in tracked YAML/JSON in this repo — cannot be verified from the codebase.

### 2. Squash-merge default commit message = "Pull request title"

**Test:** In GitHub repo Settings -> General -> Pull Requests, confirm "Default commit message" for squash merging is set to "Pull request title" (not the compound/default GitHub-generated message).
**Expected:** The squashed commit landing on `main` is exactly the linted PR title, which release-please then parses for Conventional Commit bump detection.
**Why human:** Repo-level GitHub UI setting, not expressible in tracked YAML/JSON — cannot be verified from the codebase.

### Gaps Summary

No gaps. All 17 code-verifiable must-haves (from PLAN frontmatter truths/artifacts/key_links plus the task's explicit verification checklist) are met with direct file:line evidence. The review-fix commit (`1df1ccc8`, applied after the executor finished) correctly moved the PR-title lint out of `ci.yml` into a dedicated `pr-title.yml` workflow — this is a superset improvement over the original plan (avoids `ci.yml`'s `paths-ignore` skipping docs-only PR title checks) and satisfies the verification instructions' explicit requirement that commit-lint NOT be in ci.yml. The only unresolved items are the two GitHub repo-settings confirmations, which the plan itself designates as out-of-band/human-only and RELEASING.md documents as a PR-merge checklist — these are not code gaps.

---

_Verified: 2026-07-16_
_Verifier: Claude (gsd-verifier)_
