---
phase: 8
reviewers: [cursor]
reviewed_at: 2026-07-15T15:55:34Z
review_round: 2
plans_reviewed: [08-01-PLAN.md, 08-02-PLAN.md, 08-03-PLAN.md, 08-04-PLAN.md]
prior_round_incorporated: true
---

# Cross-AI Plan Review — Phase 8 (Round 2, post-revision)

> Round 1 (reviewers: cursor) produced 7 findings; all 7 were incorporated via `/gsd-plan-phase 8 --reviews` (commit d7f1f9b1). This round re-reviews the revised plans and confirms incorporation + surfaces remaining gaps.

## Cursor Review

# Phase 8 Cross-AI Plan Review — Authoring Conversation Fidelity

**Reviewed:** 2026-07-15  
**Scope:** `.planning/phases/08-authoring-conversation-fidelity/08-01` through `08-04` (post-revision)  
**Method:** Claims checked against the current working tree at `/Volumes/Code/github.com/specgraph` (plans are not yet implemented).

---

## Executive Summary

The revised plans accurately describe the current codebase: shape/specify/decompose already enforce and record conversations inline; the approve **accept** path, MCP `handleApprove`, CLI `approve`, synthetic CLI placeholder, standalone MCP `conversation record`, and teaching surfaces are the real remaining holes. Prior review findings (#1–#7) are incorporated with concrete tasks and line-level references. The plans are implementable and should achieve CONV-01 if Wave 1 merges atomically.

Remaining risks are mostly execution gaps: MCP `handleSpark` silently drops optional exchanges (pre-existing, not scheduled), the MCP unit mock for approve cannot verify threaded exchanges without a signature change, and there is no explicit approve-accept rollback test mirroring the shape atomicity test.

**Overall risk:** **MEDIUM** — technically sound, coordination-sensitive (Wave 1 merge gate is real).

---

## Baseline Verification (Current Code vs Plan Claims)

| Claim | Verified? | Evidence |
|-------|-----------|----------|
| Approve accept records nothing today | ✓ | Accept branch `runInTxOrSequential` only transitions, promotes decisions, and `GetSpec` — no `ValidateExchanges` / `RecordConversation` at `internal/server/authoring_handler.go:453-485` |
| Reject validates + records under `"approved"` | ✓ | `ValidateExchanges(..., "approve")` at `:488-490`; `entry.Stage = storage.SpecStageApproved` at `:517-521` |
| Proto field 3 already exists | ✓ | `conversation_exchanges = 3` at `proto/specgraph/v1/authoring.proto:358-360`; `GetConversationExchanges()` at `gen/specgraph/v1/authoring.pb.go:1912` |
| `SpecStageApproved` value is `"approved"` | ✓ | `internal/storage/spec_domain.go:19` |
| `ListConversations` exact stage match | ✓ | `WHERE ... AND stage = $3` at `internal/storage/postgres/conversation.go:218-224` |
| MCP approve sends no exchanges | ✓ | `ApproveRequest{Slug: slug}` only at `internal/mcp/tools_authoring.go:337-339` |
| Author tool teaches approve as exchange-free | ✓ | Top-level Description at `:123-124`; `exchanges` prop at `:146-148` |
| MCP `conversation record` still exists | ✓ | `case "record"` at `:447-448`; `handleRecord` at `:456-485` |
| CLI synthetic placeholder in use | ✓ | `cliSyntheticExchanges` in `shape.go:42`, `specify.go:42`, `decompose.go:42`; helper at `authoring_cli_exchanges.go:16-20` |
| CLI approve sends no exchanges | ✓ | `ApproveRequest{Slug: args[0]}` at `cmd/specgraph/approve.go:31-33` |
| Three accept unit tests break under enforcement | ✓ | `_Approve_HappyPath` `:614-623`, `_AcceptUnchangedWithoutAction` `:625-634`, `_ExplicitAcceptSucceeds` `:1999-2010` — none pass `ConversationExchanges` |
| Default fake backend cannot record | ✓ | `authoringTestBackend` embeds `stubBackend`; `RecordConversation` → `errNotImplemented` at `internal/server/test_scoper_test.go:324-326` |
| E2e approve omits exchanges | ✓ | Comment at `e2e/api/mcp_only_authoring_test.go:224-225`; call at `:242` |
| SKILL.md says approve needs no exchanges | ✓ | `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md:176-177` |
| No standalone `conversation record` in SKILL | ✓ | Grep finds no `conversation record` in skill; inline persist at `:55-57` |

---

## Plan 08-01 — Server Approve-Accept Enforcement

### Summary

Correctly scoped as wiring + comment edit, not a new proto field. The reject branch is a faithful template. Task 2 explicitly fixes the three breaking accept tests and the `stubBackend` recording gap — this addresses prior review finding #1.

### Strengths

- **Reject-branch mirror is accurate** — same validate → `exchangesFromProto` → `RecordConversation` pattern as `:487-525`.
- **Stage label pinned correctly** — `storage.SpecStageApproved` (`"approved"`) matches reject behavior (`authoring_handler_test.go:669`).
- **Test tier split** (finding #5) — handler unit tests vs postgres integration in Task 3 matches package build tags.
- **Wave 1 merge gate** (finding #6) — justified: accept currently succeeds with zero exchanges at `:453-485`.

### Concerns

- **MEDIUM — No approve-accept rollback test.** `TestAuthoringHandler_Shape_RecordConversationFailureRollsBack` at `authoring_handler_test.go:1952-1976` proves atomicity for shape; Task 3 behavior claims rollback on `RecordConversation` failure but does not task an equivalent approve-accept test using `newTxConvAuthoringClient`.
- **LOW — Proto comment wording.** Task 1 says “non-default approve action”; `APPROVE_ACTION_UNSPECIFIED` falls through to accept at `:449-453`. Comment should explicitly include UNSPECIFIED/default-accept, not only `APPROVE_ACTION_ACCEPT`.
- **LOW — `buildConversationEntry` vs literal entry.** `08-PATTERNS.md` notes posture absence on `ApproveRequest`; plan should pick one approach (literal entry like reject is fine).

### Suggestions

- Add `TestAuthoringHandler_Approve_AcceptRecordConversationFailureRollsBack` mirroring `:1952-1976` with `newTxConvAuthoringClient` and valid approve exchanges.
- Clarify proto comment: required for accept (including UNSPECIFIED) and reject.

### Risk Assessment

**MEDIUM** — Implementation is straightforward; main risk is merging without 08-02/08-03.

---

## Plan 08-02 — MCP Surface + Skills

### Summary

Correctly targets the #906 skip surface (standalone `record`) and threads exchanges into `handleApprove`. Prior findings #4 (top-level Description) and #7 (SKILL scope) are addressed.

### Strengths

- **`handleShape` template is correct** — `parseOptionalExchanges` + `ConversationExchanges` at `tools_authoring.go:248-257`.
- **Security control preserved** — reuses isolated JSON parse at `:71-81` (V5).
- **List-only switch pattern exists** — `findingsTool.handle` at `internal/mcp/tools_core.go:170-178`.
- **Test pruning list is accurate** — `TestConversationTool_Record*` at `tools_authoring_test.go:676-755`.
- **SKILL.md scope trimmed correctly** (finding #7) — no `conversation record` prose to remove; approve flip at `:66-67`, `:176-177` is the real work.

### Concerns

- **MEDIUM — `mockAuthoringService.Approve` cannot verify threading.** Callback only receives `slug` at `internal/mcp/testhelpers_test.go:365-369`; `TestAuthorTool_Approve` at `tools_authoring_test.go:97-119` cannot assert `ConversationExchanges` without changing the mock to pass `req.Msg` or exchanges.
- **MEDIUM — `handleSpark` exchange gap not scheduled.** Server records spark exchanges when present (`authoring_handler.go:57-103`), but MCP `handleSpark` omits `ConversationExchanges` at `:219-223`. Plan 08-03 adds optional CLI `--conversation` for spark; MCP parity is missing. Optional ≠ drop-when-provided.
- **LOW — `TestConversationTool_UnknownAction` already exists** at `:772-785`; Task 2’s new “record → unknown action” test overlaps — extend existing test or assert `record` specifically.

### Suggestions

- Extend `mockAuthoringService.approve` to `func(req *specv1.ApproveRequest)` (or assert on `req.Msg.GetConversationExchanges()`).
- Add a one-line Task 1 follow-up: thread optional spark exchanges in `handleSpark` when `parseOptionalExchanges` returns non-nil (D-01 parity with server).

### Risk Assessment

**MEDIUM** — Correct direction; mock and spark gaps could let regressions slip through unit tests.

---

## Plan 08-03 — CLI `--conversation` Loader

### Summary

Accurately targets the synthetic escape hatch and pins the bare-array contract (A2). `conversation record` object shape vs `--conversation` array is correctly flagged (Pitfall #3).

### Strengths

- **Deletion target verified** — `authoring_cli_exchanges.go` exists; used only from shape/specify/decompose (not approve).
- **Loader analog exists** — `conversationRecordInput` + `loadJSONFileRaw` at `conversation.go:59-68`, `util.go:37-45`.
- **Flag help requirement** (finding #3) — explicit distinction from `{"exchanges":[...]}` is necessary.
- **Atomic commit** — helper + rewire + delete in one task prevents dangling references.
- **Spark optional pattern** — matches server conditional at `authoring_handler.go:57-64`.

### Concerns

- **MEDIUM — Required-flag enforcement unspecified.** Plan mentions `registerConversationFlag(..., required bool)` but Cobra needs `MarkFlagRequired("conversation")` (see `conversation.go:48-49`). Implementers should mirror that pattern per command.
- **LOW — No synthetic-placeholder tests to remove.** `authoring_test.go` has spark/shape tests but no `cliSyntheticExchanges` assertions; “remove placeholder tests” is precautionary only.
- **LOW — Approve CLI currently has no JSON/output flags** — adding required `--conversation` is a breaking CLI change (intentional per D-04).

### Suggestions

- Task 1: explicitly `cobra.CheckErr(cmd.MarkFlagRequired("conversation"))` for shape/specify/decompose/approve after `StringVar`.
- Task 2(d): use `runShape` with a cobra command built in-test (existing `authoring_test.go` pattern) to assert missing-flag error before RPC.

### Risk Assessment

**LOW–MEDIUM** — Straightforward; operator confusion between array vs object formats is the main footgun (mitigated by help text + Task 2(c)).

---

## Plan 08-04 — MCP-Only E2E Gate

### Summary

Correctly identifies the breaking e2e line and stage-string discipline. Prior finding #2 (`"approved"` vs `"approve"`) is well documented.

### Strengths

- **Breaking call verified** — `author(map[string]any{"action": "approve"})` at `mcp_only_authoring_test.go:242`.
- **Stage filter discipline is critical and correct** — exchange JSON uses `"approve"` (`ValidateExchanges` target at `validate.go:70-71`); stored/list filter must use `"approved"` (`spec_domain.go:19`, `conversation.go:222`).
- **Harness reuse** — `mcpProjectClient` at `:49`; per-Describe project isolation at `:24-31`.
- **Negative case template exists** — shape-without-exchanges rejection at `:261-276`.

### Concerns

- **LOW — Overlap between Task 1 and Task 2.** Both add full-funnel + per-stage conversation assertions; Task 2 may duplicate Task 1 unless scoped (Task 1 = extend existing spec; Task 2 = dedicated fidelity + negative-only file).
- **LOW — Negative approve case timing.** After 08-02 client-side reject, e2e may see MCP tool error before server `InvalidArgument`; assert on `res.IsError` (as `:275`) rather than only Connect code.

### Suggestions

- Task 2 negative spec: assert `res.IsError` **or** error text containing `exchanges`/`InvalidArgument` — don’t require server round-trip if 08-02 client guard fires first.
- Task 1: update stale comment at `:224-225` when adding approve exchanges.

### Risk Assessment

**LOW** — Depends on Wave 1 landing first; stage-string mistake would cause false-negative fidelity assertions (plans guard against this).

---

## Cross-Cutting Findings

### Prior review fixes — incorporation status

| Finding | Status in revised plans |
|---------|-------------------------|
| #1 Three accept tests + recording backend | ✓ `08-01` Task 2 |
| #2 `"approved"` vs `"approve"` in e2e/list filters | ✓ `08-04` Tasks 1–2, `08-01` Task 3 |
| #3 `--conversation` help vs object shape | ✓ `08-03` Task 1 |
| #4 Top-level `author` Description, not just prop | ✓ `08-02` Task 1 |
| #5 Handler unit vs storage integration tiers | ✓ `08-01` Task 3 |
| #6 Wave 1 atomic merge gate | ✓ All Wave 1 plans |
| #7 SKILL edits = approve flip only | ✓ `08-02` Task 3 |

### New / remaining issues not fully covered

1. **HIGH (operational):** Wave 1 must merge together — verified by current accept path accepting empty exchanges while MCP/CLI send none.
2. **MEDIUM:** MCP `handleSpark` does not forward exchanges (`tools_authoring.go:219-223`) while server supports them (`authoring_handler.go:96-103`).
3. **MEDIUM:** No explicit approve-accept transaction rollback test (shape has one at `authoring_handler_test.go:1952`).
4. **MEDIUM:** MCP approve unit test cannot verify `ConversationExchanges` without mock change (`testhelpers_test.go:369`).
5. **LOW (intentional):** Standalone `RecordConversation` RPC (`authoring_handler.go:612-655`) and CLI `conversation record` remain bypass paths for humans/backfill (D-07/D-08) — not CONV-01 agent-funnel violations.

### Security / performance

- Reusing `ValidateExchanges` caps (`validate.go:12-17`, `:46-48`) and `parseOptionalExchanges` isolation is correct; no new attack surface.
- Validation-before-tx on accept matches ADR-004 guidance; fourth op inside existing `runInTxOrSequential` is appropriate.

### Phase goal attainment

If executed as written (plus Wave 1 atomic merge), the plans close the documented holes and satisfy CONV-01 criteria #1–#4. The MCP-only e2e at `08-04` would have caught #906’s skip pattern.

---

## Overall Risk Assessment

| Dimension | Level | Justification |
|-----------|-------|---------------|
| Technical correctness | **LOW** | Claims match source; templates exist in-tree |
| Coordination / release | **HIGH** | Partial Wave 1 merge breaks all approve clients |
| Test completeness | **MEDIUM** | Rollback + mock threading + spark MCP gaps |
| Scope creep | **LOW** | Focused refactor; no new dependencies |

**Composite: MEDIUM** — Plans are ready to execute with the suggestions above; treat Wave 1 as a single release unit and add the missing approve rollback + MCP mock/spark items before calling the phase done.

---

## Consensus Summary

One external reviewer (Cursor agent, source-grounded against the working tree). Round-2 verdict: **MEDIUM risk, execution-ready.** All 7 Round-1 findings confirmed incorporated with correct line-level references. Remaining items are new lower-severity gaps, not regressions.

### Agreed Strengths
- Revised plans accurately match current code; reject branch is a faithful template for accept; A1 (`storage.SpecStageApproved`="approved") and A2 (bare-array `--conversation`) pins are correct.
- All 7 prior findings verified incorporated (#1 three accept tests + recording backend → 08-01 T2; #2 "approved" vs "approve" filters → 08-04 + 08-01 T3; #3 flag-help shape → 08-03 T1; #4 top-level author Description → 08-02 T1; #5 unit/integration tier split → 08-01 T3; #6 wave-1 merge gate → all wave-1 plans; #7 SKILL approve-flip scope → 08-02 T3).

### Agreed Concerns (new/remaining — none are BLOCKERs)
- **[HIGH, operational — already in plans] Wave-1 atomic merge.** 08-01+08-02+08-03 must ship as one release unit or approve breaks for all clients. Already framed as a hard-merge-gate in the plans; no plan change needed, honor at merge time.
- **[MEDIUM] MCP `handleSpark` drops optional exchanges** (`tools_authoring.go:219-223`) while the server records them when present (`authoring_handler.go:96-103`). D-01 optional-spark parity is missing on the MCP path (08-03 adds it for CLI only). Consider a one-line 08-02 follow-up: thread optional spark exchanges when `parseOptionalExchanges` returns non-nil.
- **[MEDIUM] No approve-accept transaction rollback test.** Shape has `TestAuthoringHandler_Shape_RecordConversationFailureRollsBack` (`authoring_handler_test.go:1952-1976`); 08-01 Task 3 claims rollback behavior but doesn't task an equivalent approve-accept test via `newTxConvAuthoringClient`.
- **[MEDIUM] MCP approve unit test can't verify threaded exchanges** without a mock change — `mockAuthoringService.Approve` only receives `slug` (`testhelpers_test.go:365-369`); 08-02's `TestAuthorTool_Approve` can't assert `ConversationExchanges` unless the mock passes `req.Msg`.
- **[LOW] 08-03 required-flag enforcement** needs `MarkFlagRequired("conversation")` (Cobra) per command, mirroring `conversation.go:48-49` — plan mentions a `required bool` param but not the Cobra call.
- **[LOW] 08-01 proto comment** should include `APPROVE_ACTION_UNSPECIFIED`/default-accept, not only `APPROVE_ACTION_ACCEPT`.

### Confirmed Out-of-Scope (intentional, no change)
- Standalone `AuthoringHandler.RecordConversation` RPC + CLI `conversation record` remain (D-07/D-08) — human/backfill paths, not CONV-01 agent-funnel violations.

### Divergent Views
- None (single reviewer).

### Recommendation
Plans are execution-ready. The remaining MEDIUM items are test-completeness/parity gaps best folded in during execution (or a light `--reviews` pass): add the approve-accept rollback test + MCP approve-mock exchange assertion (08-01/08-02), and decide on MCP spark-exchange parity (08-02). None block starting execution; treat wave-1 as a single release unit.
