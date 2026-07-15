---
phase: 08-authoring-conversation-fidelity
plan: 04
subsystem: testing
tags: [e2e, ginkgo, mcp, authoring, conversation-fidelity, postgres]

# Dependency graph
requires:
  - phase: 08-01
    provides: server-side accept-path conversation enforcement (approve requires exchanges)
  - phase: 08-02
    provides: MCP author-tool exchange threading on approve + client-side empty-exchanges guard; standalone conversation record action removed
  - phase: 08-03
    provides: CLI --conversation flag parity
provides:
  - MCP-only D-10 phase gate proving conversation recording is protocol-enforced end-to-end
  - Positive full-funnel fidelity assertion (non-empty retrievable conversation at shape/specify/decompose/approve)
  - Negative backstop (approve without exchanges is rejected — a missing conversation cannot silently pass)
affects: [conversation-fidelity, authoring-funnel, mcp-only-agents, CONV-01]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Per-Describe isolated project slug + dedicated mcpProjectClient harness to prevent cross-test state bleed"
    - "Stage-string discipline: exchange-level stage \"approve\" (ValidateExchanges) vs stored/queried conversation stage \"approved\" (SpecStageApproved)"
    - "Negative assertion on res.IsError + corroborating error text (client guard OR server InvalidArgument) — not tied to a specific Connect code"

key-files:
  created:
    - e2e/api/mcp_only_conversation_test.go
  modified:
    - e2e/api/mcp_only_authoring_test.go

key-decisions:
  - "Approve-gate conversation list filters on the STORED stage \"approved\", not the exchange stage \"approve\" (ListConversations does an exact SQL match)"
  - "Negative rejection asserted via res.IsError + error text (exchanges/InvalidArgument), satisfying the backstop whether the client guard or the server round-trip fires (review R2 #6)"

patterns-established:
  - "MCP-only conversation-fidelity e2e as the CONV-01 D-10 phase gate that would have caught #906"

requirements-completed: [CONV-01]

coverage:
  - id: D1
    description: "MCP-only full-funnel e2e records + retrieves a non-empty conversation at every required stage (shape/specify/decompose/approve)"
    requirement: CONV-01
    verification:
      - kind: e2e
        ref: "e2e/api/mcp_only_conversation_test.go#records a non-empty retrievable conversation at every required stage"
        status: unknown
    human_judgment: true
    rationale: "Requires Docker (testcontainers pgvector) — unavailable in this local execution environment. Test compiles cleanly under go vet -tags e2e; runtime execution deferred to CI (task pr-prep). Verifier must confirm green in CI."
  - id: D2
    description: "Negative e2e proves omitting exchanges on the approve stage is rejected — a missing conversation cannot silently pass"
    requirement: CONV-01
    verification:
      - kind: e2e
        ref: "e2e/api/mcp_only_conversation_test.go#rejects approve without exchanges"
        status: unknown
    human_judgment: true
    rationale: "Docker-gated e2e; compile-verified only in this environment. Runtime green deferred to CI."
  - id: D3
    description: "Existing MCP-only funnel test updated so approve supplies exchanges and asserts non-empty per-stage conversations under the new enforcement"
    requirement: CONV-01
    verification:
      - kind: e2e
        ref: "e2e/api/mcp_only_authoring_test.go#authors the constitution and walks a spec spark→approved via MCP only"
        status: unknown
    human_judgment: true
    rationale: "Docker-gated e2e; compile-verified via go vet -tags e2e. Runtime green deferred to CI."

# Metrics
duration: 12min
completed: 2026-07-15
status: complete
---

# Phase 08 Plan 04: MCP-Only Conversation-Fidelity Phase Gate Summary

**MCP-only Ginkgo e2e (D-10) drives the full authoring funnel via the `author` tool and proves every required stage records a non-empty, retrievable conversation — plus a negative backstop that omitting exchanges on approve is rejected (CONV-01).**

## Performance

- **Duration:** 12 min
- **Started:** 2026-07-15T17:15:00Z
- **Completed:** 2026-07-15T17:27:00Z
- **Tasks:** 2
- **Files modified:** 2 (1 created, 1 modified)

## Accomplishments

- Updated the existing MCP-only funnel test so `author action=approve` now supplies exchanges (required under the Plan 01 server enforcement + Plan 02 MCP threading), replacing the stale "approve needs no exchanges" comment (review R2 #6).
- Added per-stage `conversation action=list` assertions to the existing funnel test — each required stage (shape/specify/decompose/approve) has a non-empty conversation whose exchange content round-trips.
- Created a dedicated MCP-only conversation-fidelity e2e (`mcp_only_conversation_test.go`) with an isolated project slug: a positive full-funnel spec and a negative approve-without-exchanges spec — the D-10 backstop that a missing conversation cannot silently pass (#906 regression guard).
- Encoded stage-string discipline (review finding #2): approve-gate conversation lists filter on the STORED stage `"approved"`, not the exchange-level `"approve"`, since `ListConversations` matches the stage string exactly.

## Task Commits

Each task was committed atomically:

1. **Task 1: Update existing MCP-only funnel test — approve supplies exchanges + per-stage conversation assertions** - `389c43f9` (test)
2. **Task 2: New MCP-only conversation-fidelity e2e (positive + negative)** - `cd68fbee` (test)

## Files Created/Modified

- `e2e/api/mcp_only_conversation_test.go` (created) - Dedicated MCP-only conversation-fidelity spec: isolated project + client harness, positive full-funnel fidelity assertion at all four required stages, negative approve-without-exchanges rejection; `toolErrorText` helper for inspecting deliberately-failing tool results.
- `e2e/api/mcp_only_authoring_test.go` (modified) - Added `mcpOnlyApproveExchanges` fixture; approve call now supplies exchanges; updated stale comment (R2 #6); added per-stage `conversation list` fidelity assertions with approve filtered on stored stage `"approved"`.

## Decisions Made

- **Approve-gate list filters on `"approved"` (stored), not `"approve"` (exchange stage).** The accept path stores the conversation under `storage.SpecStageApproved` = `"approved"` (spec_domain.go:19) and `ListConversations` does an exact SQL match; filtering on `"approve"` would return an empty list and a false-negative fidelity assertion (review finding #2). Documented the deliberate divergence in test comments.
- **Negative rejection asserted on `res.IsError` + error text, not a specific Connect code.** After 08-02's client-side empty-exchanges guard, the MCP `author` tool may error before the server `InvalidArgument` round-trip; asserting on `res.IsError` with a corroborating `exchanges`/`InvalidArgument` text check satisfies the backstop whether the client guard or the server rejection fires (review R2 #6).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- **Docker unavailable in the local execution environment.** The e2e tier uses testcontainers (`pgvector/pgvector:pg18`) and cannot run locally without Docker. Per the project's e2e guidance, both files were compile-verified with `go vet -tags e2e ./e2e/api/...` (exit 0) and the full `task check` gate passes (fmt, license, lint, build, unit tests). Runtime execution of the two new/updated e2e specs is deferred to CI (`task pr-prep`). This is the expected path for Docker-gated e2e work in this environment, not a code defect.

## Next Phase Readiness

- Wave 2 phase gate (D-10) landed: the MCP-only conversation-fidelity backstop is in place and compiles cleanly. Phase 08 (all four plans) is ready for `task pr-prep` runtime verification in CI and phase verification.
- No blockers.

---
*Phase: 08-authoring-conversation-fidelity*
*Completed: 2026-07-15*

## Self-Check: PASSED
- `e2e/api/mcp_only_conversation_test.go` exists on disk ✓
- `e2e/api/mcp_only_authoring_test.go` modified ✓
- Commit `389c43f9` present (Task 1) ✓
- Commit `cd68fbee` present (Task 2) ✓
- `go vet -tags e2e ./e2e/api/...` exit 0 ✓
- `task check` exit 0 ✓
- `task license:check` exit 0 ✓
