---
phase: 07-authoring-lifecycle-semantics
plan: 04
subsystem: api
tags: [lifecycle, amend, supersede, authoring, connectrpc, proto, deletion, consolidation]

# Dependency graph
requires:
  - phase: 07-authoring-lifecycle-semantics
    provides: 07-03 rerouted the MCP author amend/supersede handlers onto LifecycleService, leaving the authoring-level Amend/Supersede path with zero callers
provides:
  - AuthoringService no longer exposes Amend/Supersede RPCs or their four request/response messages
  - AuthoringHandler.Amend/Supersede handlers deleted
  - Store.AmendSpec/SupersedeSpec deleted (TransitionStage preserved)
  - storage.AuthoringSpecLifecycle interface + AuthoringBackend embed and storage.AmendResult struct deleted
  - storage.ValidateAmendTransition deleted
  - Single source of truth for amend/supersede = the LifecycleService path
affects: [07-05, mcp-authoring, lifecycle]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Compile-safe two-commit deletion of a proto RPC: remove the rpc method lines first (shrinks both generated Handler and Client interfaces so surviving Go methods become legal extras), regenerate, then delete Go impls + orphaned messages and regenerate again"

key-files:
  created: []
  modified:
    - proto/specgraph/v1/authoring.proto
    - gen/specgraph/v1/authoring.pb.go
    - gen/specgraph/v1/specgraphv1connect/authoring.connect.go
    - web/src/lib/api/gen/specgraph/v1/authoring_pb.ts
    - internal/auth/actions.go
    - internal/server/authoring_handler.go
    - internal/server/authoring_handler_test.go
    - internal/server/test_scoper_test.go
    - internal/storage/postgres/authoring.go
    - internal/storage/postgres/authoring_test.go
    - internal/storage/authoring.go
    - internal/storage/stage_validation.go
    - internal/storage/stage_validation_test.go
    - internal/mcp/testhelpers_test.go

key-decisions:
  - "Deleted AuthoringSpecLifecycle entirely (rather than leaving it empty) once its only two methods were removed, and dropped its embed from AuthoringBackend — per RESEARCH A1/OQ1."
  - "Moved the 13 dead server handler tests (which call the removed generated client.Amend/.Supersede) into the Task 1 commit because the proto regen makes them non-compiling immediately; keeping each commit's tree fully compilable (build+vet+test) takes precedence over the plan's Task 1/Task 2 test-file split."
  - "Removed two AuthoringServiceAmend/SupersedeProcedure entries from the internal/auth scope map (internal/auth/actions.go) — an unlisted file the proto regen broke."
  - "Deleted TestTransitionStage_BackwardViaAmend and TestTransitionStage_SupersededGuard as collateral: both used the removed Store.AmendSpec/SupersedeSpec purely as setup and could not compile after deletion."

patterns-established:
  - "RPC-removal-first ordering is the only compile-safe way to retire a ConnectRPC method when the Go handler has no Unimplemented embed (the compile-time interface assertion must keep holding at every boundary)."

requirements-completed: [LIFE-01]

coverage:
  - id: D1
    description: "AuthoringService exposes no Amend/Supersede RPCs, messages, handlers, Store methods, AmendResult, or ValidateAmendTransition — the broken divergent path is fully retired (single source of truth)"
    requirement: "LIFE-01"
    verification:
      - kind: unit
        ref: "rg absence checks: no 'rpc Amend|rpc Supersede' in authoring.proto; no AmendRequest/SupersedeRequest in gen specgraphv1connect; no func (h *AuthoringHandler) Amend/Supersede; no func (s *Store) AmendSpec/SupersedeSpec; no AmendResult/AuthoringSpecLifecycle/ValidateAmendTransition"
        status: pass
      - kind: unit
        ref: "task check (fmt, license, lint, build, unit tests) — exit 0"
        status: pass
    human_judgment: false
  - id: D2
    description: "TransitionStage preserved and funnel + approve RPCs unaffected; the LifecycleService amend/supersede path (TransitionAmend/Supersede, LifecycleAmendSpec/SupersedeSpec) is untouched"
    requirement: "LIFE-01"
    verification:
      - kind: unit
        ref: "rg confirms func (s *Store) TransitionStage present in internal/storage/postgres/authoring.go"
        status: pass
      - kind: integration
        ref: "go test -tags integration ./internal/storage/postgres/... — ok (11.146s)"
        status: pass
    human_judgment: false

# Metrics
duration: 13min
completed: 2026-07-14
status: complete
---

# Phase 7 Plan 4: Retire Broken Authoring Amend/Supersede Path Summary

**Fully deleted the divergent authoring-level amend/supersede twin (D-02) — the `AuthoringService.Amend/Supersede` RPCs and four messages, the handlers, `Store.AmendSpec/SupersedeSpec`, the `AuthoringSpecLifecycle` interface + embed, `storage.AmendResult`, and `ValidateAmendTransition` — via a compile-safe two-commit RPC-removal-first ordering, leaving the LifecycleService path as the single source of truth and keeping `task check` green at every boundary.**

## Performance

- **Duration:** ~13 min
- **Started:** 2026-07-14T21:55:51Z
- **Completed:** 2026-07-14T22:08:29Z
- **Tasks:** 2
- **Files modified:** 14

## Accomplishments
- **Task 1 (RPC-removal-first):** Deleted the two `rpc Amend`/`rpc Supersede` method lines from `authoring.proto` and regenerated gen/ (Go + TS). Both generated interfaces (`AuthoringServiceHandler` and `AuthoringServiceClient`) dropped Amend/Supersede, so surviving Go handler/mock methods became legal extra methods and the `line-33` interface assertion still held — production build stayed green.
- **Task 2 (full deletion):** Removed all remaining Go pieces of the broken path plus the four now-orphaned proto messages, and regenerated a second time.
- Achieved the phase's core consolidation goal: no reachable divergent (approved-rejecting / any-state-supersede / land-at-target) semantics remain (threat T-07-07 mitigated).
- Verified `TransitionStage` and the entire LifecycleService amend/supersede path are untouched; funnel + approve RPCs unaffected.

## Task Commits

Each task was committed atomically:

1. **Task 1: Remove Amend/Supersede RPC method lines + regenerate** — `694c89d5` (refactor)
2. **Task 2: Delete all Go impls/handlers/storage/interface + orphaned messages, prune fakes/dead tests, regenerate** — `f644fc4c` (refactor)

**Plan metadata:** _(this SUMMARY commit)_ (docs)

## Files Created/Modified
- `proto/specgraph/v1/authoring.proto` — Removed 2 RPC methods (Task 1) and 4 request/response messages (Task 2).
- `gen/specgraph/v1/authoring.pb.go`, `.../specgraphv1connect/authoring.connect.go`, `web/src/lib/api/gen/.../authoring_pb.ts` — Regenerated by `task proto` (never hand-edited).
- `internal/auth/actions.go` — Dropped AuthoringServiceAmend/SupersedeProcedure scope-map entries (broke on regen; deviation).
- `internal/server/authoring_handler.go` — Deleted Amend/Supersede handler methods.
- `internal/server/authoring_handler_test.go` — Deleted 13 dead handler tests (Task 1) + pruned fakeAuthoringBackend/authoringTestBackend amend/supersede methods and fields (Task 2).
- `internal/server/test_scoper_test.go` — Removed stubBackend.SupersedeSpec/AmendSpec (kept Lifecycle* variants).
- `internal/storage/postgres/authoring.go` — Removed Store.AmendSpec/SupersedeSpec; preserved TransitionStage.
- `internal/storage/postgres/authoring_test.go` — Deleted TestSupersedeSpec_*/TestAmendSpec_* plus two collateral TestTransitionStage_* tests that used the removed methods for setup.
- `internal/storage/authoring.go` — Deleted AmendResult struct, AuthoringSpecLifecycle interface, and its AuthoringBackend embed.
- `internal/storage/stage_validation.go` / `_test.go` — Deleted ValidateAmendTransition and TestValidateAmendTransition.
- `internal/mcp/testhelpers_test.go` — Removed mockAuthoringService.Amend/Supersede methods + fn fields (kept mockLifecycleService.TransitionAmend/Supersede).

## Decisions Made
- **RPC-removal-first ordering** is the only compile-safe path because `AuthoringHandler` has no `Unimplemented` embed; removing the RPCs from the interface is what keeps the still-present handler methods legal as extras.
- **Deleted AuthoringSpecLifecycle outright** (not left empty) once both methods were gone (RESEARCH A1/OQ1).
- **Collateral TransitionStage tests deleted** — `TestTransitionStage_BackwardViaAmend`/`_SupersededGuard` depended on the removed authoring methods for state setup and could not compile.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Removed AuthoringServiceAmend/SupersedeProcedure from the auth scope map**
- **Found during:** Task 1 (proto regen)
- **Issue:** `internal/auth/actions.go` (not in the plan's file list) referenced `specgraphv1connect.AuthoringServiceAmendProcedure` and `...SupersedeProcedure`, which the regen removed → `go build ./...` failed.
- **Fix:** Deleted the two now-undefined scope-map entries; the funnel/approve/conversation authoring procedures remain scoped.
- **Files modified:** internal/auth/actions.go
- **Verification:** `go build ./...` and `go vet ./...` green after the fix; `task check` green.
- **Committed in:** 694c89d5 (Task 1 commit)

**2. [Rule 3 - Blocking] Moved 13 dead server handler tests into the Task 1 commit**
- **Found during:** Task 1 (proto regen)
- **Issue:** The plan deferred all server test deletions to Task 2, but `internal/server/authoring_handler_test.go` calls the *generated* `client.Amend`/`client.Supersede` (not a mock). Removing those methods from the generated `AuthoringServiceClient` interface made 13 test functions non-compiling immediately, breaking `go vet ./...` / `go test ./internal/server/...` at the Task 1 boundary (which the plan's own Task 1 acceptance criteria require to pass).
- **Fix:** Deleted the 13 `TestAuthoringHandler_Amend_*`/`_Supersede_*` functions in Task 1. The fake backend's still-required `AmendSpec`/`SupersedeSpec` methods (needed to satisfy the not-yet-removed `AuthoringSpecLifecycle` interface) were left until Task 2.
- **Files modified:** internal/server/authoring_handler_test.go
- **Verification:** Task 1 boundary: `go build`, `go vet`, `go test ./internal/server/... ./internal/mcp/...` all green.
- **Committed in:** 694c89d5 (Task 1 commit)

**3. [Rule 3 - Blocking] Deleted two collateral TransitionStage integration tests**
- **Found during:** Task 2 (postgres test pruning)
- **Issue:** `TestTransitionStage_BackwardViaAmend` and `TestTransitionStage_SupersededGuard` were not named in the plan but used the removed `Store.AmendSpec`/`SupersedeSpec` for state setup, so they could not compile after deletion.
- **Fix:** Deleted both alongside the planned TestAmendSpec_*/TestSupersedeSpec_* cases.
- **Files modified:** internal/storage/postgres/authoring_test.go
- **Verification:** `go build -tags integration ./...` and `go test -tags integration ./internal/storage/postgres/...` green.
- **Committed in:** f644fc4c (Task 2 commit)

---

**Total deviations:** 3 auto-fixed (all Rule 3 - blocking, all in test/scope-map files broken by the proto regen).
**Impact on plan:** No scope creep — every deviation was a compile-blocking callsite of a deliberately-deleted symbol, resolved by deletion so each commit's tree stays fully compilable (the AGENTS.md invariant). The production deletion set matches the plan exactly.

## Issues Encountered
- The plan's Task 1/Task 2 test-file split assumed only handler/mock *methods* (harmless extras) would be affected in Task 1; in practice the server tests call the generated client directly, so their deletion had to move to Task 1. Resolved by keeping "each commit compiles" as the governing invariant (see Deviation 2).

## Verification Results
- **Task 1:** `task proto` exit 0; `go build ./...` + `go vet ./...` green; `go test ./internal/server/... ./internal/mcp/...` green. Absence greps confirmed both generated interfaces dropped Amend/Supersede while the four messages remained.
- **Task 2:** `task proto` exit 0; `go build ./...` + `go vet ./...` green; `task check` exit 0. Integration: `go build -tags integration ./...` + `go vet -tags integration ./...` green; `go test -tags integration ./internal/storage/postgres/...` — ok (11.146s).
- Integration/e2e scan: the only remaining amend/supersede references are the Lifecycle path (`LifecycleAmendSpec/SupersedeSpec`, `TransitionAmend/Supersede`) — no integration-callsite trap.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The authoring amend/supersede path is fully retired; there is now a single source of truth (LifecycleService) for amend/supersede semantics. Plan 07-05 (MCP-only e2e for amend/supersede) is unblocked.

## Self-Check: PASSED
- All 14 modified files verified present on disk.
- Commits `694c89d5`, `f644fc4c` verified in git log.
- `task check` exits 0; postgres integration suite passes under Docker.
- Absence greps confirm no residual Amend/Supersede RPCs, messages, handlers, Store methods, AmendResult, AuthoringSpecLifecycle, or ValidateAmendTransition.

---
*Phase: 07-authoring-lifecycle-semantics*
*Completed: 2026-07-14*
