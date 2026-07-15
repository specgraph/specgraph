---
phase: 08-authoring-conversation-fidelity
fixed_at: 2026-07-15T00:00:00Z
review_path: .planning/phases/08-authoring-conversation-fidelity/08-REVIEW.md
iteration: 1
findings_in_scope: 7
fixed: 6
skipped: 1
status: partial
---

# Phase 8: Code Review Fix Report

**Fixed at:** 2026-07-15
**Source review:** .planning/phases/08-authoring-conversation-fidelity/08-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 7 (all — Warning + Info)
- Fixed: 6
- Skipped: 1

All commits carry the DCO `Signed-off-by` trailer required by AGENTS.md. Each
fix was verified with `gofmt`, `go build`, and targeted `go test` for the
touched packages (`internal/server`, `cmd/specgraph`, `internal/authoring`) —
all green. Postgres integration and e2e suites (Docker-gated) were not run in
this environment; see the WR-01 note.

## Fixed Issues

### WR-01: `RecordConversation` RPC bypasses `ValidateExchanges` — partial hardening

**Files modified:** `internal/server/authoring_handler.go`
**Commit:** 1187fdb0
**Status:** fixed (partial) — requires human verification

**Applied fix:** Added three guards to the standalone `RecordConversation`
handler that close the material integrity gap the reviewer flagged:
1. `stage` is now validated via `storage.SpecStage(stage).IsValid()` (rejects
   bogus stages like `"banana"` instead of blindly coercing them).
2. Each exchange's `content` must be non-empty.
3. Each exchange's `content` must be `≤ authoring.MaxExchangeContentLen` (4096),
   directly closing the "unbounded JSONB writes" concern (a caller writing 100
   exchanges each with arbitrarily large content).

**Deliberately NOT applied (partial):** The full routing through
`authoring.ValidateExchanges` — specifically its *strictly-increasing sequence*
check — was intentionally left out. Applying it would break existing behavior
and tests that this RPC's callers rely on:
- The unit test `TestAuthoringHandler_RecordConversation` records two exchanges
  both at `Sequence: 1` (paired probe/response), which `ValidateExchanges`
  rejects as non-monotonic.
- The e2e tests in `e2e/api/conversation_test.go` record exchanges with unset
  `Sequence` (0), which `ValidateExchanges` rejects (`sequence must be >= 1`).
- There is a genuine semantic tension in the codebase: `SKILL.md` documents
  "sequence … same number pairs a probe with its response," while
  `ValidateExchanges` enforces strictly-increasing sequences. Propagating the
  strict rule to this RPC would encode the disputed side of that ambiguity.
- The e2e suite is Docker-gated and could not be verified in this environment,
  so rewriting those fixtures blind was avoided.

Per the phase's fix guidance ("if it risks breaking other callers/tests,
document and skip … or relax only the stage-match check"), the sequence check
was skipped while the security-adjacent checks (content bounds, stage validity)
were applied. A human should confirm whether the RPC should adopt the strict
sequence contract (and update the unit + e2e fixtures accordingly) or keep the
looser paired/unset-sequence contract documented here.

### WR-02: Unbounded input read in `--conversation` loader

**Files modified:** `cmd/specgraph/conversation_flag.go`
**Commit:** 288d0df7
**Applied fix:** Bounded both the stdin and file read paths to
`maxConversationBytes = 1 MiB` via `io.LimitReader` (reading `limit+1` to detect
overflow), replacing the unbounded `io.ReadAll(os.Stdin)` and the shared
`loadJSONFileRaw` (`os.ReadFile`) path. The file path now opens with `os.Open` +
`io.LimitReader` so the shared `loadJSONFileRaw` (still used by `conversation.go`)
is left untouched. Also added an early `len(input) > authoring.MaxConversationExchanges`
(100) check to fail with a clear message before building the proto slice,
matching the server contract via the exported constant (single source of truth).

### WR-03: SKILL.md `exchanges.stage` field doc omits `approve`

**Files modified:** `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md`
**Commit:** 64f28719
**Applied fix:** Updated the `stage` field reference (line 154) to list
`shape, specify, decompose, approve`, resolving the contradiction with the same
file's approve requirement. Verified with `task skills:validate` (all packages
OK). The added token `approve` has no underscore, so it does not trip the
`TestContentProtoDrift` snake_case check.

### IN-01: Handler logs use `context.Background()` instead of the request context

**Files modified:** `internal/server/authoring_handler.go`,
`internal/server/authoring_handler_internal_test.go`
**Commit:** bbfe7af7
**Applied fix:** Threaded the request `ctx` into `postureToString` and
`stageError` (both now accept `ctx context.Context`) so their warn/error logs
carry trace correlation and context-scoped telemetry attributes. Updated
`buildConversationEntry` (which calls `postureToString`) to accept and forward
`ctx`, and all four stage-handler call sites plus the internal test
(`postureToString(context.Background(), …)`) accordingly. All 8 `stageError`
call sites updated to pass `ctx`.

### IN-02: Safety-input validation runs inside the transaction, against ADR-004

**Files modified:** `internal/server/authoring_handler.go`
**Commit:** 33c2fc71
**Applied fix:** Hoisted `input.Validate()` out of `persistSafetyFlags` (which
ran inside `runInTxOrSequential`) into each of the four stage handlers (spark,
shape, specify, decompose) immediately after `safetyInput` is constructed and
before the transaction opens. `persistSafetyFlags` now performs only
`RunSafetyNet` + `StoreSafetyFlags` inside the tx; its doc comment states the
caller must validate first. Error surface is unchanged (`CodeInvalidArgument`
on invalid input); the transaction is no longer opened and rolled back solely
to reject structurally-invalid input.

### IN-03: CLI command tests mutate shared package-global flag state

**Files modified:** `cmd/specgraph/authoring_test.go`
**Commit:** a6f930ac
**Applied fix:** Chose the finding's documentation option (the lower-risk of the
two suggested fixes; a full per-test fresh-`cobra.Command` refactor was out of
scope and risked reordering-fragility regressions). Added a prominent comment
documenting that these tests mutate package-level flag globals via
save/restore, are NOT parallel-safe, must never call `t.Parallel()`, and that
new coverage should prefer fresh `cobra.Command` instances (as
`TestConversationFlag_RequiredBeforeDispatch` already does).

## Skipped Issues

### IN-04: CLI happy-path tests exercise no real conversation validation

**File:** `cmd/specgraph/authoring_test.go:196-201, 280-285, 336-341, 407-414`
**Reason:** skipped — the finding is explicitly optional ("Fix: Optional — add
one CLI test…") and, per its own text, "a coverage-shape note, not a gap in the
system's guarantees." Real conversation validation is already covered by the
server handler tests (`internal/server/authoring_handler_test.go`) and the
MCP-only e2e suite. Adding a real-`AuthoringHandler` CLI test with per-stage
fixtures is additive scaffolding that does not fix a defect; deferring it avoids
introducing new, unrequested test infrastructure.
**Original issue:** The CLI happy-path tests use fake handlers that never
validate, and `withConversation` supplies an exchange whose `stage` is the
literal `"stage"` (a value a real server would reject), so they confirm wiring
but give no coverage that the CLI produces server-acceptable exchanges.

---

_Fixed: 2026-07-15_
_Fixer: the agent (gsd-code-fixer)_
_Iteration: 1_
