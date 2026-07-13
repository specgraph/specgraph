---
phase: 04-verification-integration-reliability
plan: 01
subsystem: testing
tags: [drift, e2e, integration, ginkgo, testcontainers, connectrpc, postgres]

# Dependency graph
requires:
  - phase: 03-coordination-export
    provides: LifecycleService.CheckDrift/AcknowledgeDrift + drift engine (already shipped)
provides:
  - e2e no-false-positive-on-unrelated-edit drift proof (headline SC#2)
  - real-DB full-graph mixed-state SkippedCount integration proof
  - e2e per-upstream (UpstreamSlug) acknowledge round-trip proof
affects: [drift-detection, DRFT-02, verification-audits]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Three-done-spec seed to prove no-false-positive on unrelated edits"
    - "drift.NewEngine(store, nil).Check(ctx, \"\", \"deps\") for real-DB full-graph SkippedCount assertions from package postgres_test"
    - "Per-subtest clearDatabase isolation to avoid colliding with the e2e (all specs) blanket-zero assertion"

key-files:
  created: []
  modified:
    - e2e/api/lifecycle_test.go
    - internal/storage/postgres/lifecycle_test.go

key-decisions:
  - "Verification-only: no drift engine, converter, scope-table, proto, or migration change (D-01/D-02/D-03)"
  - "Full-graph SkippedCount proof placed at the integration layer (isolated clearDatabase) rather than e2e, to avoid the ordering collision with the existing (all specs) blanket-zero assertion"

patterns-established:
  - "No-false-positive drift proof: seed three done specs, wire only downstream→upstream, mutate the unrelated one, assert downstream clean"
  - "Per-upstream ack round-trip: UpstreamSlug (not All) + timestampSkew + 3-attempt retry idiom"

requirements-completed: [DRFT-01]

coverage:
  - id: D1
    description: "Editing a spec that is NOT an upstream dependency produces no drift on a downstream done-spec (headline SC#2 no-false-positive), proven against a real DB through LifecycleService.CheckDrift"
    requirement: "DRFT-01"
    verification:
      - kind: e2e
        ref: "e2e/api/lifecycle_test.go#Drift detection (no false-positive on unrelated edit)"
        status: pass
    human_judgment: false
  - id: D2
    description: "A full-graph CheckDrift (empty slug) over a mixed seed (drifted-done + clean-done + non-done) reports SkippedCount >= 1 and surfaces exactly the drifted spec, proven against a real DB"
    requirement: "DRFT-01"
    verification:
      - kind: integration
        ref: "internal/storage/postgres/lifecycle_test.go#TestLifecycle/CheckAllSpecs_MixedState_SkippedCount"
        status: pass
    human_judgment: false
  - id: D3
    description: "A per-upstream (UpstreamSlug) AcknowledgeDrift re-baselines content_hash_at_link and a subsequent CheckDrift on the downstream reports no drift items, proven end-to-end through the interface"
    requirement: "DRFT-01"
    verification:
      - kind: e2e
        ref: "e2e/api/lifecycle_test.go#Drift detection (per-upstream acknowledge)"
        status: pass
    human_judgment: false

# Metrics
duration: 4min
completed: 2026-07-10
status: complete
---

# Phase 4 Plan 01: DRFT-01 SC#2 Verification Tests Summary

**Three real-DB drift proofs close the SC#2 gaps for DRFT-01: no-false-positive on an unrelated edit (e2e), full-graph mixed-state SkippedCount (integration), and per-upstream acknowledge round-trip (e2e) — no engine/proto/schema change.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-07-10T16:08:01Z
- **Completed:** 2026-07-10T16:12:28Z
- **Tasks:** 3
- **Files modified:** 2

## Accomplishments
- Added the headline SC#2 e2e proof: editing an unrelated spec yields **zero** drift on a downstream done-spec (`Drift detection (no false-positive on unrelated edit)`), with a Pitfall-2 sanity guard (the downstream→upstream edge stays in the seed so the path is genuinely exercised).
- Added a real-DB `CheckAllSpecs_MixedState_SkippedCount` integration subtest that drives `drift.NewEngine(store, nil).Check(ctx, "", "deps")` over a drifted-done + clean-done + non-done seed and asserts `SkippedCount >= 1` with exactly the drifted downstream in `Reports` (clean-done filtered out as zero-item).
- Added an e2e per-upstream acknowledge round-trip (`Drift detection (per-upstream acknowledge)`) using `UpstreamSlug` (not `All`) + the timestampSkew/3-attempt retry idiom, proving the ack re-baselines `content_hash_at_link` and a re-check reports clean.

## Task Commits

Each task was committed atomically:

1. **Task 1: e2e no-false-positive-on-unrelated-edit spec** - `3b00de73` (test)
2. **Task 2: real-DB full-graph mixed-state SkippedCount integration test** - `44a4e025` (test)
3. **Task 3: e2e per-upstream acknowledge round-trip spec** - `7acd0f4a` (test)

## Files Created/Modified
- `e2e/api/lifecycle_test.go` - Two new `Describe` blocks (`no false-positive on unrelated edit`, `per-upstream acknowledge`) placed before the existing `Drift detection (all specs)` block; both fully resolve any drift they create so no residual is left for the blanket-zero assertion.
- `internal/storage/postgres/lifecycle_test.go` - New `CheckAllSpecs_MixedState_SkippedCount` subtest under `TestLifecycle` plus the `internal/drift` import (no import cycle from `package postgres_test`).

## Decisions Made
- Kept this strictly verification-only: no change to the drift engine, converters, the three-way scope SYNC tables, proto, or migrations (D-01/D-02/D-03).
- Chose the integration layer (isolated `clearDatabase` per subtest) for the full-graph SkippedCount proof to avoid the ordering collision with the e2e `Drift detection (all specs)` blanket-zero assertion (RESEARCH Open Question 1 resolution).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## Verification Results

Docker was **available** in this environment, so the real-DB proofs were executed (not just compiled):

- `go vet -tags e2e ./e2e/api/...` → pass (Tasks 1 & 3 compile).
- `go vet -tags integration ./internal/storage/postgres/...` → pass (Task 2 compiles, no import cycle).
- `go test ./internal/drift/... ./internal/driftscope/... ./cmd/specgraph/...` → pass (unit gate + three-way scope SYNC/completeness tests green; no scope table touched).
- `go test -tags integration -p 1 -run 'TestLifecycle/CheckAllSpecs_MixedState_SkippedCount' ./internal/storage/postgres/` → **ok** (2.09s).
- `go test -tags e2e -v ./e2e/api/... -ginkgo.focus="no false-positive on unrelated edit|per-upstream acknowledge"` → **SUCCESS: 5 Passed | 0 Failed** (the 2 no-false-positive specs + 3 per-upstream ack specs, real DB).
- `go test -tags e2e ./e2e/api/... -ginkgo.focus="Drift detection"` → **ok** (all drift blocks incl. the `(all specs)` blanket-zero assertion still green — no residual drift leaked).

## Next Phase Readiness
- DRFT-01 SC#2 real-DB proofs are complete and green. Ready for 04-02 (the `site/docs/concepts/drift.md` API/MCP access note, D-04).

## Self-Check: PASSED
- `e2e/api/lifecycle_test.go` and `internal/storage/postgres/lifecycle_test.go` modified on disk (verified via `git show`).
- Commits present: `3b00de73`, `44a4e025`, `7acd0f4a` (verified via `git log`).
- New test identifiers present: `no false-positive on unrelated edit`, `CheckAllSpecs_MixedState_SkippedCount`, `per-upstream acknowledge`.

---
*Phase: 04-verification-integration-reliability*
*Completed: 2026-07-10*
