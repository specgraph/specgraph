---
phase: 01-release-build-tooling
verified: 2026-07-09T00:00:00Z
status: human_needed
score: 7/7 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Confirm ROADMAP.md Phase 1 detail section (lines 34-48) is updated to reflect completion"
    expected: "Status line should read 'Complete' (not 'In progress — ... only CFG-02 remains open') and Success Criterion 3 should be marked Met (not Open, and not still citing the pre-fix '2.12.2 drift' fact as current)."
    why_human: "This is a documentation/roadmap-bookkeeping decision (what text to write, when to update it) rather than a code-verifiable fact. The underlying code fully satisfies SC3 (verified below) — only the roadmap narrative text is stale."
---

# Phase 1: Release & Build Tooling Verification Report

**Phase Goal:** Maintainers can cut a tagged release and trust the build/lint tooling without manual intervention or double-published/broken artifacts
**Verified:** 2026-07-09
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | (REL-01, roadmap SC1) A pushed release tag produces exactly one coherent GitHub Release via a single goreleaser-owned job | ✓ VERIFIED | `.github/workflows/release.yml` defines exactly one job (`goreleaser:`, line 23) using `goreleaser/goreleaser-action`; tag `v0.12.0` exists in repo history. Matches 01-CONTEXT.md D-00 claim (PR #981). |
| 2 | (CFG-01, roadmap SC2) All server/CLI config sourced through one layered koanf loader, with `SPECGRAPH_PG_URL` emitting a deprecation warning | ✓ VERIFIED | `internal/config/global.go` imports and wires `koanf/v2` + `confmap`/`env`/`file`/`structs` providers. `cmd/specgraph/serve.go:284-285` checks `os.Getenv("SPECGRAPH_PG_URL")` and logs a `slog` warning naming the replacement var. |
| 3 | (CFG-02) `task tools` installs golangci-lint at the pinned version (v2.12.1) via `go install`, not unpinned `brew install` | ✓ VERIFIED | `Taskfile.yml:358` brew line no longer lists `golangci-lint`; `Taskfile.yml:363` adds `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@{{.GOLANGCI_LINT_VERSION}}`. |
| 4 | `task tools:golangci-lint-version` prints exactly the bare pinned version string, no Task command-echo banner | ✓ VERIFIED | Ran `task tools:golangci-lint-version \| od -c` — output is exactly `v2.12.1\n`, no ANSI/banner bytes. `Taskfile.yml:365-369` sets `silent: true`, no `deps:`. |
| 5 | The golangci-lint version has exactly one literal declaration in the repo (Taskfile.yml); ci.yml no longer declares its own | ✓ VERIFIED | `Taskfile.yml:20` declares `GOLANGCI_LINT_VERSION: v2.12.1` once. `rg -e '^\s*GOLANGCI_LINT_VERSION:\s*v[0-9]' .github/workflows/ci.yml` — no match (exit 1); the `env:` block (lines 24-36) has no such entry. |
| 6 | CI's build-and-test job resolves the golangci-lint version from Taskfile.yml via command substitution | ✓ VERIFIED | `.github/workflows/ci.yml:77-82` — `GOLANGCI_LINT_VERSION="$(task tools:golangci-lint-version)"`, validated against `^v[0-9]+\.[0-9]+\.[0-9]+$` before use (WR-01 fix, commit `0f1beba7`), then passed to `go install ...@$GOLANGCI_LINT_VERSION`. |
| 7 | Requirements REL-01, CFG-01, CFG-02 are fully accounted for in REQUIREMENTS.md traceability, with REL-01/CFG-01 explicitly documented as already-shipped (not silently missing) | ✓ VERIFIED | REQUIREMENTS.md lines 110/116/117 mark all three `Phase 1 / Done`/`Complete`. 01-CONTEXT.md D-00 and 01-01-PLAN.md's "Already Shipped (traceability only — NO tasks)" section both explicitly document why REL-01/CFG-01 carry no tasks. No orphaned requirements for Phase 1. |

**Score:** 7/7 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `Taskfile.yml` | New `GOLANGCI_LINT_VERSION` var, new `tools:golangci-lint-version` leaf task, edited `tools:` task | ✓ VERIFIED | Var at line 20; leaf task at lines 365-369 (`silent: true`, no `deps:`); `tools:` task edited at lines 358/363. |
| `.github/workflows/ci.yml` | Env var removed, Install Go tools step resolves version via task | ✓ VERIFIED | Env block (lines 24-36) has no `GOLANGCI_LINT_VERSION`; Install Go tools step (lines 75-84) resolves via `$(task tools:golangci-lint-version)` with format validation (WR-01 fix). |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `ci.yml` build-and-test "Install Go tools" step | `Taskfile.yml` `tools:golangci-lint-version` leaf task | `$(task tools:golangci-lint-version)` command substitution | ✓ WIRED | "Install Task" step (lines 62-66) runs before "Install Go tools" (lines 75-84) in the same job, putting `task` on `PATH` first. Leaf task has no `deps:`, so the call is cheap. Substitution result is format-validated before use (regex guard added in WR-01 fix). |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Leaf task prints bare version, no banner | `task tools:golangci-lint-version \| od -c` | `v2.12.1\n` (8 bytes, no escape/banner) | ✓ PASS |
| Version string matches Taskfile var | `test "$(task tools:golangci-lint-version)" = "v2.12.1"` | match | ✓ PASS |
| `actionlint` passes on edited ci.yml | `actionlint .github/workflows/ci.yml` | exit 0, no output | ✓ PASS |
| `yamlfmt -lint` passes on both edited files | `yamlfmt -lint .github/workflows/ci.yml Taskfile.yml` | exit 0, no output | ✓ PASS |
| brew install line no longer references golangci-lint | `rg 'brew install.*golangci-lint' Taskfile.yml` | no match (exit 1) | ✓ PASS |
| ci.yml env block has no golangci-lint version literal | `rg '^\s*GOLANGCI_LINT_VERSION:\s*v[0-9]' .github/workflows/ci.yml` | no match (exit 1) | ✓ PASS |
| Task commits exist as claimed in SUMMARY.md | `git log --oneline -1 <hash>` for `82073eaf`, `322ee457`, `0f1beba7` | all three found with matching subjects | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|--------------|--------|----------|
| REL-01 | 01-01-PLAN.md (traceability) | Single-job goreleaser release model | ✓ SATISFIED | `.github/workflows/release.yml` single `goreleaser:` job; `v0.12.0` tag present. Traceability-only per plan design; independently re-verified above (Truth 1). |
| CFG-01 | 01-01-PLAN.md (traceability) | koanf layered config loader | ✓ SATISFIED | `internal/config/global.go` + `cmd/specgraph/serve.go:284-285`. Traceability-only per plan design; independently re-verified above (Truth 2). |
| CFG-02 | 01-01-PLAN.md (implementation) | Pin `task tools`' golangci-lint to CI's version | ✓ SATISFIED | Taskfile.yml + ci.yml edits, verified above (Truths 3-6). |

No orphaned requirements — REQUIREMENTS.md's Phase 1 mapping (REL-01, CFG-01, CFG-02) matches exactly what 01-01-PLAN.md's frontmatter declares.

### Anti-Patterns Found

None. `grep -n -E "TBD|FIXME|XXX|TODO|HACK|PLACEHOLDER"` on both modified files (`Taskfile.yml`, `.github/workflows/ci.yml`) returned no matches. No stub returns, no empty handlers — these are build/CI config files, not application code, so the standard stub-detection patterns don't apply, but a manual read of both diffs confirms every line does real work.

### Code Review Follow-up

`01-REVIEW.md` found one warning (WR-01: no failure handling if the command substitution fails) and two info-level notes (IN-01: pattern not generalized to `PROTOC_GEN_*` vars; IN-02: other brew-installed tools remain unpinned — explicitly out of scope per D-05). `01-REVIEW-FIX.md` confirms WR-01 was fixed in commit `0f1beba7`, independently verified above (Truth 6, the regex validation guard is present in the current `ci.yml`). IN-01/IN-02 are correctly left open as documented follow-up, not silently dropped.

## Gaps Summary

No code-level gaps. All must-haves for CFG-02 are implemented, wired, and behaviorally verified; the review-flagged warning was fixed and the fix is present in the current file. REL-01 and CFG-01 were confirmed shipped independently of the CONTEXT.md claim (not just trusted at face value).

One documentation-staleness item routed to human verification: `.planning/ROADMAP.md`'s Phase 1 "Phase Details" narrative (Status line + Success Criterion 3 text, lines 39/44) still reads as if CFG-02 were open and describes the pre-fix `2.12.2` drift as current fact, even though the top-of-file checklist (line 27) already marks Phase 1 `[x]` complete and REQUIREMENTS.md marks CFG-02 `Complete`. This is inconsistent within ROADMAP.md itself and should be reconciled, but it does not block the phase goal — the underlying code fully satisfies SC3.

---
*Verified: 2026-07-09*
*Verifier: Claude (gsd-verifier)*
