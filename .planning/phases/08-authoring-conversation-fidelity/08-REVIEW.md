---
phase: 08-authoring-conversation-fidelity
reviewed: 2026-07-15T00:00:00Z
depth: standard
iteration: 2
files_reviewed: 17
files_reviewed_list:
  - cmd/specgraph/approve.go
  - cmd/specgraph/authoring_test.go
  - cmd/specgraph/conversation_flag.go
  - cmd/specgraph/decompose.go
  - cmd/specgraph/shape.go
  - cmd/specgraph/spark.go
  - cmd/specgraph/specify.go
  - e2e/api/mcp_only_authoring_test.go
  - e2e/api/mcp_only_conversation_test.go
  - internal/mcp/skills/embedded/specgraph-authoring/SKILL.md
  - internal/mcp/testhelpers_test.go
  - internal/mcp/tools_authoring_test.go
  - internal/mcp/tools_authoring.go
  - internal/server/authoring_handler_test.go
  - internal/server/authoring_handler.go
  - internal/storage/postgres/conversation_test.go
  - proto/specgraph/v1/authoring.proto
findings:
  critical: 0
  warning: 0
  info: 0
  total: 0
status: clean
---

# Phase 8: Code Review Report (Iteration 2)

**Reviewed:** 2026-07-15
**Depth:** standard
**Files Reviewed:** 17
**Status:** clean

## Summary

This is iteration 2 of the auto fix+review loop. Iteration 1 raised 3 Warnings
and 4 Info findings; the fixer resolved 6 (WR-01 partial, WR-02, WR-03, IN-01,
IN-02, IN-03) and deliberately deferred IN-04 (explicitly-optional coverage
note). This pass re-verified every fix against the live source, re-ran the
touched-package build/vet/test suites, and looked for regressions or new
defects.

**Verification result: all six fixes are correct, complete, and free of
regressions. No new Critical/Warning/Info issues were found.** Per the loop
rules, the lone documented-optional skip (IN-04) does not count as unresolved,
so the phase is marked `clean`.

Verification evidence:

- `go build ./...` — clean.
- `go vet ./internal/server/... ./cmd/specgraph/...` — clean (no shadow/wrapcheck
  regressions in the touched scope).
- `go test ./internal/server/... ./cmd/specgraph/... ./internal/authoring/...`
  — all green, including the `RecordConversation`, `PostureToString`, and
  conversation-rollback tests, and the CLI `--conversation` loader tests.

## Fix Verification

**WR-01 — `RecordConversation` RPC hardening (partial, accepted).**
`internal/server/authoring_handler.go:676-696` now rejects (a) bogus stages via
`storage.SpecStage(stage).IsValid()`, (b) empty exchange content, and (c)
content longer than `authoring.MaxExchangeContentLen` (4096). This closes the
material integrity gap the finding raised — the "unbounded JSONB writes" vector
is now bounded to ≤100 exchanges × ≤4096 chars. The strictly-increasing
sequence check from `authoring.ValidateExchanges` was intentionally NOT applied,
because this RPC's existing callers/tests rely on paired/unset-sequence
semantics (documented probe/response pairing) that the strict rule would break.
**This deliberate skip is accepted** — it preserves documented behavior and is
not a blocker; the security-adjacent bounds are all in place. The
`TestAuthoringHandler_RecordConversation*` suite passes against the new guards.

**WR-02 — bounded `--conversation` loader.**
`cmd/specgraph/conversation_flag.go:44-95` now caps both the stdin and file read
paths at `maxConversationBytes = 1 MiB` via `io.LimitReader` (reading `limit+1`
to distinguish at-limit from over-limit), and adds an early
`len(input) > authoring.MaxConversationExchanges` (100) check before building the
proto slice. The file path switched from `os.ReadFile` to `os.Open` +
`io.LimitReader` with a correct `//nolint:gosec` for the CLI-supplied local
path, leaving the shared `loadJSONFileRaw` (still used by `conversation.go`)
untouched — no collateral regression. The bound reuses the exported
`authoring.MaxConversationExchanges` constant (single source of truth with the
server contract).

**WR-03 — SKILL.md `exchanges.stage` doc.**
`internal/mcp/skills/embedded/specgraph-authoring/SKILL.md:154` now lists
`shape, specify, decompose, approve`, resolving the contradiction with the same
file's approve requirement (:176-179). The added token has no underscore, so it
does not trip the `TestContentProtoDrift` snake_case check.

**IN-01 — request-context logging.**
`postureToString` and `stageError` now take `ctx context.Context` and log via
`slog.LogAttrs(ctx, …)`; `buildConversationEntry` threads `ctx` through, and all
call sites (four stage handlers + the internal test) pass the request context.
No remaining `context.Background()` in the handler's log paths.

**IN-02 — safety validation hoisted out of the transaction (ADR-004).**
`safetyInput.Validate()` now runs before `runInTxOrSequential` in all four stage
handlers (Spark:78, Shape:180, Specify:304, Decompose:399), and was removed from
`persistSafetyFlags` (whose doc comment now states the caller must validate
first). Confirmed `persistSafetyFlags` has exactly four call sites, each
preceded by validation; the Approve handler carries no safety input, so it is
correctly unaffected. Error surface unchanged (`CodeInvalidArgument`).

**IN-03 — parallel-unsafe shared flag globals.**
`cmd/specgraph/authoring_test.go:21-28` adds the documenting comment (the
finding's lower-risk option): tests mutate package-level flag globals, are not
parallel-safe, must never call `t.Parallel()`, and new coverage should prefer
fresh `cobra.Command` instances.

**IN-04 — CLI happy-path real-validation coverage (accepted skip).**
Intentionally deferred as explicitly optional ("a coverage-shape note, not a gap
in the system's guarantees"). Real conversation validation remains covered by
the server handler tests and the MCP-only e2e suite. Does not count against
`clean`.

## Narrative Findings (AI reviewer)

No new Critical, Warning, or Info defects were found. The fixes are localized,
the build/vet/test gates are green for the touched packages, and no behavioral
regression was observed in the conversation-enforcement, transaction-atomicity,
input-validation, or error-sanitization paths that were the focus of this phase.

---

_Reviewed: 2026-07-15_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
