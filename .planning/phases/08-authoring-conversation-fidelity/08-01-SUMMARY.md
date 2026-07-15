---
phase: 08-authoring-conversation-fidelity
plan: 01
subsystem: authoring
tags: [connectrpc, conversation-recording, protobuf, postgres, transactions, adr-004]

# Dependency graph
requires:
  - phase: 07-authoring-lifecycle-semantics
    provides: authoring stage-transition + conversation-recording protocol (shape/specify/decompose enforced)
provides:
  - Approve ACCEPT branch enforces non-empty conversation_exchanges (hard reject, D-03)
  - Approve ACCEPT records a conversation under stage "approved" atomically with the stage transition (D-02, T-08-04)
  - ApproveRequest.conversation_exchanges (field 3) documented as REQUIRED for both accept and reject dispositions
affects: [08-02-mcp-approve, 08-03-cli-approve, authoring-funnel, conversation-recording]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "validate-before-tx then record-op-into-existing-runInTxOrSequential block (mirrors REJECT branch)"
    - "conversation recorded under the approval gate stage (SpecStageApproved) not the current decompose stage"

key-files:
  created: []
  modified:
    - proto/specgraph/v1/authoring.proto
    - gen/specgraph/v1/authoring.pb.go
    - web/src/lib/api/gen/specgraph/v1/authoring_pb.ts
    - internal/server/authoring_handler.go
    - internal/server/authoring_handler_test.go
    - internal/storage/postgres/conversation_test.go

key-decisions:
  - "Accept records under storage.SpecStageApproved (value \"approved\"); exchange-level stage string validates against \"approve\" (A1)"
  - "RecordConversation op placed as the final op in the accept runInTxOrSequential block (after GetSpec) to preserve the GetSpecError test semantics; ordering is atomicity-neutral since all ops share the tx"
  - "Field 3 comment reworded only — no new proto field (Pitfall #1); requirement covers APPROVE_ACTION_UNSPECIFIED default-accept too (review R2 #5)"

patterns-established:
  - "Approve accept path is now the fourth enforced conversation-recording stage, symmetric with shape/specify/decompose/reject"

requirements-completed: [CONV-01]

coverage:
  - id: D1
    description: "Approve ACCEPT/UNSPECIFIED hard-rejects empty conversation_exchanges with connect.CodeInvalidArgument"
    requirement: "CONV-01"
    verification:
      - kind: unit
        ref: "internal/server/authoring_handler_test.go#TestAuthoringHandler_Approve_AcceptRequiresExchanges"
        status: pass
    human_judgment: false
  - id: D2
    description: "A successful Approve ACCEPT records one conversation under stage \"approved\""
    requirement: "CONV-01"
    verification:
      - kind: unit
        ref: "internal/server/authoring_handler_test.go#TestAuthoringHandler_Approve_AcceptRecordsConversation"
        status: pass
    human_judgment: false
  - id: D3
    description: "Approve ACCEPT records the conversation atomically with the stage transition — RecordConversation failure rolls back the approval"
    requirement: "CONV-01"
    verification:
      - kind: unit
        ref: "internal/server/authoring_handler_test.go#TestAuthoringHandler_Approve_AcceptRecordConversationFailureRollsBack"
        status: pass
    human_judgment: false
  - id: D4
    description: "An approved-stage conversation is persisted and retrievable via ListConversations filtered on the stored value \"approved\""
    requirement: "CONV-01"
    verification:
      - kind: integration
        ref: "internal/storage/postgres/conversation_test.go#TestListConversations_ApprovedStageRetrievable"
        status: unknown
    human_judgment: true
    rationale: "Integration tier requires Docker (testcontainers pgvector:pg18), which was unavailable in the execution environment. The test binary compiles cleanly (go vet + test-binary build pass; failure is solely TestMain container startup). CI (task pr-prep) runs it under the Wave-1 merge gate — a human/CI must confirm the green run."
  - id: D5
    description: "ApproveRequest.conversation_exchanges (field 3) documented as REQUIRED for both approve and reject; gen/ regenerated with no new field"
    requirement: "CONV-01"
    verification:
      - kind: other
        ref: "task proto:check (exit 0, no staleness)"
        status: pass
    human_judgment: false

# Metrics
duration: 14min
completed: 2026-07-15
status: complete
---

# Phase 08 Plan 01: Approve-Accept Conversation Fidelity Summary

**Approve ACCEPT now validates non-empty conversation_exchanges (hard reject) and records the conversation under stage "approved" atomically inside the approval transaction — symmetric with the already-shipped REJECT branch (CONV-01, D-02/D-03).**

## Performance

- **Duration:** ~14 min
- **Started:** 2026-07-15T16:34Z (approx)
- **Completed:** 2026-07-15T16:47Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments
- Closed the approve-accept conversation hole: accept enforces exchanges and records atomically (the fourth enforced recording stage after shape/specify/decompose/reject).
- Reworded `ApproveRequest.conversation_exchanges` (field 3) to state it is REQUIRED for both accept (including default `APPROVE_ACTION_UNSPECIFIED`) and reject; regenerated committed `gen/` (Go + web TS) with no new field.
- Added unit proof of enforcement + atomic rollback (via `newTxConvAuthoringClient`) and a storage integration proof of approved-stage retrieval filtered on `"approved"`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Reword proto comment + regenerate gen** - `09c461f9` (docs)
2. **Task 2: Enforce + record exchanges in Approve ACCEPT branch (+ fix existing accept tests)** - `b95d2c12` (feat)
3. **Task 3: Tests — enforcement, atomic rollback, retrieval** - `c949c6e5` (test)

**Plan metadata:** (this commit) (docs: complete plan)

## Files Created/Modified
- `proto/specgraph/v1/authoring.proto` - Reworded field 3 doc comment (REQUIRED for accept AND reject; unchanged number/name).
- `gen/specgraph/v1/authoring.pb.go` - Regenerated descriptor rawDesc reflecting the new comment.
- `web/src/lib/api/gen/specgraph/v1/authoring_pb.ts` - Regenerated web TS descriptor (same comment).
- `internal/server/authoring_handler.go` - Approve ACCEPT branch calls `authoring.ValidateExchanges(..., "approve")` before the tx and adds a `RecordConversation` op (Stage `SpecStageApproved`) into the existing accept `runInTxOrSequential` block.
- `internal/server/authoring_handler_test.go` - Updated existing accept-path tests to supply valid exchanges + a recording backend; added `AcceptRequiresExchanges`, `AcceptRecordsConversation`, `AcceptRecordConversationFailureRollsBack`; extended `fakeFullBackend`/`fullAuthoringTestBackend` with conversation support.
- `internal/storage/postgres/conversation_test.go` - Added `TestListConversations_ApprovedStageRetrievable` (integration tier).

## Decisions Made
- Record under `storage.SpecStageApproved` (value `"approved"`); the exchange-level stage string validates against `"approve"` (A1 pinned) — the conversation is associated with the approval gate, not the current decompose stage.
- Placed the `RecordConversation` op as the final op (after `GetSpec`) in the accept `runInTxOrSequential` block. Ordering is atomicity-neutral (all ops share the tx); placing it last preserved the `GetSpecError` test's semantics (GetSpec failure surfaces before RecordConversation) with a minimal test change.
- Field 3 comment reworded only — treated D-02 as wiring, not a new field (RESEARCH Pitfall #1).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated four additional accept-path tests broken by the new enforcement**
- **Found during:** Task 2 (running `go test ./internal/server/...`)
- **Issue:** Beyond the three accept tests named in the plan (`_HappyPath`, `_AcceptUnchangedWithoutAction`, `_ExplicitAcceptSucceeds`), the enforcement also broke five more accept-path tests that call `Approve` without exchanges: `TestAuthoringHandler_Approve_GetSpecError`, and the four `TestAuthoringHandler_Approve_AcceptLinkedDecisions_*` tests (`_EdgeListError`, `_HappyPath`, `_SpecToDecisionDirection`, `_UpdateError`). They failed with `CodeInvalidArgument` at the new validation gate instead of exercising their intended downstream paths.
- **Fix:** Added valid approve `ConversationExchanges` to each. Because the two success-expecting `AcceptLinkedDecisions` tests use `fakeFullBackend` (which did not implement `RecordConversation`), extended `fakeFullBackend` with a lazily-initialized `conv *fakeConversationBackend` and added `RecordConversation`/`ListConversations` methods to `fullAuthoringTestBackend`.
- **Files modified:** internal/server/authoring_handler_test.go
- **Verification:** `go test ./internal/server/...` green; full-repo `go test ./...` green.
- **Committed in:** b95d2c12 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug — test breakage directly caused by the task's enforcement change).
**Impact on plan:** In-scope collateral of the enforcement (same class as the plan-named "fix the 3 existing accept tests" work). No production-code scope creep; validator caps and reject branch untouched.

## Issues Encountered
- **Storage integration tier not executed locally — Docker unavailable.** `TestListConversations_ApprovedStageRetrievable` compiles cleanly (`go vet -tags integration` passes; the test binary builds and only fails in `TestMain`'s testcontainers Postgres startup). It must be run under Docker via `task pr-prep` / CI as part of the Wave-1 hard merge gate. Recorded as `human_judgment: true` (coverage D4).

## Known Stubs
None — no placeholder/empty-return stubs introduced.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- **Wave-1 hard merge gate (review finding #6):** This plan makes the approve ACCEPT path REQUIRE exchanges. 08-01 MUST merge together with 08-02 (MCP `handleApprove`) and 08-03 (CLI `approve`) — otherwise those clients still send zero exchanges and get `InvalidArgument`. Run `task check` after all three land; run `task pr-prep` (Docker) to exercise the storage integration tier.
- Ready for 08-02.

---
*Phase: 08-authoring-conversation-fidelity*
*Completed: 2026-07-15*

## Self-Check: PASSED
- Files verified on disk: 08-01-SUMMARY.md, authoring_handler.go, conversation_test.go — all FOUND.
- Commits verified in git: 09c461f9 (docs/proto), b95d2c12 (feat), c949c6e5 (test), 4969aeaf (docs/metadata) — all FOUND.
- Unit tier `go test ./...` green (exit 0). Integration tier compiles; deferred to CI/Docker (see Issues Encountered).
