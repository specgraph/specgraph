---
phase: 08-authoring-conversation-fidelity
plan: 03
subsystem: cli
tags: [cobra, conversation-exchanges, authoring, connectrpc, go]

# Dependency graph
requires:
  - phase: 08-authoring-conversation-fidelity
    provides: "Server Approve enforcement (08-01) + MCP handleApprove exchange threading (08-02) — the enforcement authority this CLI supplies real input to"
provides:
  - "Shared loadConversationFlag loader (bare JSON array, - for stdin) for CLI stage commands"
  - "Required --conversation on shape/specify/decompose/approve; optional on spark"
  - "Deletion of the cliSyntheticExchanges placeholder escape hatch (D-04)"
affects: [authoring, conversation-fidelity, gastown-execution]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Shared registerConversationFlag helper with cobra.MarkFlagRequired enforcement mirroring conversation.go:48-49"
    - "CLI-only bare-array loader via loadJSONFileRaw (never reused in server/network paths per util.go security note)"

key-files:
  created:
    - cmd/specgraph/conversation_flag.go
  modified:
    - cmd/specgraph/shape.go
    - cmd/specgraph/specify.go
    - cmd/specgraph/decompose.go
    - cmd/specgraph/approve.go
    - cmd/specgraph/spark.go
    - cmd/specgraph/authoring_test.go

key-decisions:
  - "--conversation accepts a BARE JSON array of ConversationExchange objects (A2/D-05), matching the MCP author tool shape — one validation contract; NOT the {\"exchanges\":[...]} object shape"
  - "Per-command flag globals + shared loader/registration helpers, keeping one registration idiom across all five commands"
  - "spark's --conversation stays optional; exchanges set only when provided (D-01)"

patterns-established:
  - "registerConversationFlag(cmd, target, required): StringVar + conditional cobra.CheckErr(MarkFlagRequired) — required-flag enforcement needs the explicit call, a bool param alone is inert (review R2 #4)"
  - "loadConversationFlag(path): '-' reads stdin, else CLI-only loadJSONFileRaw into a bare []conversationExchangeInput, then maps to []*specv1.ConversationExchange"

requirements-completed: [CONV-01]

# Coverage metadata
coverage:
  - id: D1
    description: "shape/specify/decompose/approve require a real --conversation input and error before RPC dispatch if it is missing"
    requirement: "CONV-01"
    verification:
      - kind: unit
        ref: "cmd/specgraph/authoring_test.go#TestConversationFlag_RequiredBeforeDispatch"
        status: pass
    human_judgment: false
  - id: D2
    description: "CLI no longer stamps a synthetic placeholder conversation; cliSyntheticExchanges deleted"
    requirement: "CONV-01"
    verification:
      - kind: unit
        ref: "test ! -f cmd/specgraph/authoring_cli_exchanges.go && go build ./..."
        status: pass
    human_judgment: false
  - id: D3
    description: "--conversation accepts a bare JSON array and '-' reads from stdin; object shape rejected"
    requirement: "CONV-01"
    verification:
      - kind: unit
        ref: "cmd/specgraph/authoring_test.go#TestLoadConversationFlag_ValidArray,TestLoadConversationFlag_Stdin,TestLoadConversationFlag_RejectsObjectShape"
        status: pass
    human_judgment: false
  - id: D4
    description: "spark's --conversation is optional; exchanges set only when provided"
    requirement: "CONV-01"
    verification:
      - kind: unit
        ref: "cmd/specgraph/authoring_test.go#TestRunSpark_HappyPath"
        status: pass
    human_judgment: false
  - id: D5
    description: "CLI approve threads the loaded --conversation exchanges into ApproveRequest (wave-1 hard-merge third leg with 08-01/08-02)"
    requirement: "CONV-01"
    verification:
      - kind: unit
        ref: "cmd/specgraph/authoring_test.go#TestRunApprove_HappyPath"
        status: pass
    human_judgment: false

# Metrics
duration: 18min
completed: 2026-07-15
status: complete
---

# Phase 08 Plan 03: CLI Conversation-Input Fidelity Summary

**Replaced the CLI synthetic-placeholder escape hatch with a shared `loadConversationFlag` loader — shape/specify/decompose/approve now require a real `--conversation` bare-JSON-array input (`-` for stdin), matching the MCP `author` tool's single validation contract.**

## Performance

- **Duration:** ~18 min
- **Tasks:** 2
- **Files modified:** 8 (1 created, 1 deleted, 6 modified)

## Accomplishments
- Added `cmd/specgraph/conversation_flag.go` with `loadConversationFlag` (bare JSON array, stdin via `-`) and `registerConversationFlag` (required-flag enforcement).
- Rewired all five stage commands: shape/specify/decompose/approve require `--conversation`; spark keeps it optional (D-01).
- Deleted `cliSyntheticExchanges` — the CLI no longer stamps a fake conversation to pass the server gate (D-04).
- Proved the array-only contract, stdin path, object-shape rejection, and pre-dispatch required-flag validation with unit tests; wave-1 `task check` green.

## Task Commits

1. **Task 1: Add loadConversationFlag; rewire all five stage commands; delete synthetic helper** - `3f39d073` (feat)
2. **Task 2: CLI unit tests — loader shape, stdin, object-shape rejection, missing-flag error** - `c509801d` (test)

## Files Created/Modified
- `cmd/specgraph/conversation_flag.go` - Shared loader + flag-registration helpers; CLI-only bare-array contract.
- `cmd/specgraph/authoring_cli_exchanges.go` - **Deleted** (`cliSyntheticExchanges` placeholder removed).
- `cmd/specgraph/shape.go` / `specify.go` / `decompose.go` - Required `--conversation`, loaded exchanges replace the synthetic call.
- `cmd/specgraph/approve.go` - Added required `--conversation`; sets `ConversationExchanges` on `ApproveRequest`.
- `cmd/specgraph/spark.go` - Added optional `--conversation`; exchanges set only when provided.
- `cmd/specgraph/authoring_test.go` - Loader/stdin/object-rejection/required-flag tests; happy-path tests supply a conversation file; approve asserts exchanges threading.

## Decisions Made
- `--conversation` takes a **bare JSON array** of `ConversationExchange` objects (A2/D-05), identical to the MCP `author` tool shape — one validation contract, distinct from the `conversation record` `{"exchanges":[...]}` object shape (flag help calls this out explicitly, review finding #3).
- Required-flag enforcement uses an explicit `cobra.CheckErr(cmd.MarkFlagRequired("conversation"))` (review R2 #4) — a `required bool` param alone is inert.
- Per-command flag globals with shared helpers, keeping a single registration idiom.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Wave-1 hard merge gate complete: 08-01 (server enforcement), 08-02 (MCP), and 08-03 (CLI) all land together with `task check` green.
- CLI now supplies real exchanges on all four required stages; no synthetic placeholder remains.

---
*Phase: 08-authoring-conversation-fidelity*
*Completed: 2026-07-15*

## Self-Check: PASSED
- FOUND: cmd/specgraph/conversation_flag.go, shape.go, specify.go, decompose.go, approve.go, spark.go, authoring_test.go
- CONFIRMED DELETED: cmd/specgraph/authoring_cli_exchanges.go
- FOUND commit: 3f39d073 (feat 08-03)
- FOUND commit: c509801d (test 08-03)
- `task check` exit 0 (Wave-1 hard merge gate green)
