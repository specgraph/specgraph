---
phase: 8
reviewers: [cursor]
reviewed_at: 2026-07-15T15:27:17Z
plans_reviewed: [08-01-PLAN.md, 08-02-PLAN.md, 08-03-PLAN.md, 08-04-PLAN.md]
---

# Cross-AI Plan Review — Phase 8

## Cursor Review

# Phase 8 Plan Review — Authoring Conversation Fidelity

Verified against the current tree at `/Volumes/Code/github.com/specgraph` (2026-07-15). Claims below cite on-disk source.

---

## 1. Summary

The four plans correctly describe a **close-the-holes refactor**, not greenfield work. Research is largely accurate: shape/specify/decompose already enforce non-empty exchanges and record atomically in `runInTxOrSequential`; the approve **accept** path, MCP `handleApprove`, CLI synthetic placeholder, and MCP `conversation record` action are the real gaps. Plan decomposition (server → MCP → CLI → e2e), wave-1 file isolation, and the reject-branch template for accept are sound. **Overall risk is MEDIUM**: execution is straightforward, but wave-1 must merge atomically, several existing tests will break unless updated alongside 08-01, and e2e conversation assertions must use the stored stage string `"approved"` (not `"approve"`).

---

## 2. Strengths

- **Accurate baseline diagnosis.** Shape unconditionally validates and records in one transaction:

```136:201:internal/server/authoring_handler.go
	// Required conversation_exchanges per design §Conversation Recording Coupling.
	if err := authoring.ValidateExchanges(msg.GetConversationExchanges(), "shape"); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// ...
	if err := runInTxOrSequential(ctx, store,
		// ... TransitionStage, StoreShapeOutput, persistSafetyFlags ...
		func(c context.Context) error {
			if _, err := store.RecordConversation(c, msg.Slug, entry); err != nil {
				return fmt.Errorf("record conversation: %w", err)
			}
			return nil
		},
	); err != nil {
```

- **Reject branch is a verified template for accept.** Reject validates, records under `SpecStageApproved`, and runs inside `runInTxOrSequential`:

```487:524:internal/server/authoring_handler.go
	case specv1.ApproveAction_APPROVE_ACTION_REJECT:
		if err := authoring.ValidateExchanges(req.Msg.GetConversationExchanges(), "approve"); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		// ...
				entry.Stage = storage.SpecStageApproved
		// ...
				if _, err := store.RecordConversation(txCtx, slug, entry); err != nil {
					return fmt.Errorf("record conversation: %w", err)
				}
```

- **Proto scope reduction is correct.** `conversation_exchanges` is already field 3; only the comment needs widening:

```352:361:proto/specgraph/v1/authoring.proto
message ApproveRequest {
  string slug = 1;
  ApproveAction action = 2;
  // REQUIRED when action == APPROVE_ACTION_REJECT. Conversation exchanges
  // capturing the rejection rationale.
  repeated ConversationExchange conversation_exchanges = 3;
}
```

- **Root-cause surfaces are correctly identified.** MCP approve sends slug only (`internal/mcp/tools_authoring.go:332-339`); author Description says approve does not need exchanges (`:146-148`); `conversationTool` still exposes `record` (`:444-454`, `handleRecord` `:456-485`); CLI stamps `cliSyntheticExchanges` in shape/specify/decompose (`cmd/specgraph/shape.go:42`, etc.) and approve sends no exchanges (`cmd/specgraph/approve.go:31-33`).

- **Wave-1 parallelism is real.** 08-01 / 08-02 / 08-03 touch disjoint paths (`internal/server/`, `internal/mcp/`, `cmd/specgraph/`). Atomic-release warning is justified (see Concerns).

- **D-01 (spark optional) matches code.** Spark validates and records only when exchanges are present (`internal/server/authoring_handler.go:57-104`); spark CLI has no conversation input (`cmd/specgraph/spark.go:33-36`).

- **Reusable primitives exist as claimed.** `ValidateExchanges` at `internal/authoring/validate.go:42-48` with caps at `:12-17`; `parseOptionalExchanges` at `internal/mcp/tools_authoring.go:71-82`; `loadJSONFileRaw` at `cmd/specgraph/util.go:37-45`; export keeps `RecordConversation` at `internal/export/engine.go:588`.

- **08-04 correctly flags the breaking e2e.** Funnel test approves without exchanges and documents that as current behavior:

```222:242:e2e/api/mcp_only_authoring_test.go
		// decompose pass friendly YAML output PLUS explicit JSON exchanges; approve
		// passes slug only (the ACCEPT path does not require exchanges).
		// ...
		author(map[string]any{"action": "approve"})
```

---

## 3. Concerns

### HIGH

- **Wave-1 partial merge breaks approve for all clients.** Accept today succeeds with zero exchanges (`internal/server/authoring_handler.go:453-485`). If 08-01 lands alone, MCP (`handleApprove` at `tools_authoring.go:337-339`) and CLI (`approve.go:31-33`) still send no exchanges → `InvalidArgument`. Plans note this; treat it as a **hard merge gate**, not advisory.

- **Existing approve unit tests will fail on 08-01 and are not listed for update.** These pass today without `ConversationExchanges`:
  - `TestAuthoringHandler_Approve_HappyPath` (`authoring_handler_test.go:614-623`)
  - `TestAuthoringHandler_Approve_AcceptUnchangedWithoutAction` (`:625-634`)
  - `TestAuthoringHandler_Approve_ExplicitAcceptSucceeds` (`:1999-2010`)  
  08-01 Task 3 adds new cases but does not call out fixing these. Task 2’s verify (`go test ./internal/server/...`) will fail until they are updated. Even with exchanges added, happy-path backends use `&fakeBackend{}` whose embedded `stubBackend.RecordConversation` returns `errNotImplemented` (`test_scoper_test.go:324-326`) — accept recording tests need `fakeConvBackend` / `rejectAuthoringTestBackend`-style wiring.

### MEDIUM

- **E2e stage filter likely wrong for approve.** Plans say list conversations filtered by stage including `"approve"`, but accept/reject recordings set `entry.Stage = storage.SpecStageApproved` (`authoring_handler.go:517`), and the domain constant is `"approved"` (`internal/storage/spec_domain.go:19`). `ListConversations` filters with exact SQL match (`internal/storage/postgres/conversation.go:218-224`). Filtering `stage="approve"` will return empty; use `"approved"` or assert via unfiltered list + count.

- **08-01 Task 3 mislabels test tier.** `internal/server/authoring_handler_test.go` has no `//go:build integration` tag (unit + fakes). `internal/storage/postgres/conversation_test.go` is integration (`conversation_test.go:4`). Mixing them under “handler integration test” is fine for coverage but verify commands should not imply Docker is required for handler package unit tests.

- **Residual out-of-band recording path (in scope by design, but weakens “only path”).** Removing MCP `conversation record` does not remove `AuthoringHandler.RecordConversation` RPC (`authoring_handler.go:612-652`) or CLI `conversation record` (`cmd/specgraph/conversation.go:70-98`). D-07/D-08 justify this; worth noting CONV-01 is “MCP/skills funnel” enforcement, not a global ban on standalone writes.

- **CLI `--conversation` shape divergence (Pitfall #3).** Stage `--conversation` is specified as bare JSON array (D-05/A2), but `conversation record` uses `{"exchanges":[...]}` (`conversation.go:60-67`). Plan acknowledges it; operators will confuse the two formats without prominent flag help.

- **D-09 “conversation record” framing may be stale in SKILL.md.** Research cites standalone-record teaching; current `specgraph-authoring` SKILL already says persist via `author` inline (`SKILL.md:55-57`) and does not mention the `conversation` tool. D-09 is still required for the approve exchanges flip (`SKILL.md:66-67`, `:176-177`) and author Description parity — not for removing record-step prose that may already be gone.

### LOW

- **Line-number drift in research/plans.** Examples: `conversationTool` def is at `tools_authoring.go:424`, not ~416; proto field is at `authoring.proto:360`, not always cited consistently. Plans use `read_first` ranges — harmless if implementers search by symbol.

- **`TestContentProtoDrift` location.** Plan 08-02 verify runs it under `./internal/mcp/...`; the test lives in `internal/authoring/drift_test.go:15`. Still runnable via `go test ./internal/authoring/... -run TestContentProtoDrift` or `task check`.

- **`mcp_only_conversation_test.go` does not exist yet.** Expected for 08-04 (Wave 0 gap); not a plan error.

---

## 4. Suggestions

- **08-01 Task 2:** Add an explicit subtask to update `TestAuthoringHandler_Approve_HappyPath`, `_AcceptUnchangedWithoutAction`, and `_ExplicitAcceptSucceeds` to supply valid approve exchanges and use a backend that implements `RecordConversation` (mirror `newFakeRejectBackend` / `fakeConvBackend` at `authoring_handler_test.go:259-296`).

- **08-04 Tasks 1–2:** Pin e2e list filter for approve conversations to `stage: "approved"` (stored value), while exchange JSON uses `stage: "approve"` for `ValidateExchanges` — document both strings in test comments to avoid A1 confusion.

- **08-02 Task 1:** Update the top-level author `Description` sentence at `tools_authoring.go:123-124` (currently lists only shape/specify/decompose), not only the `exchanges` prop at `:146-148`.

- **08-03 Task 1:** In `--conversation` help text, state explicitly: “bare JSON array; not the `conversation record` object shape.”

- **08-01 Task 3:** Split verify commands: `go test ./internal/server/...` (unit, no Docker) vs `go test -tags integration ./internal/storage/postgres/...` (Docker).

- **Merge checklist:** Single PR or coordinated merge for 08-01 + 08-02 + 08-03; run `task check` after all three; run `task pr-prep` after 08-04.

---

## 5. Risk Assessment

**Overall: MEDIUM**

| Factor | Level | Evidence |
|--------|-------|----------|
| Technical approach | Low | Reject branch + shape handler patterns are in-repo templates; no schema migration |
| Coordination | **High** | Accept enforcement without MCP/CLI updates breaks approve (`authoring_handler.go:453-485` vs `tools_authoring.go:337-339`, `approve.go:31-33`) |
| Test churn | **Medium** | Existing approve happy-path tests and e2e funnel test (`mcp_only_authoring_test.go:242`) must change with 08-01 |
| E2e correctness | **Medium** | Stage string `"approved"` vs `"approve"` mismatch risks false-negative fidelity assertions |
| Security | Low | Reuses `ValidateExchanges` caps and `parseOptionalExchanges` isolation; ADR-004 tx pattern preserved |
| Scope creep | Low | Plans stay deletion + wiring; no new features |

**Phase goal achievability:** **Yes**, if wave-1 ships together and tests are updated for the new accept contract. The plans map cleanly to CONV-01’s four success criteria; 08-04 is the right backstop once 08-01/02 land.

---

## Per-Plan Notes

### 08-01 (Server / proto)
- **Verified gap:** Accept path has no `ValidateExchanges` and no `RecordConversation` (`authoring_handler.go:453-485` vs reject at `:487-524`).
- **Proto plan:** Comment-only change on field 3 is correct (`authoring.proto:358-360`).
- **Gap:** Existing unit tests at `authoring_handler_test.go:614-634` and `:1999-2010` not in task list.

### 08-02 (MCP + skill)
- **Verified gaps:** `handleApprove` slug-only (`tools_authoring.go:332-339`); `record` action present (`:447-448`); Description/skill say approve needs no exchanges (`tools_authoring.go:148`, `SKILL.md:176-177`).
- **Sound:** Mirror `handleShape` exchange threading (`:248-256`); prune `TestConversationTool_Record*` (`tools_authoring_test.go:676+`).

### 08-03 (CLI)
- **Verified gaps:** `cliSyntheticExchanges` (`authoring_cli_exchanges.go:16-20`) used by shape/specify/decompose; approve has no conversation input.
- **Sound:** New `conversation_flag.go` + delete placeholder in one commit; bare-array contract distinct from `conversationRecordInput` object shape (`conversation.go:60-67`).

### 08-04 (E2e)
- **Verified:** Funnel e2e will break post-08-01 (`mcp_only_authoring_test.go:242`); negative shape-without-exchanges test already exists (`:261-276`).
- **Add:** Approve-without-exchanges negative case and per-stage retrieval; fix stage filter to `"approved"` for approve gate conversations.

---

## Consensus Summary

One external reviewer (Cursor agent, source-grounded against the working tree at `/Volumes/Code/github.com/specgraph`). Findings below are single-reviewer but carry concrete `file:line` evidence, so they are weighted as verified observations rather than impressions.

### Agreed Strengths
- Accurate "close-the-holes" diagnosis: shape/specify/decompose already validate + record atomically in `runInTxOrSequential`; the real gaps are the approve **accept** path, MCP `handleApprove`, CLI synthetic placeholder, and the MCP `conversation record` action.
- Reject branch (`internal/server/authoring_handler.go:487-524`) is a verified in-repo template for the accept branch; proto `conversation_exchanges` is already field 3 (`authoring.proto:352-361`) so 08-01 is comment + wiring, not a new field.
- Wave-1 file isolation is real (`internal/server/` vs `internal/mcp/` vs `cmd/specgraph/`); reusable primitives (`ValidateExchanges`, `parseOptionalExchanges`, `loadJSONFileRaw`) exist as claimed.

### Agreed Concerns (highest priority)
- **[HIGH] Wave-1 must merge atomically.** Accept succeeds today with zero exchanges; if 08-01 lands alone, MCP + CLI still send none → `InvalidArgument` breaks approve for all clients. Treat as a hard merge gate, not advisory.
- **[HIGH] Existing approve unit tests will break and are not listed for update.** `TestAuthoringHandler_Approve_HappyPath` (`authoring_handler_test.go:614-623`), `_AcceptUnchangedWithoutAction` (`:625-634`), `_ExplicitAcceptSucceeds` (`:1999-2010`) pass today without exchanges; 08-01 Task 2's `go test ./internal/server/...` verify will fail until they supply exchanges AND use a backend that implements `RecordConversation` (the default `&fakeBackend{}` / `stubBackend.RecordConversation` returns `errNotImplemented`).
- **[MEDIUM] E2e stage-filter string mismatch.** Accept/reject store `entry.Stage = storage.SpecStageApproved` = `"approved"` (`spec_domain.go:19`), but exchange validation + plan text use `"approve"`. `ListConversations` does exact SQL match — filtering `stage="approve"` returns empty. 08-04 must filter/assert on `"approved"`.
- **[MEDIUM] CLI `--conversation` shape divergence.** Stage flag is a bare JSON array (A2/D-05) while `conversation record` uses `{"exchanges":[...]}`; flag help must call out the difference.

### Divergent Views
- None (single reviewer).

### Recommended Actions Before Execution
1. Add an explicit 08-01 subtask to update the three existing approve unit tests (supply exchanges + a `RecordConversation`-implementing backend, mirroring `newFakeRejectBackend`/`fakeConvBackend` at `authoring_handler_test.go:259-296`).
2. Pin all 08-04 approve conversation list filters/assertions to the stored stage value `"approved"` (document that exchange JSON still uses `"approve"` for `ValidateExchanges`).
3. Split 08-01 Task 3 verify: `go test ./internal/server/...` (unit, no Docker) vs `go test -tags integration ./internal/storage/postgres/...` (Docker).
4. 08-02 Task 1: also update the top-level author `Description` sentence (`tools_authoring.go:123-124`), not just the `exchanges` prop.
5. 08-03 Task 1: `--conversation` help text must state "bare JSON array; not the `conversation record` object shape."
