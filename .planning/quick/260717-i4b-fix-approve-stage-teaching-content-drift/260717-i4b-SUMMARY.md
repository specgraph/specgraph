---
phase: 260717-i4b
plan: 01
subsystem: authoring
tags: [mcp, embedded-content, conversation-recording, approve-stage]

# Dependency graph
requires:
  - phase: 08 (Authoring Lifecycle Semantics — CONV-01)
    provides: Server/client enforcement making conversation exchanges REQUIRED on approve for both accept and reject dispositions (ValidateExchanges, handleApprove hard-reject)
provides:
  - Corrected `conversation-recording.md` Approve Special Case section teaching exchanges REQUIRED on both accept and reject
  - Removed dangling reference to a standalone MCP conversation-record tool action (deleted in Phase 8); amendment described as CLI-only
  - Corrected `stage-approve.md` Accept path subsection stating exchanges are REQUIRED, matching the Reject path and SKILL.md
  - Regenerated golden fixtures (`testdata/golden/*.md`) to match corrected composed content
affects: [authoring, mcp-composer, approve-stage-teaching]

# Tech tracking
tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified:
    - internal/authoring/content/conversation-recording.md
    - internal/authoring/content/stage-approve.md
    - internal/authoring/testdata/golden/spark.md
    - internal/authoring/testdata/golden/shape.md
    - internal/authoring/testdata/golden/specify.md
    - internal/authoring/testdata/golden/decompose.md
    - internal/authoring/testdata/golden/approve.md

key-decisions:
  - "Mirrored SKILL.md lines 176-179 wording exactly rather than inventing new phrasing, per plan instruction to align to the already-correct reference"
  - "Post-hoc amendment of a prior recording described as CLI-only (`specgraph conversation record <slug>`) rather than deleted outright, since the capability genuinely exists via CLI"

patterns-established: []

requirements-completed: [CONV-01]

coverage:
  - id: D1
    description: "conversation-recording.md Approve Special Case teaches exchanges REQUIRED on both accept and reject dispositions; no dangling MCP standalone conversation-record tool reference remains"
    requirement: "CONV-01"
    verification:
      - kind: unit
        ref: "grep gates (no 'only on rejection', no 'self-evident', no 'standalone conversation-record tool'; presence of 'accept' and 'REQUIRED') + go build ./..."
        status: pass
      - kind: unit
        ref: "go test ./internal/authoring/... (TestComposeGolden — snapshots the composed prose these edits change, regenerated via -update; TestEmbeddedContent_Present; TestContentPersistenceContractSnakeCase)"
        status: pass
      - kind: unit
        ref: "task check (fmt:check, license:check, lint, build, unit tests) — run on the PR branch after CodeRabbit review"
        status: pass
    human_judgment: false
  - id: D2
    description: "stage-approve.md Accept path subsection explicitly states exchanges are REQUIRED, matching the Reject path and SKILL.md; Reject path left unchanged"
    requirement: "CONV-01"
    verification:
      - kind: unit
        ref: "grep gates (>=2 REQUIRED occurrences; Accept-path-scoped awk/grep for 'exchange' and 'REQUIRED') + go build ./..."
        status: pass
      - kind: unit
        ref: "go test ./internal/authoring/... (TestComposeGolden approve subtest — snapshots composed approve-stage prose, regenerated via -update)"
        status: pass
      - kind: unit
        ref: "task check (fmt:check, license:check, lint, build, unit tests) — run on the PR branch after CodeRabbit review"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-07-17
status: complete
---

# Quick Task 260717-i4b: Fix Approve-Stage Teaching Content Drift Summary

**Aligned two `//go:embed`'d MCP teaching files to CONV-01's enforced behavior — conversation exchanges are now taught as REQUIRED on approve for both accept and reject, matching SKILL.md and the server/client enforcement shipped in Phase 8.**

## Performance

- **Duration:** ~15 min
- **Completed:** 2026-07-17
- **Tasks:** 2 completed
- **Files modified:** 7 (2 content files + 5 golden fixtures)

## Accomplishments
- `conversation-recording.md`'s "Approve Special Case" section now teaches exchanges REQUIRED on approve for BOTH accept and reject dispositions, replacing the stale "record only on rejection / clean approvals self-evident" framing.
- Removed the dangling claim that a standalone MCP conversation-record tool action exists; the amendment capability is now correctly described as CLI-only (`specgraph conversation record <slug>`), matching the actual `tools_authoring.go` surface (only `list` action remains — `record` was deleted in Phase 8).
- `stage-approve.md`'s "Accept path" subsection now explicitly states exchanges capturing the approval rationale are REQUIRED and load-bearing for audit, matching the strength of the adjacent (already-correct) "Reject path" subsection and SKILL.md lines 176-179.
- Regenerated `TestComposeGolden` fixtures for all five stages (conversation-recording.md is composed into every stage's `author_start_stage` response) to reflect the corrected prose.

## Task Commits

Each task was committed atomically:

1. **Task 1: Correct conversation-recording.md** - `9552ff62` (fix)
2. **Task 2: Correct stage-approve.md** - `8fd8151e` (fix)

**Plan metadata:** committed separately by the orchestrator (SUMMARY.md, STATE.md not committed by this executor per constraints)

## Files Created/Modified
- `internal/authoring/content/conversation-recording.md` - Approve Special Case rewritten (REQUIRED on both dispositions); standalone MCP record-tool reference replaced with CLI-only pointer
- `internal/authoring/content/stage-approve.md` - Accept path subsection now states exchanges REQUIRED, matching Reject path
- `internal/authoring/testdata/golden/spark.md` - Regenerated to reflect conversation-recording.md change
- `internal/authoring/testdata/golden/shape.md` - Regenerated to reflect conversation-recording.md change
- `internal/authoring/testdata/golden/specify.md` - Regenerated to reflect conversation-recording.md change
- `internal/authoring/testdata/golden/decompose.md` - Regenerated to reflect conversation-recording.md change
- `internal/authoring/testdata/golden/approve.md` - Regenerated twice (once per task) to reflect both content changes

## Decisions Made
- Mirrored SKILL.md's existing correct wording rather than composing fresh phrasing, per the plan's explicit instruction to treat SKILL.md lines 176-179 as the reference source of truth.
- Kept the amendment-capability pointer in `conversation-recording.md` rather than deleting it outright, since `specgraph conversation record <slug>` genuinely exists as a CLI-only path (verified against `cmd/specgraph/conversation.go`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Regenerated `TestComposeGolden` fixtures after content edits broke golden-diff assertions**
- **Found during:** Task 1 verification (running `go test ./internal/authoring/...` per the plan's overall `<verification>` block)
- **Issue:** `internal/authoring/composer_golden_test.go` snapshots the fully-composed per-stage MCP content into `testdata/golden/*.md`. Editing `conversation-recording.md` (composed into all 5 stages) and `stage-approve.md` (composed into the approve stage) caused `TestComposeGolden` to fail for all 5 stage subtests, since the golden fixtures still held the pre-edit prose. This wasn't explicitly called out in the plan's task-level `<verify>` blocks (which only grep-gate the source `.md` files + `go build`), but the plan's overall `<verification>` block does say to run `go test ./internal/authoring/...` and expects it to pass.
- **Fix:** Regenerated fixtures via `go test ./internal/authoring/ -run TestComposeGolden -update`, run twice to keep the two tasks' golden-fixture changes atomic and scoped: once after Task 1's edit alone (updating spark/shape/specify/decompose/approve), and again after Task 2's edit (updating only approve.md further, since stage-approve.md is composed only into the approve stage).
- **Files modified:** `internal/authoring/testdata/golden/{spark,shape,specify,decompose,approve}.md`
- **Verification:** Diffed each golden file to confirm changes matched only the intended prose edits (no unrelated drift); reran `go test ./internal/authoring/...` and `go build ./...` — both pass.
- **Committed in:** `9552ff62` (Task 1 commit, 4 golden files + spark/shape/specify/decompose/approve conversation-recording delta) and `8fd8151e` (Task 2 commit, approve.md's additional stage-approve.md delta)

---

**Total deviations:** 1 auto-fixed (1 blocking test-infrastructure fix)
**Impact on plan:** Necessary to keep `go test ./internal/authoring/...` green as required by the plan's own verification block. No scope creep — golden fixtures are generated test data reflecting the exact same prose changes the plan specified, nothing additional.

## Issues Encountered
None beyond the golden-fixture regeneration documented above.

## Post-Review Follow-up (PR #1011, CodeRabbit)

CodeRabbit flagged 4 items on the PR: two documentation-accuracy issues in this
PLAN.md/SUMMARY.md (the "no test changes" scope statement and the "no existing
test snapshots approve wording" claim, both corrected in PLAN.md and above),
one request to record `task check` (added above and confirmed passing on this
branch), and one content-correctness item — `conversation-recording.md`'s
Approve Special Case didn't state that Approve has no `output` payload at all
(unlike Shape/Specify/Decompose), which the original plan's task action had
specified but the executed edit omitted. Fixed in commit `0611d9b7`, with
golden fixtures regenerated again.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- A fresh MCP-only agent following the composed approve-stage guidance will now be taught to record exchanges on a clean acceptance, matching what the server and MCP client actually enforce (CONV-01).
- No further follow-up needed; this was a scoped documentation-only fix with no Go/proto/test-suite changes beyond fixture regeneration.

---
*Quick task: 260717-i4b*
*Completed: 2026-07-17*

## Self-Check: PASSED

All created/modified files verified present on disk; both task commit hashes (`9552ff62`, `8fd8151e`) verified present in git log.
