---
phase: 08-authoring-conversation-fidelity
reviewed: 2026-07-15T00:00:00Z
depth: standard
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
  warning: 3
  info: 4
  total: 7
status: issues_found
---

# Phase 8: Code Review Report

**Reviewed:** 2026-07-15
**Depth:** standard
**Files Reviewed:** 17
**Status:** issues_found

## Summary

Phase 8 enforces conversation recording across the authoring funnel at three
layers: the server handlers (`ValidateExchanges` + atomic `RecordConversation`
inside `runInTxOrSequential`), the MCP `author` tool (exchange threading,
client-side approve guard), and the CLI (`--conversation` loader with
required-flag wiring). The core enforcement path is sound: every stage that
advances the funnel (shape/specify/decompose, and approve-accept) validates a
non-empty, structurally-correct exchange list *before* the transaction and
records the conversation *inside* the same transaction, so a stage transition
cannot commit without its conversation (verified by the rollback tests and the
MCP-only e2e backstops).

No BLOCKER-class correctness or security defects were found. The transaction
atomicity, slug validation (path-traversal test present), error sanitization
(tests assert connect codes, not messages), and sentinel-error handling all
hold up. However, the enforcement is **not uniform**: the standalone
`RecordConversation` RPC is a materially weaker validation path than the stage
handlers, and the CLI loader reads unbounded input. Plus several documentation
and code-quality issues.

## Warnings

### WR-01: `RecordConversation` RPC bypasses `ValidateExchanges` — weaker, inconsistent validation path

**File:** `internal/server/authoring_handler.go:637-680`
**Issue:** The stage handlers (Shape/Specify/Decompose/Approve) enforce
conversation integrity through `authoring.ValidateExchanges`, which rejects
empty content, content longer than `MaxExchangeContentLen` (4096), invalid
roles, non-monotonic sequences, and stage mismatches. The standalone
`RecordConversation` RPC does **none** of this. It only checks:
- `slug != ""`, `stage != ""`
- `1 <= len(exchanges) <= maxElements` (count only)
- role validity (via `conversationExchangesFromProto`)

It does **not** validate:
- per-exchange content length — `maxElements` (line 23 comment) claims to
  "prevent unbounded writes to graph storage," but a caller may write 100
  exchanges each with arbitrarily large `content`, defeating that guarantee
  and enabling unbounded JSONB writes.
- non-empty content — an exchange with `content: ""` is accepted.
- sequence monotonicity — duplicate/decreasing sequences are accepted.
- that `stage` is a known funnel stage — `storage.SpecStage(stage)` coerces any
  string (e.g. `"banana"`), so a conversation can be recorded under a bogus
  stage.

The `conversation` MCP tool no longer exposes `record` (D-06), but this RPC
remains registered and reachable by any project-scoped ConnectRPC client,
providing a lower-integrity side door around the phase-8 invariants.

**Fix:** Route this handler through the same validator the stage handlers use,
and cap content length:
```go
if err := authoring.ValidateExchanges(req.Msg.Exchanges, stage); err != nil {
    return nil, connect.NewError(connect.CodeInvalidArgument, err)
}
```
If callers legitimately need to record under `"approved"` while exchanges carry
`"approve"`, pass the exchange-level target stage (as the accept path does) or
relax only the stage-match check — but keep the content-length, non-empty, and
sequence checks. Also validate `stage` against `protoToStage`/known values
rather than blind `storage.SpecStage(stage)` coercion.

### WR-02: Unbounded input read in `--conversation` loader (stdin and file)

**File:** `cmd/specgraph/conversation_flag.go:43-57`
**Issue:** `loadConversationFlag` reads the entire input with no size bound —
`io.ReadAll(os.Stdin)` (line 46) and, via `loadJSONFileRaw`, `os.ReadFile(path)`
(util.go:38). There is no cap on payload size or on the number of parsed
exchange objects before they are marshalled into `[]*specv1.ConversationExchange`
and sent. A multi-gigabyte file or piped stream is fully buffered into process
memory client-side; a file with millions of array entries builds millions of
proto messages before the server rejects the request at the 100-exchange limit.
This is CLI-local (path is operator-supplied, so no traversal/injection risk),
so impact is bounded to the local process, but it is a real robustness gap and
was explicitly in the review scope for this loader.

**Fix:** Bound the read and reject oversized input early, e.g.:
```go
const maxConversationBytes = 1 << 20 // 1 MiB
data, err := io.ReadAll(io.LimitReader(os.Stdin, maxConversationBytes+1))
if err != nil { return nil, fmt.Errorf("read stdin: %w", err) }
if len(data) > maxConversationBytes {
    return nil, fmt.Errorf("conversation input exceeds %d bytes", maxConversationBytes)
}
```
Apply the same limit to the file path (read via `os.Open` + `io.LimitReader`
instead of `os.ReadFile`), and optionally reject `len(input) > maxElements`
before building the slice to match the server contract and fail with a clearer
message.

### WR-03: SKILL.md `exchanges.stage` field doc omits `approve`, contradicting the same file's approve requirement

**File:** `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md:154` (vs. `:179`)
**Issue:** The `exchanges` field reference lists the `stage` values as
"`shape`, `specify`, `decompose`" (line 154), omitting `approve`. Twenty-five
lines later the doc states "**Approve** now REQUIRES `exchanges` … Set `stage`
to `approve` on the exchange entries." (line 176-179). An agent that treats the
field-reference list as authoritative may omit or mis-set the `approve` exchange
stage. Because `ValidateExchanges` rejects a mismatched non-empty stage
(`exchange[i] stage "x" does not match target stage "approve"`), this drives the
exact failure the skill exists to prevent. This is agent-facing contract
documentation shipped in the binary, so treat it as source, not prose.

**Fix:** Update line 154 to include `approve`:
```
- `stage` — the authoring stage (`shape`, `specify`, `decompose`, `approve`)
```

## Info

### IN-01: Handler logs use `context.Background()` instead of the request context

**File:** `internal/server/authoring_handler.go:878` (`postureToString`), `:1050` (`stageError`)
**Issue:** Both `slog.LogAttrs(context.Background(), …)` calls discard the
request context, so these warning/error logs lose trace correlation and any
context-scoped attributes the telemetry slog handler would attach (see
`internal/telemetry`). Every other log in the handler threads `ctx`.
**Fix:** Thread the request `ctx` through — pass it into `postureToString`/`stageError`
(or accept it as a parameter) and use `slog.LogAttrs(ctx, …)`.

### IN-02: Safety-input validation runs inside the transaction, against ADR-004

**File:** `internal/server/authoring_handler.go:1062-1065` (`persistSafetyFlags`, called from each stage's tx ops)
**Issue:** `persistSafetyFlags` calls `input.Validate()` — a pure, non-DB
structural check — as an operation *inside* `runInTxOrSequential`. When it fails
(e.g. Decompose with empty slices → empty safety text), a DB transaction is
opened and rolled back solely to reject structurally-invalid input. ADR-004
(cited in AGENTS.md) states "Validation that doesn't hit the DB stays outside to
reduce lock time." This holds an unnecessary transaction/lock for a check that
could run before the tx.
**Fix:** Call `input.Validate()` before entering `runInTxOrSequential` in each
stage handler (or split `persistSafetyFlags` so validation is hoisted out), and
keep only `RunSafetyNet` + `StoreSafetyFlags` inside the tx.

### IN-03: CLI command tests mutate shared package-global flag state

**File:** `cmd/specgraph/authoring_test.go:24-30` (`withConversation`), and the `*Conversation`/`*JSONFile` save/restore blocks throughout
**Issue:** Tests read/write package-level globals (`shapeConversation`,
`shapeJSONFile`, `specifyConversation`, `sparkSeed`, …) via save-old/restore-in-cleanup.
This shared mutable state is not parallel-safe and creates ordering fragility
between tests (e.g. `TestRunShape_InvalidJSONFile` relies on the JSON-file load
failing before the empty `shapeConversation` is ever loaded). No test uses
`t.Parallel()` today, so this is latent rather than active.
**Fix:** Prefer constructing fresh `cobra.Command` instances with local flag
bindings per test (as `TestConversationFlag_RequiredBeforeDispatch` already
does) instead of mutating the shared command globals, or document that these
tests must never be parallelized.

### IN-04: CLI happy-path tests exercise no real conversation validation

**File:** `cmd/specgraph/authoring_test.go:196-201, 280-285, 336-341, 407-414`
**Issue:** The CLI happy-path tests use fake handlers (`fakeShapeHandler`, etc.)
that return canned responses and never validate. The `withConversation` helper
even supplies an exchange whose `stage` is the literal `"stage"` — a value a
real server would reject as a stage mismatch. So these tests confirm wiring
(flag → request field) but give no coverage that the CLI produces
server-acceptable exchanges. Real validation is covered by the server handler
tests and the MCP-only e2e suite, so this is a coverage-shape note, not a gap in
the system's guarantees.
**Fix:** Optional — add one CLI test that drives a real `AuthoringHandler`
(as the server tests do) with a realistic per-stage `--conversation` fixture to
prove the CLI-produced payload passes `ValidateExchanges`.

---

_Reviewed: 2026-07-15_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
