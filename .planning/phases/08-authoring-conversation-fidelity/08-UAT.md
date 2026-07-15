---
status: testing
phase: 08-authoring-conversation-fidelity
source: [08-VERIFICATION.md]
started: 2026-07-15T17:25:33Z
updated: 2026-07-15T17:25:33Z
---

## Current Test

number: 1
name: Storage integration — `TestListConversations_ApprovedStageRetrievable` passes under Docker
expected: |
  A conversation recorded under storage.SpecStageApproved is retrievable via
  ListConversations filtered on the stored value "approved" — proving SC4 runtime
  retrieval and the approve-gate stage-string discipline.
awaiting: user response

## Tests

### 1. Storage integration — `TestListConversations_ApprovedStageRetrievable` passes under Docker
expected: Run `task pr-prep` (Docker). The storage integration test `TestListConversations_ApprovedStageRetrievable` (internal/storage/postgres/conversation_test.go) passes green — a conversation recorded under storage.SpecStageApproved is retrievable via ListConversations filtered on the stored value "approved" (SC4 runtime retrieval + approve-gate stage-string discipline).
result: [pending]

### 2. MCP-only conversation-fidelity e2e passes under Docker
expected: Run the e2e tier (`go test -tags e2e ./e2e/api/...` under Docker, via `task pr-prep`). Positive full-funnel spec (`records a non-empty retrievable conversation at every required stage`) passes — every required stage (shape/specify/decompose/approve) has a non-empty retrievable conversation (approve filtered on "approved"). Negative spec (`rejects approve without exchanges`) passes — approve with no exchanges returns res.IsError (client guard or server InvalidArgument); a missing conversation cannot silently pass. The D-10 regression backstop for #906.
result: [pending]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
