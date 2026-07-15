---
phase: 07-authoring-lifecycle-semantics
plan: 01
subsystem: api
tags: [lifecycle, supersede, proto, connectrpc, changelog, cli]

# Dependency graph
requires:
  - phase: 06-mcp-self-teaching-authoring
    provides: existing LifecycleService supersede path and changelog infrastructure
provides:
  - TransitionSupersedeRequest.reason proto field (field 3)
  - LifecycleSupersedeSpec(ctx, oldSlug, newSlug, reason) signature threading reason to changelog
  - CLI `supersede --reason` optional flag
  - updated lifecycle fake backends for the new signature
affects: [07-02, 07-03, mcp-supersede-reroute, lifecycle]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Optional free-text audit note threaded proto->handler->store->changelog with default fallback"

key-files:
  created: []
  modified:
    - proto/specgraph/v1/lifecycle.proto
    - gen/specgraph/v1/lifecycle.pb.go
    - internal/storage/lifecycle.go
    - internal/storage/postgres/lifecycle.go
    - internal/server/lifecycle_handler.go
    - cmd/specgraph/lifecycle.go
    - internal/server/lifecycle_handler_test.go
    - internal/server/error_sanitize_test.go
    - internal/server/test_scoper_test.go

key-decisions:
  - "Supersede reason is optional and NOT length-bounded on the lifecycle path (unlike amend's required reason), per threat register T-07-04 accept disposition."
  - "Reason precedence: caller reason used when non-empty, else default 'Superseded by <newSlug>' note (RESEARCH A2)."

patterns-established:
  - "Signature-change TDD: extend interface+impl+fakes to compile, RED with handler passing empty string, GREEN wires msg.Reason."

requirements-completed: [LIFE-01]

coverage:
  - id: D1
    description: "TransitionSupersedeRequest.reason proto field added and generated code regenerated"
    requirement: "LIFE-01"
    verification:
      - kind: unit
        ref: "grep TransitionSupersedeRequest.GetReason gen/specgraph/v1/lifecycle.pb.go; go build ./..."
        status: pass
    human_judgment: false
  - id: D2
    description: "Handler threads request Reason into LifecycleSupersedeSpec; store records it on the superseded spec changelog with default fallback"
    requirement: "LIFE-01"
    verification:
      - kind: unit
        ref: "internal/server/lifecycle_handler_test.go#TestLifecycleHandler_Supersede_ThreadsReason"
        status: pass
    human_judgment: false
  - id: D3
    description: "Postgres changelog reason precedence (caller reason vs default note)"
    requirement: "LIFE-01"
    verification: []
    human_judgment: true
    rationale: "Storage integration assertion (changelog Reason value) is owned by Plan 02's lifecycle_test.go per the plan; not covered by a unit test in this plan."
  - id: D4
    description: "CLI supersede --reason optional flag wired to the request"
    requirement: "LIFE-01"
    verification:
      - kind: manual_procedural
        ref: "go run ./cmd/specgraph supersede --help | grep -- '--reason'"
        status: pass
    human_judgment: false

# Metrics
duration: 6min
completed: 2026-07-14
status: complete
---

# Phase 7 Plan 1: Supersede Reason Threading Summary

**Optional `reason` threaded through the supersede lifecycle path (proto field, `LifecycleSupersedeSpec` signature, handler, changelog, and CLI `--reason` flag) so both CLI and MCP can record why a done spec was replaced.**

## Performance

- **Duration:** 6 min
- **Started:** 2026-07-14T21:25:27Z
- **Completed:** 2026-07-14T21:31:52Z
- **Tasks:** 3 (Task 2 executed as RED→GREEN)
- **Files modified:** 9

## Accomplishments
- Added `string reason = 3` to `TransitionSupersedeRequest` and regenerated `gen/`.
- Changed `LifecycleSupersedeSpec` to `(ctx, oldSlug, newSlug, reason)` across the interface, postgres impl, and all three lifecycle fakes.
- Handler now passes `msg.Reason`; the superseded spec's changelog records the caller reason when non-empty, else the `Superseded by <newSlug>` default.
- Added optional CLI `supersede --reason` flag (not marked required).

## Task Commits

Each task was committed atomically:

1. **Task 1: Add optional reason field to TransitionSupersedeRequest and regenerate** - `fbc8df2d` (feat)
2. **Task 2: Thread reason through LifecycleSupersedeSpec, handler, and fakes (TDD)** - `71613379` (test/RED) → `f84ae843` (feat/GREEN)
3. **Task 3: Add optional --reason flag to CLI supersede** - `1d70f230` (feat)

_Note: Task 2 was a TDD task producing a RED test commit followed by a GREEN implementation commit._

## Files Created/Modified
- `proto/specgraph/v1/lifecycle.proto` - Added `reason = 3` to `TransitionSupersedeRequest`.
- `gen/specgraph/v1/lifecycle.pb.go` - Regenerated (Reason accessor).
- `internal/storage/lifecycle.go` - `LifecycleSupersedeSpec` interface signature + doc.
- `internal/storage/postgres/lifecycle.go` - Impl signature + reason precedence in old-spec changelog.
- `internal/server/lifecycle_handler.go` - `TransitionSupersede` passes `msg.Reason`.
- `cmd/specgraph/lifecycle.go` - `supersedeReason` var, request wiring, `--reason` flag registration.
- `internal/server/lifecycle_handler_test.go` - Fake field/method signature, callsite updates, `TestLifecycleHandler_Supersede_ThreadsReason`.
- `internal/server/error_sanitize_test.go` - `errorBackend.LifecycleSupersedeSpec` signature.
- `internal/server/test_scoper_test.go` - `stubBackend.LifecycleSupersedeSpec` signature.

## Decisions Made
- Supersede reason is optional and intentionally unbounded on the lifecycle path (threat register T-07-04, accept). No new validation added — mirrors the fact that supersede is a done-only transition writing an internal changelog audit note.
- Reason precedence follows RESEARCH A2: caller reason wins when non-empty; otherwise the existing `Superseded by <newSlug>` default is preserved.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## TDD Gate Compliance
- RED gate: `71613379` `test(07-01): add failing test for supersede reason threading` — test failed at runtime (captured reason `""` != `"replaced by clearer design"`) because the handler temporarily passed an empty string.
- GREEN gate: `f84ae843` `feat(07-01): thread supersede reason into changelog` — handler passes `msg.Reason`; test passes.
- REFACTOR gate: not needed.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The `LifecycleSupersedeSpec(ctx, oldSlug, newSlug, reason)` signature and `TransitionSupersedeRequest.reason` field are in place; Plan 03 (MCP supersede reroute) can build on the single lifecycle path.
- Plan 02 owns the storage integration test asserting the changelog `Reason` value (deferred here per plan).

## Self-Check: PASSED
- All modified files verified present on disk.
- All four commits (`fbc8df2d`, `71613379`, `f84ae843`, `1d70f230`) verified in git log.
- `task check` exits 0 (fmt, license, lint, build, unit tests).

---
*Phase: 07-authoring-lifecycle-semantics*
*Completed: 2026-07-14*
