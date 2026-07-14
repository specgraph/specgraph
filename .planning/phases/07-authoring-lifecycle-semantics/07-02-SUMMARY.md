---
phase: 07-authoring-lifecycle-semantics
plan: 02
subsystem: database
tags: [lifecycle, amend, claims, lease, supersede, changelog, postgres, integration-tests]

# Dependency graph
requires:
  - phase: 07-authoring-lifecycle-semantics
    provides: LifecycleSupersedeSpec(ctx, oldSlug, newSlug, reason) signature and reason-precedence changelog (07-01)
provides:
  - Conditional claim + CLAIMED_BY edge release inside LifecycleAmendSpec (D-08)
  - Storage integration tests pinning claim-release, re-entry landing, done-only supersede, and supersede reason
affects: [07-03, mcp-supersede-reroute, lifecycle, claims, execution]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Lease invalidation on state regression: releasing a claim + its edge inside the same transaction that returns a spec to authoring"

key-files:
  created: []
  modified:
    - internal/storage/postgres/lifecycle.go
    - internal/storage/postgres/lifecycle_test.go
    - internal/storage/postgres/changelog_test.go
    - internal/storage/postgres/graph_ready_provenance_test.go

key-decisions:
  - "Claim release is conditional on GetActiveClaim returning non-nil — unclaimed (approved) specs are a harmless no-op, no spurious deletes."
  - "Slices table intentionally left intact on amend (T-07-06 accept): slices are re-authored decompose output, not stale lease state."
  - "CLAIMED_BY is an internal-only edge type (not in the EdgeType enum), so the test asserts its absence via a direct edges-table count rather than ListEdges."

patterns-established:
  - "Transaction-scoped lease invalidation: any lifecycle transition that makes a spec non-executable deletes the claims row and CLAIMED_BY edge with the same txCtx."

requirements-completed: [LIFE-01, LIFE-02]

coverage:
  - id: D1
    description: "Amending an in_progress/review spec deletes its active claim row and CLAIMED_BY edge inside the amend transaction (D-08)"
    requirement: "LIFE-02"
    verification:
      - kind: integration
        ref: "internal/storage/postgres/lifecycle_test.go#TestLifecycleAmend_ReleasesClaim/ClaimedSpec_ReleasesClaimAndEdge"
        status: pass
    human_judgment: false
  - id: D2
    description: "Amending an unclaimed (approved) spec is a harmless no-op with respect to claims — no error, no residual claim/edge"
    requirement: "LIFE-02"
    verification:
      - kind: integration
        ref: "internal/storage/postgres/lifecycle_test.go#TestLifecycleAmend_ReleasesClaim/UnclaimedSpec_NoErrorNoClaim"
        status: pass
    human_judgment: false
  - id: D3
    description: "Amend lands the spec one stage before the re-entry target (PrecedingAuthStage)"
    requirement: "LIFE-02"
    verification:
      - kind: integration
        ref: "internal/storage/postgres/lifecycle_test.go#TestLifecycleAmend_ReleasesClaim (stage assertion)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Supersede changelog Reason reflects the supplied reason (non-empty) and the default 'Superseded by <new>' note when empty"
    requirement: "LIFE-01"
    verification:
      - kind: integration
        ref: "internal/storage/postgres/lifecycle_test.go#TestLifecycle/SupersedeSpec_ReasonThreaded"
        status: pass
    human_judgment: false
  - id: D5
    description: "Supersede on a non-done spec returns storage.ErrSpecNotDone"
    requirement: "LIFE-01"
    verification:
      - kind: integration
        ref: "internal/storage/postgres/lifecycle_test.go#TestLifecycle/SupersedeSpec_NotDone"
        status: pass
    human_judgment: false

# Metrics
duration: 9min
completed: 2026-07-14
status: complete
---

# Phase 7 Plan 2: Amend Claim-Release + Lifecycle Storage Tests Summary

**A spec amended back to authoring now releases its active claim and CLAIMED_BY edge inside the same transaction (D-08, closing the stale-lease gap #900), with storage integration tests pinning claim-release, re-entry landing, done-only supersede, and supersede-reason semantics.**

## Performance

- **Duration:** ~9 min
- **Started:** 2026-07-14T21:33:00Z
- **Completed:** 2026-07-14T21:42:03Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- `LifecycleAmendSpec` fetches the active claim via `GetActiveClaim(txCtx, slug)` and, when non-nil, deletes the `claims` row (agent-scoped) and the `CLAIMED_BY` edge — both inside the amend transaction. Unclaimed specs are a no-op. The `slices` table is untouched.
- Added `TestLifecycleAmend_ReleasesClaim` (claimed → claim + edge gone, lands at `PrecedingAuthStage`; unclaimed → no error/no claim).
- Added `SupersedeSpec_ReasonThreaded` asserting the changelog `Reason` for a supplied reason vs the default `Superseded by <new>` note.
- Verified the full lifecycle suite passes against a real Postgres container (`task test:integration`-equivalent run).

## Task Commits

Each task was committed atomically:

1. **Task 1: Release active claim + CLAIMED_BY edge inside LifecycleAmendSpec** - `9e9ef761` (feat)
2. **Task 2: Storage integration tests — claim release, supersede reason, re-entry landing** - `b2a752be` (test)

**Plan metadata:** _(this SUMMARY commit)_ (docs)

## Files Created/Modified
- `internal/storage/postgres/lifecycle.go` - Conditional claim + CLAIMED_BY edge release inside the `LifecycleAmendSpec` transaction (D-08).
- `internal/storage/postgres/lifecycle_test.go` - New `TestLifecycleAmend_ReleasesClaim` and `SupersedeSpec_ReasonThreaded`; helpers `latestSupersededReason`, `countClaimedByEdges`, `mustGetActiveClaim`; existing supersede callsites updated to the 4-arg signature.
- `internal/storage/postgres/changelog_test.go` - Updated stale `LifecycleSupersedeSpec` callsite to the 4-arg signature (blocking-fix, see Deviations).
- `internal/storage/postgres/graph_ready_provenance_test.go` - Updated stale `LifecycleSupersedeSpec` callsite to the 4-arg signature (blocking-fix, see Deviations).

## Decisions Made
- **Conditional deletion, not unconditional:** `GetActiveClaim` guards the two DELETEs so amending an unclaimed (approved) spec produces no spurious deletes and no error — matching D-08's "harmless no-op" requirement.
- **Slices left intact** per threat register T-07-06 (accept): slices are the re-authored decompose output, not stale lease state.
- **CLAIMED_BY edge assertion via direct DB count:** `CLAIMED_BY` is an internal-only edge type (not in the `EdgeType` enum, so `ListEdges` cannot filter it). The test queries the `edges` table directly through a `pgxpool` connection (project `test`) to assert edge absence.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated stale `LifecycleSupersedeSpec` callsites in two integration test files**
- **Found during:** Task 2 (integration `go vet` / build)
- **Issue:** Plan 07-01 changed `LifecycleSupersedeSpec` to a 4-arg signature `(ctx, oldSlug, newSlug, reason)` but left three integration-tagged test callsites at the old 3-arg form. Because `task check` excludes `//go:build integration`, these never failed 07-01's gate — but they blocked this plan's `go build -tags integration` / `task test:integration`. Two of the files (`changelog_test.go`, `graph_ready_provenance_test.go`) are outside this plan's declared `files_modified`.
- **Fix:** Appended `, ""` (empty reason, preserving prior behavior) to the stale callsites so the integration suite compiles. The `lifecycle_test.go` callsites were in-scope and updated as part of Task 2.
- **Files modified:** internal/storage/postgres/changelog_test.go, internal/storage/postgres/graph_ready_provenance_test.go
- **Verification:** `go vet -tags integration ./...` clean; the affected suites (`TestChangeLog*`, `TestGraphReady*`) pass under Docker.
- **Committed in:** `b2a752be` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary to compile and run the integration suite this plan's Task 2 requires. No behavior change — empty reason preserves the pre-07-01 default-note path. No scope creep beyond restoring compilation.

## Issues Encountered
None beyond the blocking-fix above.

## TDD Gate Compliance
This plan's two tasks are decomposed as implementation-first (Task 1) then integration-tests (Task 2), per the PLAN's explicit ordering and per-task `<verify>` blocks (Task 1 verifies via `go build`/`go vet`; Task 2 owns the integration assertions). The integration tests in Task 2 validate Task 1's claim-release code path against a real Postgres container:
- `feat(07-02)` (`9e9ef761`) — claim-release implementation, build + vet green.
- `test(07-02)` (`b2a752be`) — integration tests; all new subtests PASS under Docker.

Because these are Docker-gated integration tests decomposed into a separate task after the implementation task, the cycle is tests-after (GREEN-validating) rather than a strict RED-first commit. The new tests were confirmed to exercise the new code path (claim + edge deletion, reason precedence).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Claim-release on amend (D-08) is in place; Plan 07-03 (MCP supersede reroute), which depends on claim-release landing, is unblocked.
- Supersede-reason storage semantics from 07-01 are now pinned by integration tests (the D3 coverage gap 07-01 deferred is closed here).

## Self-Check: PASSED
- `internal/storage/postgres/lifecycle.go`, `lifecycle_test.go`, `changelog_test.go`, `graph_ready_provenance_test.go` verified present on disk.
- Commits `9e9ef761` and `b2a752be` verified in git log.
- `task check` exits 0 (fmt, license, lint, build, unit tests).
- Integration suite (`TestLifecycleAmend_ReleasesClaim`, `TestLifecycle/SupersedeSpec_ReasonThreaded`, `TestLifecycle`, `TestChangeLog*`, `TestGraphReady*`) passes under Docker.

---
*Phase: 07-authoring-lifecycle-semantics*
*Completed: 2026-07-14*
