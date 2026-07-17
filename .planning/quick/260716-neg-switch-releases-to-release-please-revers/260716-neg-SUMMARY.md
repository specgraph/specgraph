---
phase: quick-260716-neg
plan: 01
subsystem: release-engineering
status: complete
tags: [release-please, goreleaser, ci, github-actions]
dependency-graph:
  requires: []
  provides:
    - release-please-config.json
    - .release-please-manifest.json
    - gated-release-workflow
    - pr-title-lint
  affects:
    - .github/workflows/release.yml
    - .github/workflows/ci.yml
    - .goreleaser.yaml
    - cog.toml
    - Taskfile.yml
    - CHANGELOG.md
tech-stack:
  added:
    - googleapis/release-please-action v5.0.0 (45996ed1f6d02564a971a2fa1b5860e934307cf7)
    - amannn/action-semantic-pull-request v6.1.1 (48f256284bd46cdaab1048c3721360e808335d50)
  patterns:
    - "release-please owns version bump/tag/CHANGELOG/Release; GoReleaser gated on release_created uploads artifacts only"
key-files:
  created:
    - release-please-config.json
    - .release-please-manifest.json
    - RELEASING.md
  modified:
    - .github/workflows/release.yml
    - .github/workflows/ci.yml
    - .goreleaser.yaml
    - cog.toml
    - Taskfile.yml
    - CHANGELOG.md
decisions:
  - "Omit bump-patch-for-minor-pre-major to preserve feat:->minor cadence (deliberate divergence from engram reference)"
  - "GoReleaser release: block collapses to replace_existing_artifacts: true only; no mode: append (avoids duplicate changelog)"
  - "App token (create-github-app-token v3) gains pull-requests: write; used as GITHUB_TOKEN for GoReleaser too"
metrics:
  duration: "~35m"
  completed: 2026-07-16
---

# Quick Task 260716-neg: Switch releases to release-please Summary

Switched SpecGraph's release pipeline from cog + GoReleaser (single always-running job) to release-please + GoReleaser (split ownership) — release-please owns version bump/tag/CHANGELOG/Release creation, GoReleaser (gated on `release_created`) uploads artifacts to the pre-existing release.

## What Was Built

1. **`release-please-config.json` + `.release-please-manifest.json`** — single Go-root package config (`bump-minor-pre-major`, `always-update`, no `bump-patch-for-minor-pre-major`/`extra-files`), manifest seeded to `0.13.0`.
2. **`.github/workflows/release.yml`** rewritten — triggers on `push: main` only (no `workflow_dispatch`); mints a GitHub App token (v3, +`pull-requests: write`) → runs `release-please-action` v5 → gated `actions/checkout` at `steps.release.outputs.tag_name` → every existing build/publish step (web build, GoReleaser, Trivy, SBOM, cosign, 2x attestations) gated on `steps.release.outputs.release_created`. All cog steps and the "Get version" step removed; Trivy now uses `steps.release.outputs.version`.
3. **`.goreleaser.yaml`** — `release:` block collapsed to `replace_existing_artifacts: true`; removed `target_commitish`, `draft`, `prerelease`, `github: {owner,name}`, and the entire `changelog:` block.
4. **`.github/workflows/ci.yml`** — added `commit-lint` job (`amannn/action-semantic-pull-request` v6.1.1, `pull_request`-only, `pull-requests: read`) that lints PR titles as Conventional Commits.
5. **`cog.toml`** — header rewritten: cog is now commit-message-validation-only (no release role). Functional keys unchanged.
6. **`Taskfile.yml`** — removed `release:cut`; kept `release:check`, `release:snapshot`, `cocogitto` in tools.
7. **`CHANGELOG.md`** — un-froze the top note: documents the expected v0.7.0→v0.14.0 gap (v0.8.0–v0.13.0 exist only on GitHub Releases) and that v0.14.0+ is bot-maintained by release-please. Historical block (≤v0.7.0) untouched.
8. **`RELEASING.md`** (new) — documents the merge-the-release-PR flow, version cadence, commit-lint chain, and the two out-of-band repo settings (App bypass actor on `main` ruleset; squash "default commit message" = "Pull request title").

## Task Outcomes

| Task | Outcome | Commit |
|------|---------|--------|
| 1 — release-please config + manifest | Pass | `2f2a1fe1` |
| 2 — rewrite release.yml (gated release-please + GoReleaser) | Pass | `6489d4e8` |
| 3 — reduce GoReleaser release block | Pass | `1749097a` |
| 4 — PR-title lint job in ci.yml | Pass | `bfaa6aa1` |
| 5 — demote cog + remove release:cut | Pass | `2f9779fc` |
| 6 — un-freeze CHANGELOG + add RELEASING.md | Pass | `e0196595` |
| 7 — full validation gate | Pass (no fixup commit needed) | n/a |

All 7 tasks' `<verify><automated>` commands passed on first or second attempt (Task 6 required one fix — see Deviations).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] RELEASING.md line-wrap broke the Task 6 verify grep**
- **Found during:** Task 6 verification
- **Issue:** The phrase `"Pull request title"` was written across a Markdown line wrap (`"Pull request\n   title"`), so `grep -qi 'Pull request title'` failed (grep matches single lines only).
- **Fix:** Reflowed the sentence so the full phrase sits on one line.
- **Files modified:** `RELEASING.md`
- **Commit:** `e0196595` (folded into the Task 6 commit before it was made — the file was corrected prior to staging/committing).

No other deviations. Plan executed as written otherwise.

## Task 7 — Full Validation Gate Results

```
task check   → PASS (fmt:check, license:check, lint incl. actionlint on both workflows,
                skills:validate, build, verify:version-stamp, go test -short -race ./...)
goreleaser check → PASS (1 configuration file validated)
```

`git status --short` after `task check` showed only the pre-existing untracked `.planning/quick/` directory — no formatting/license fixups were needed on any edited YAML/JSON/Markdown file, so Task 7 made no additional code commit.

## Out-of-Band Follow-Up (human, before first release PR merges)

Per `RELEASING.md` and the plan's Task 7 note, these two GitHub repo settings must be confirmed before the first release-please PR is merged — they cannot be expressed in this repo's code:

1. The release-please GitHub App must be an allowed **bypass actor** on the `main` branch-protection ruleset.
2. Squash-merge **"default commit message"** must be set to **"Pull request title"** (Settings → General → Pull Requests).

## Self-Check: PASSED

All 9 `files_modified` artifacts confirmed present on disk; all 6 task commit hashes confirmed present in `git log`.
