---
phase: 01-release-build-tooling
plan: 01
subsystem: infra
tags: [taskfile, golangci-lint, github-actions, ci, build-tooling]

# Dependency graph
requires: []
provides:
  - Single source of truth for the pinned golangci-lint version (Taskfile.yml GOLANGCI_LINT_VERSION var)
  - task tools installs golangci-lint via go install at the pinned version (replaces unpinned brew install)
  - CI's build-and-test job resolves the golangci-lint version from Taskfile.yml via command substitution
affects: [ci, build-tooling, future-tool-pinning-phases]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "silent: true Taskfile leaf task as a cross-boundary value transfer mechanism (CI reads a Taskfile var via shell command substitution, not YAML scraping)"

key-files:
  created:
    - .planning/phases/01-release-build-tooling/deferred-items.md
  modified:
    - Taskfile.yml
    - .github/workflows/ci.yml

key-decisions:
  - "None new — implementation followed CONTEXT.md decisions D-01 through D-05 and RESEARCH.md/PATTERNS.md exact diffs without deviation."

patterns-established:
  - "Taskfile-as-source-of-truth: a version lives once in Taskfile.yml vars:; a silent:true, deps-free leaf task echoes it; external consumers (CI) read it via $(task <leaf-task>) command substitution instead of duplicating the literal."

requirements-completed: [REL-01, CFG-01, CFG-02]

coverage:
  - id: D1
    description: "task tools installs golangci-lint via go install at the single Taskfile.yml-pinned version (v2.12.1), replacing unpinned brew install"
    requirement: "CFG-02"
    verification:
      - kind: other
        ref: "task tools:golangci-lint-version outputs bare 'v2.12.1' (no banner); rg confirms brew install line no longer contains golangci-lint and the go install@{{.GOLANGCI_LINT_VERSION}} line is present"
        status: pass
    human_judgment: false
  - id: D2
    description: "CI's build-and-test job resolves the golangci-lint version from Taskfile.yml via $(task tools:golangci-lint-version) instead of its own env var, closing the drift vector structurally"
    requirement: "CFG-02"
    verification:
      - kind: other
        ref: "rg confirms no GOLANGCI_LINT_VERSION literal remains in ci.yml; actionlint and yamlfmt -lint pass directly on ci.yml; lefthook pre-commit (runs actionlint on staged files) passed on commit 322ee457"
        status: pass
    human_judgment: false

# Metrics
duration: 7min
completed: 2026-07-09
status: complete
---

# Phase 1 Plan 01: Release & Build Tooling Summary

**Pinned golangci-lint to one Taskfile.yml source of truth (`v2.12.1`, installed via `go install`), with CI reading that same value via `$(task tools:golangci-lint-version)` instead of its own env var — closing the local/CI version drift (brew's `2.12.2` vs CI's pinned `2.12.1`).**

## Performance

- **Duration:** 7 min
- **Started:** 2026-07-09T03:11:02Z
- **Completed:** 2026-07-09T03:19:19Z
- **Tasks:** 2 completed
- **Files modified:** 2

## Accomplishments
- `Taskfile.yml` now declares `GOLANGCI_LINT_VERSION: v2.12.1` once, in the global `vars:` block, as the single literal declaration of the pinned version repo-wide.
- `task tools` installs golangci-lint via `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@{{.GOLANGCI_LINT_VERSION}}` (matching the two existing `protoc-gen-*` `go install` lines), no longer via unpinned `brew install golangci-lint`.
- New `tools:golangci-lint-version` leaf task (`silent: true`, no `deps:`) exposes the pinned version to external callers as a bare stdout string.
- `.github/workflows/ci.yml`'s `build-and-test` job no longer declares its own `GOLANGCI_LINT_VERSION` env var; its "Install Go tools" step now resolves the version via `$(task tools:golangci-lint-version)` command substitution.
- REL-01 and CFG-01 (already shipped prior to this plan, per 01-CONTEXT.md D-00) required no work — carried in `requirements-completed` for phase-level traceability only, consistent with REQUIREMENTS.md already marking both `[x]` Done before this plan executed.

## Task Commits

Each task was committed atomically:

1. **Task 1: Pin golangci-lint in Taskfile.yml and add the version-source leaf task** - `82073eaf` (feat)
2. **Task 2: Make ci.yml read the pinned version from Taskfile.yml instead of its own env var** - `322ee457` (fix)

**Plan metadata:** pending (this commit)

## Files Created/Modified
- `Taskfile.yml` - Added `GOLANGCI_LINT_VERSION` var; moved golangci-lint off the `brew install` line onto a pinned `go install` line; added the `tools:golangci-lint-version` silent leaf task
- `.github/workflows/ci.yml` - Removed the independent `GOLANGCI_LINT_VERSION` env var; "Install Go tools" step now resolves the version via `$(task tools:golangci-lint-version)`
- `.planning/phases/01-release-build-tooling/deferred-items.md` - New tracking file logging pre-existing, out-of-scope `task lint` failures discovered during verification (see Deviations below)

## Decisions Made
None - followed plan as specified. All implementation choices (var placement, install-line shape, leaf-task shape, ci.yml edit scope) were fully locked by 01-CONTEXT.md (D-01 through D-05) and grounded in exact diffs from 01-RESEARCH.md/01-PATTERNS.md; no open decisions remained for the executor.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Cleared stale golangci-lint cache before trusting `task lint` output**
- **Found during:** Task 2 verification (`task lint`)
- **Issue:** `task lint`'s `lint:go` sub-task initially reported 296 issues, all located under a path (`.claude/worktrees/joyful-crunching-cook/...`) that no longer exists on disk — a stale golangci-lint result cache left over from a previously deleted worktree (matches a known class of issue: cache replays lint findings against deleted workspace paths).
- **Fix:** Ran `golangci-lint cache clean`; re-ran `task lint` and the Go-lint portion passed cleanly with zero issues attributable to this plan's changes (which touch no `.go` files).
- **Files modified:** None (environment/cache state only, not a repo file)
- **Verification:** Re-ran `task lint:go`-equivalent portion after cache clean; zero issues remained.
- **Committed in:** N/A (no file change; cache is local build state, not repo content)

### Out-of-Scope Discoveries (logged, not fixed)

**2. Pre-existing `task lint` failures unrelated to this plan's files**
- **Found during:** Task 2 verification (`task lint`)
- **Issue:** After the cache-clean fix above, `task lint` still failed on two sub-tasks: `lint:markdown` (303 rumdl issues, dominated by `.planning/intel/constraints.md`) and `lint:yaml` (yamlfmt drift in `.planning/INGEST-MANIFEST.yaml`). Both files predate this plan (introduced by the docs/ corpus ingest, commit `040f5181`) and are outside this plan's `files_modified` scope (`Taskfile.yml`, `.github/workflows/ci.yml`).
- **Action:** Per the executor's scope boundary, these were NOT fixed. Logged to `.planning/phases/01-release-build-tooling/deferred-items.md` instead. Both files this plan actually edited were individually verified clean: `yamlfmt -lint` passes on both `Taskfile.yml` and `.github/workflows/ci.yml`; `actionlint` passes on `.github/workflows/ci.yml`.
- **Files modified:** None (logged only)

---

**Total deviations:** 1 auto-fixed (Rule 3, environment cache state, no file changes), 1 out-of-scope discovery logged (not fixed).
**Impact on plan:** No scope creep. The cache-clean was necessary to get a trustworthy `task lint` signal for verifying this plan's own changes; the deferred markdown/YAML issues belong to unrelated pre-existing files and are correctly left to a future pass.

## Issues Encountered
- The plan's phase-level `<verification>` section describes `task lint` as running actionlint over ci.yml. In this repo, `actionlint` actually runs via the `lefthook.yaml` pre-commit hook (on staged files, e.g. during `git commit`), not as a sub-task of `task lint` (which composes `lint:go` + `lint:markdown` + `lint:yaml` + `lint:constitution-callers` — YAML formatting only, via `yamlfmt -lint`, not schema/syntax validation). This is a documentation nuance, not a code defect: `actionlint .github/workflows/ci.yml` was run directly and passed, and the same check ran again automatically (and passed) via lefthook's pre-commit hook when Task 2's commit was created.

## Next Phase Readiness
- Phase 1 (release-build-tooling) is now fully complete: REL-01 and CFG-01 were already shipped on `main`; CFG-02 is closed by this plan. All three requirements show satisfied verification.
- The brew-based local/CI golangci-lint drift vector is closed structurally (one declaration, one install method for both sides) — no residual follow-up work identified for this specific fix.
- Deferred, out-of-scope markdown/YAML lint cleanup in `.planning/intel/constraints.md` and `.planning/INGEST-MANIFEST.yaml` remains open for whichever future phase/pass owns repo-wide lint hygiene (see `deferred-items.md`).

---
*Phase: 01-release-build-tooling*
*Completed: 2026-07-09*

## Self-Check: PASSED

- FOUND: `.planning/phases/01-release-build-tooling/01-01-SUMMARY.md`
- FOUND: `.planning/phases/01-release-build-tooling/deferred-items.md`
- FOUND: commit `82073eaf` (Task 1)
- FOUND: commit `322ee457` (Task 2)
