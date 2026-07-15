---
status: complete
phase: 08-authoring-conversation-fidelity
source: [08-VERIFICATION.md]
started: 2026-07-15T17:25:33Z
updated: 2026-07-15T22:05:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Storage integration — `TestListConversations_ApprovedStageRetrievable` passes under Docker
expected: Run `task pr-prep` (Docker). The storage integration test `TestListConversations_ApprovedStageRetrievable` (internal/storage/postgres/conversation_test.go) passes green — a conversation recorded under storage.SpecStageApproved is retrievable via ListConversations filtered on the stored value "approved" (SC4 runtime retrieval + approve-gate stage-string discipline).
result: pass
source: docker-run (go test -tags integration ./internal/storage/postgres/... -run TestListConversations_ApprovedStageRetrievable → PASS, 2.162s, pgvector:pg18)

### 2. MCP-only conversation-fidelity e2e passes under Docker
expected: Run the e2e tier (`go test -tags e2e ./e2e/api/...` under Docker, via `task pr-prep`). Positive full-funnel spec (`records a non-empty retrievable conversation at every required stage`) passes — every required stage (shape/specify/decompose/approve) has a non-empty retrievable conversation (approve filtered on "approved"). Negative spec (`rejects approve without exchanges`) passes — approve with no exchanges returns res.IsError (client guard or server InvalidArgument); a missing conversation cannot silently pass. The D-10 regression backstop for #906.
result: pass
source: docker-run (Phase-8 MCP-only specs isolated: --ginkgo.focus="MCP-only conversation fidelity" → ok, 8.267s; positive + negative + mcp_only_authoring funnel all green)

## Summary

total: 2
passed: 2
issues: 1
pending: 0
skipped: 0
blocked: 0

## Gaps

- truth: "Full e2e tier (`task pr-prep` / `go test -tags e2e ./e2e/api/...`) is green after Phase 8"
  status: failed
  reason: "Cross-phase regression: running the full e2e suite yields 10 failures. All are pre-existing specs that advance a spec to 'approved'. Phase 8 made conversation_exchanges REQUIRED on Approve, but these older tests call Approve with only a Slug (no exchanges), so they now hard-reject with InvalidArgument. The Phase-8 executors ran only the unit tier (Docker-gated e2e was never run), so the regression slipped through. The Phase-8-specific fidelity specs themselves pass."
  severity: major
  test: 2
  root_cause: "Approve now enforces non-empty conversation_exchanges (internal/server/authoring_handler.go:478). Pre-existing e2e Approve call sites do not supply exchanges and were not updated by Phase 8."
  artifacts:
    - path: "e2e/api/helpers_test.go"
      issue: "line 169 — shared helper `ac.Approve(ctx, {Slug: slug})` (drives most pipeline/claim/graph/lifecycle/errors/skills specs) omits conversation_exchanges"
    - path: "e2e/api/authoring_test.go"
      issue: "lines 142, 256 — Approve without exchanges"
    - path: "e2e/api/lifecycle_pipeline_test.go"
      issue: "line 140 — Approve without exchanges"
    - path: "e2e/api/pipeline_test.go"
      issue: "line 190 — Approve without exchanges"
  missing:
    - "Update the 5 e2e Approve call sites to supply a minimal valid ConversationExchanges (matching the fixture shape used by the Phase-8 mcp_only_* specs), then re-run `go test -tags e2e ./e2e/api/...` to confirm 0 failures"
  debug_session: ""
  failing_specs:
    - "Full pipeline — approves the spec (pipeline_test.go:193)"
    - "graph queries [BeforeAll] shows dependencies (graph_test.go:64)"
    - "Claim protocol — advances to approved (claim_test.go:49)"
    - "Authoring funnel — steel thread — approves (authoring_test.go:259)"
    - "Authoring funnel — approves a decomposed spec (authoring_test.go:145)"
    - "Constitution pipeline — advances to approved and claims (constitution_pipeline_test.go:148)"
    - "error handling — rejects double claim (errors_test.go:68)"
    - "Lifecycle Pipeline — advances to approved (lifecycle_pipeline_test.go:48)"
    - "MCP-only lifecycle amend/supersede (skills_test.go:65)"
    - "Lifecycle Amend flow — advances to approved (lifecycle_test.go:52)"
