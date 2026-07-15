---
phase: 07-authoring-lifecycle-semantics
plan: 03
subsystem: api
tags: [lifecycle, amend, supersede, mcp, connectrpc, allowlist, re-entry]

# Dependency graph
requires:
  - phase: 07-authoring-lifecycle-semantics
    provides: LifecycleAmendSpec claim-release (07-02) and TransitionSupersedeRequest.reason proto field + LifecycleSupersedeSpec reason threading (07-01)
provides:
  - SpecStage.IsValidReEntryStage() single-source-of-truth re-entry allowlist (spark|shape|specify|decompose)
  - TransitionAmend handler + LifecycleAmendSpec storage enforce IsValidReEntryStage (rejects approved/in_progress/review/done)
  - MCP author tool amend/supersede rerouted to LifecycleService with re_entry_stage/new_slug params, required amend reason, and a next-step hint
affects: [07-04, 07-05, mcp-authoring, lifecycle]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Single-source-of-truth validation helper (IsValidReEntryStage) called from both the RPC handler and the storage layer for defense-in-depth"
    - "MCP tool as a thin router to the canonical ConnectRPC handler — no duplicate value-allowlist in the tool"

key-files:
  created: []
  modified:
    - internal/storage/spec_domain.go
    - internal/storage/spec_domain_test.go
    - internal/server/lifecycle_handler.go
    - internal/server/lifecycle_handler_test.go
    - internal/storage/postgres/lifecycle.go
    - internal/storage/postgres/lifecycle_test.go
    - internal/mcp/tools_authoring.go
    - internal/mcp/tools_authoring_test.go
    - internal/mcp/testhelpers_test.go

key-decisions:
  - "IsValidReEntryStage implemented as an explicit four-value switch (not a membership test over authoringStages, which includes approved as its 5th element) to prevent silently reintroducing the review HIGH bug."
  - "TransitionAmend re_entry_stage validation collapsed to a single path (empty-check then !IsValidReEntryStage) — the redundant IsValid()+ExcludesReEntry() branch that wrongly accepted approved/in_progress/review was removed."
  - "The MCP amend/supersede handlers do presence guards only (empty slug/re_entry_stage/reason/new_slug); value validation stays in the single RPC gate to avoid drift."

patterns-established:
  - "Terminal-only exclusion (ExcludesReEntry) is documented as distinct from the amend re-entry allowlist (IsValidReEntryStage) so future readers do not conflate them."

requirements-completed: [LIFE-01, LIFE-02]

coverage:
  - id: D1
    description: "SpecStage.IsValidReEntryStage() returns true only for spark|shape|specify|decompose; approved/in_progress/review/done/terminal/unknown are false"
    requirement: "LIFE-02"
    verification:
      - kind: unit
        ref: "internal/storage/spec_domain_test.go#TestSpecStage_IsValidReEntryStage"
        status: pass
    human_judgment: false
  - id: D2
    description: "TransitionAmend rejects re_entry_stage in {approved, in_progress, review, done} with CodeInvalidArgument (asserted by code, not message) for both CLI and MCP"
    requirement: "LIFE-02"
    verification:
      - kind: unit
        ref: "internal/server/lifecycle_handler_test.go#TestLifecycleHandler_Amend_NonAuthoringReEntryStage"
        status: pass
    human_judgment: false
  - id: D3
    description: "LifecycleAmendSpec storage guard rejects approved/in_progress/review (+ done/superseded/abandoned) re-entry via ErrInvalidReEntryStage — defense-in-depth"
    requirement: "LIFE-02"
    verification:
      - kind: integration
        ref: "internal/storage/postgres/lifecycle_test.go#TestLifecycle/AmendSpec_InvalidReEntryStage"
        status: pass
    human_judgment: false
  - id: D4
    description: "MCP author amend routes to Lifecycle.TransitionAmend with slug/reason/re_entry_stage, requires reason, and emits a next-step hint referencing re_entry_stage"
    requirement: "LIFE-02"
    verification:
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestAuthorTool_Amend"
        status: pass
    human_judgment: false
  - id: D5
    description: "MCP author supersede routes to Lifecycle.TransitionSupersede with slug/new_slug/reason; new_slug required, reason optional"
    requirement: "LIFE-01"
    verification:
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestAuthorTool_Supersede"
        status: pass
    human_judgment: false
  - id: D6
    description: "Tool surfaces the sanitized handler connect error (res.IsError) on sentinel failures rather than a tool-side re-implementation"
    requirement: "LIFE-02"
    verification:
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestAuthorTool_Amend_SurfacesHandlerError"
        status: pass
    human_judgment: false

# Metrics
duration: 12min
completed: 2026-07-14
status: complete
---

# Phase 7 Plan 3: Re-entry Allowlist + MCP Amend/Supersede Reroute Summary

**A single-source-of-truth `SpecStage.IsValidReEntryStage()` allowlist (spark|shape|specify|decompose) enforced at both the `TransitionAmend` handler and `LifecycleAmendSpec` storage, plus the MCP `author` tool's amend/supersede handlers rerouted onto `LifecycleService` with `re_entry_stage`/`new_slug` params, required amend `reason`, and a next-step hint — closing the review HIGH re-entry gap and the #899 MCP no-op atomically.**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-07-14T21:41:00Z
- **Completed:** 2026-07-14T21:53:16Z
- **Tasks:** 3 (Task 1 executed as RED→GREEN)
- **Files modified:** 9

## Accomplishments
- Added `SpecStage.IsValidReEntryStage()` — an explicit four-value switch (spark|shape|specify|decompose) that is the single D-03 allowlist, deliberately NOT built on `authoringStages` (which includes `approved`).
- Collapsed the `TransitionAmend` re_entry_stage validation to one path that now rejects `approved`/`in_progress`/`review`/`done` with `CodeInvalidArgument` for both CLI and MCP callers (fixes the review HIGH bug).
- Tightened `LifecycleAmendSpec` storage to guard on `!IsValidReEntryStage()` and extended the integration test to reject `approved`/`in_progress`/`review` in addition to the terminal stages (defense-in-depth).
- Rerouted MCP `handleAmend`→`Lifecycle.TransitionAmend` and `handleSupersede`→`Lifecycle.TransitionSupersede`; renamed params `target_stage`→`re_entry_stage` and `superseded_by`→`new_slug`; added required-`reason` guard (D-04) and a next-step hint naming the `author action=<re_entry_stage>` to run (D-05).
- Rewrote the tool docs/Description to teach the land-one-before model and deleted `authoringStageFromString` + `TestAuthoringStageFromString`.

## Task Commits

Each task was committed atomically:

1. **Task 1: IsValidReEntryStage allowlist + handler/storage enforcement (TDD)** — `b2aec4a3` (test/RED) → `71341a08` (feat/GREEN)
2. **Task 2: Reroute handleAmend/handleSupersede to Lifecycle, rename/redocument params, next-step hint** — `180e348f` (feat)
3. **Task 3: Rewrite MCP amend/supersede tool tests against a lifecycle mock** — `75a7f4ff` (test)

**Plan metadata:** _(this SUMMARY commit)_ (docs)

_Note: Task 1 was TDD (RED test commit → GREEN implementation commit). Tasks 2 and 3 are the plan's intentional implementation/test split for the same MCP-package change; between them the working tree stayed consistent (the test package always compiled)._

## Files Created/Modified
- `internal/storage/spec_domain.go` — Added `IsValidReEntryStage()`; clarified `ExcludesReEntry` doc as terminal-only.
- `internal/storage/spec_domain_test.go` — `TestSpecStage_IsValidReEntryStage` (asserts approved==false footgun).
- `internal/server/lifecycle_handler.go` — Single-path re_entry_stage validation using `IsValidReEntryStage`.
- `internal/server/lifecycle_handler_test.go` — `TestLifecycleHandler_Amend_NonAuthoringReEntryStage` (approved/in_progress/review/done → CodeInvalidArgument).
- `internal/storage/postgres/lifecycle.go` — `LifecycleAmendSpec` guard tightened to `!IsValidReEntryStage()`.
- `internal/storage/postgres/lifecycle_test.go` — `AmendSpec_InvalidReEntryStage` extended to approved/in_progress/review.
- `internal/mcp/tools_authoring.go` — Rerouted amend/supersede handlers, renamed params, next-step hint, rewritten docs, deleted `authoringStageFromString`.
- `internal/mcp/tools_authoring_test.go` — Rewrote amend/supersede tests against the lifecycle mock; deleted `TestAuthoringStageFromString`.
- `internal/mcp/testhelpers_test.go` — Extended `mockLifecycleService` with `TransitionAmend`/`TransitionSupersede` (records request, returns configurable response or sentinel error).

## Decisions Made
- **Explicit switch over slice membership** for `IsValidReEntryStage` — reusing `authoringStages` would accept `approved` and silently reintroduce the review HIGH bug.
- **Single validation gate** — the MCP tool passes `re_entry_stage` straight through and does presence guards only; the handler owns the value allowlist so the two cannot drift.
- **Left `mockAuthoringService.Amend/Supersede` methods in place** — Plan 07-04 owns their removal after the proto deletion; leaving them is harmless and keeps this plan scoped.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- A pre-existing `TestAuthorTool_Amend_MissingSlug` already covered the empty-slug guard, so the duplicate I initially added was removed before committing (resolved during Task 3; no functional impact).

## TDD Gate Compliance
- **Task 1 (type: tdd):** RED `b2aec4a3` (`test(07-03)` — storage test failed to compile on the undefined `IsValidReEntryStage`, handler test failed with CodeInternal for `approved`) → GREEN `71341a08` (`feat(07-03)` — helper added, handler/storage tightened; all unit tests pass). No REFACTOR needed.
- **Tasks 2 & 3:** the plan's intentional implementation-then-test split for one MCP-package change; Task 2 verified via `go build` + param grep, Task 3 added the lifecycle-mock assertions (`go test ./internal/mcp/...` green).

## Verification Results
- `go build ./... && go vet ./...` — green.
- `go test ./internal/storage/... ./internal/server/... ./internal/mcp/...` — green.
- `task check` (fmt, license, lint, build, unit tests) — exit 0.
- `task test:integration`-equivalent (`go test -tags integration -run TestLifecycle ./internal/storage/postgres/...`, Docker available) — green; `AmendSpec_InvalidReEntryStage` PASS including approved/in_progress/review.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The MCP surface now uses the single correct lifecycle implementation with a single-source-of-truth re-entry allowlist. Plan 07-04 (proto deletion of the old Authoring amend/supersede path + removal of the now-unused mock methods) and Plan 07-05 (MCP-only e2e for amend/supersede) are unblocked.

## Self-Check: PASSED
- All 9 modified files verified present on disk.
- Commits `b2aec4a3`, `71341a08`, `180e348f`, `75a7f4ff` verified in git log.
- `task check` exits 0; integration `TestLifecycle` suite passes under Docker.

---
*Phase: 07-authoring-lifecycle-semantics*
*Completed: 2026-07-14*
