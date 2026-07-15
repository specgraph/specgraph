---
phase: 08-authoring-conversation-fidelity
plan: 02
subsystem: api
tags: [mcp, authoring, conversation, protojson, connectrpc, go]

# Dependency graph
requires:
  - phase: 08-authoring-conversation-fidelity
    provides: server-side Approve ACCEPT branch requires/records conversation_exchanges (08-01)
provides:
  - MCP author approve threads required conversation exchanges into ApproveRequest (D-02)
  - MCP author spark forwards optional exchanges into SparkRequest when provided (D-01, no longer drops-when-provided)
  - conversation MCP tool collapsed to list-only; standalone record action removed (D-06)
  - author tool Description + specgraph-authoring SKILL.md teach approve-requires-exchanges (D-09)
  - mockAuthoringService.Approve callback extended to receive full *ApproveRequest for threading assertions (review R2 #2)
affects: [08-03 CLI conversation surface, gastown execution consuming authoring exchanges]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "handleApprove mirrors handleShape exchange threading but with a client-side required-guard"
    - "handleSpark conditional-set (len>0) mirrors handleShape MINUS the required-guard (optional stays optional)"
    - "conversation tool single-action switch models findingsTool.handle (list-only + unknown-action default)"

key-files:
  created: []
  modified:
    - internal/mcp/tools_authoring.go
    - internal/mcp/tools_authoring_test.go
    - internal/mcp/testhelpers_test.go
    - internal/mcp/skills/embedded/specgraph-authoring/SKILL.md

key-decisions:
  - "Approve rejects empty exchanges client-side with a friendlier message than the server's validation error; spark tolerates absent exchanges (optional)"
  - "handleSpark sets ConversationExchanges only when len>0, leaving an absent slice absent (D-01 parity, not a hard reject)"

patterns-established:
  - "MCP inline-with-save is the only agent recording path — no skippable standalone record action (#906 root-cause removal)"

requirements-completed: [CONV-01]

coverage:
  - id: D1
    description: "MCP author approve threads required exchanges into ApproveRequest and rejects empty exchanges client-side"
    requirement: CONV-01
    verification:
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestAuthorTool_Approve"
        status: pass
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestAuthorTool_Approve_MissingExchanges"
        status: pass
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestAuthorTool_Approve_MalformedExchanges"
        status: pass
    human_judgment: false
  - id: D2
    description: "MCP author spark forwards optional exchanges when provided, stays optional when absent"
    requirement: CONV-01
    verification:
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestAuthorTool_Spark_ForwardsExchanges"
        status: pass
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestAuthorTool_Spark_NoExchangesStillSucceeds"
        status: pass
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestAuthorTool_Spark_MalformedExchanges"
        status: pass
    human_judgment: false
  - id: D3
    description: "conversation MCP tool exposes only list; record action removed and returns unknown-action"
    requirement: CONV-01
    verification:
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestConversationTool_Record_RemovedReturnsUnknownAction"
        status: pass
      - kind: unit
        ref: "internal/mcp/tools_authoring_test.go#TestConversationTool_List"
        status: pass
    human_judgment: false
  - id: D4
    description: "author Description + specgraph-authoring SKILL.md teach approve requires exchanges"
    requirement: CONV-01
    verification:
      - kind: other
        ref: "task skills:validate"
        status: pass
      - kind: unit
        ref: "internal/authoring#TestContentProtoDrift"
        status: pass
    human_judgment: false

# Metrics
duration: 12min
completed: 2026-07-15
status: complete
---

# Phase 8 Plan 2: MCP Conversation-Recording Surface Collapse Summary

**MCP `author approve` now threads required conversation exchanges into `ApproveRequest`, `spark` forwards optional exchanges instead of dropping them, and the `conversation` tool collapses to list-only — closing the skippable standalone-record path (#906).**

## Performance

- **Duration:** 12 min
- **Started:** 2026-07-15T16:47Z
- **Completed:** 2026-07-15T16:59Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments
- `handleApprove` parses `exchanges` via `parseOptionalExchanges`, rejects empty client-side ("exchanges is required for approve"), and sets `ConversationExchanges` on `ApproveRequest` (D-02, mirrors `handleShape`).
- `handleSpark` forwards agent-supplied exchanges when non-empty (no longer drops-when-provided), keeping spark exchanges OPTIONAL — parity with the server (D-01, review R2 #3).
- `conversation` tool reduced to a single `list` action: `record` branch and `handleRecord` deleted; `record` now returns an unknown-action error; retrieval (`list`) intact (D-06).
- `author` tool Description (top-level sentence + `exchanges` prop) and `specgraph-authoring` SKILL.md flipped to teach approve-requires-exchanges (D-09).
- `mockAuthoringService.Approve` callback extended to receive the full `*specv1.ApproveRequest`, so `TestAuthorTool_Approve` asserts threaded exchanges rather than just the slug (review R2 #2).

## Task Commits

1. **Task 1 (RED): failing tests for approve/spark exchange threading** - `86180077` (test)
2. **Task 1 (GREEN): thread exchanges into approve/spark + Description flip** - `4a16443d` (feat)
3. **Task 2: remove conversation record action, keep list only** - `5c677120` (feat)
4. **Task 3: teach approve-requires-exchanges in SKILL.md** - `857723d1` (docs)

_Task 1 followed the TDD RED→GREEN cycle (no separate refactor commit needed)._

## Files Created/Modified
- `internal/mcp/tools_authoring.go` - `handleApprove` requires+threads exchanges; `handleSpark` conditionally forwards exchanges; `authorTool.def()` Description flip; `conversationTool` record action + `handleRecord` removed (list-only).
- `internal/mcp/tools_authoring_test.go` - approve threading/missing/malformed tests; spark forward/optional/malformed tests; record-action tests replaced with an unknown-action assertion.
- `internal/mcp/testhelpers_test.go` - `mockAuthoringService.approve` callback signature widened to `func(*specv1.ApproveRequest)`.
- `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md` - exchanges-requirement lines and the Approve block flipped to require exchanges inline on clean acceptance.

## Decisions Made
- Approve rejects empty exchanges client-side with a friendlier message than the server would; spark tolerates an absent slice (optional), setting `ConversationExchanges` only when `len>0`.
- Kept `Store.RecordConversation` and the CLI `conversation record` command out of scope (D-07/D-08) — only the agent-facing MCP record action was removed.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Wave-1 hard merge gate (review finding #6): this MCP `handleApprove` change must land together with 08-01 (server enforcement) and 08-03 (CLI). 08-01 is already committed on this branch; `task check` passes across the combined change.
- No blockers. `go build ./...`, `go test ./internal/mcp/...`, `task skills:validate`, `TestContentProtoDrift`, and full `task check` all green.

---
*Phase: 08-authoring-conversation-fidelity*
*Completed: 2026-07-15*

## Self-Check: PASSED

- All modified files present on disk (tools_authoring.go, SKILL.md, SUMMARY.md).
- All four task commits present in git history (86180077, 4a16443d, 5c677120, 857723d1).
- `go build ./...`, `go test ./internal/mcp/...`, `task skills:validate`, `TestContentProtoDrift`, and full `task check` all green.
