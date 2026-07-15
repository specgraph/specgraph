# Phase 8: Authoring Conversation Fidelity - Research

**Researched:** 2026-07-15
**Domain:** Go / ConnectRPC + protobuf authoring funnel; MCP tool surface; Cobra CLI; pgx/PostgreSQL conversation storage
**Confidence:** HIGH (all claims verified against on-disk source this session via codegraph + Read)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Required stages = shape, specify, decompose, and approve (accept). Shape/specify/decompose already enforce non-empty exchanges. **Spark stays OPTIONAL** — record only if exchanges present.
- **D-02:** The approve ACCEPT path must record a non-empty conversation, symmetric with the existing reject path. Validate via `authoring.ValidateExchanges(..., "approve")`, record atomically in the approve transaction (same pattern as reject).
  - Requires: proto change to `ApproveRequest` (`conversation_exchanges`); `task proto`; MCP `author` `approve` action accepts + threads `exchanges`; tool `Description` corrected.
- **D-03:** Enforcement is a hard reject — a required stage with missing/empty exchanges fails with `InvalidArgument`. No silent auto-synthesis on any agent-driven path. Extends to approve-accept.
- **D-04:** Remove the CLI synthetic-placeholder escape hatch. Delete `cliSyntheticExchanges` (`cmd/specgraph/authoring_cli_exchanges.go`) and its uses. Real exchanges required everywhere.
- **D-05:** CLI supplies real exchanges via a `--conversation <file.json>` flag (and `--conversation -` for stdin). File is a JSON array of `ConversationExchange` objects — same shape the MCP `author` tool accepts. Stage commands error if the flag is missing/empty for a required stage. Applies to shape/specify/decompose/approve; spark's `--conversation` is optional.
- **D-06:** Retire the agent-facing standalone MCP `record` action. Remove the MCP `conversation` tool's `record` action; the ONLY MCP recording path is inline with the stage save. **Keep the `list` action** (criterion #4).
- **D-07:** Keep the storage-level `Store.RecordConversation` method — consumed by export/import (`internal/export/engine.go`) and inline stage-handler recording.
- **D-08:** Keep the CLI `conversation record` command for manual/backfill/out-of-band recording by humans.
- **D-09:** Update the MCP-first skills so exchanges are taught as inline-with-save only, and approve is taught as **now requiring** exchanges on clean acceptance (flip current text). Remove "then call conversation record" framing. `TestContentProtoDrift`-style checks gate token drift.
- **D-10:** Verification gate = MCP-only funnel e2e + integration tests. MCP-only Ginkgo e2e (`e2e/api/`) drives full funnel via `author` tool, asserts non-empty retrievable conversation at every required stage + `InvalidArgument` when exchanges omitted. Plus handler/storage integration tests for approve-accept coupling, per-stage gate, and new CLI `--conversation` input.

### the agent's Discretion
- Exact proto field number for `ApproveRequest.conversation_exchanges` — enumerate against current `.proto`. **(RESOLVED by research — see Key Finding #1: field 3 already exists.)**
- Whether approve-accept validation reuses the reject-branch `ValidateExchanges(..., "approve")` call verbatim or refactors into a shared helper.
- Precise `--conversation` flag ergonomics (file vs `-` stdin parsing, error messages) and whether it shares a loader with existing JSON-exchange parsing.
- Exact wording of the corrected `author` tool `Description` and the skill edits.
- Whether removing the MCP `conversation` record action means deleting the action branch only or restructuring the `conversation` tool.

### Deferred Ideas (OUT OF SCOPE)
- **Conversation-coverage lint/query** (#906's third option) — not chosen; e2e + integration is the gate. Revisit as a future guardrail.
- **Making spark mandatory** — explicitly rejected (D-01); seed-only sparks valid.
- **Conversation editing / versioning / drift-on-conversations** — new capabilities, own phases.
- Spark mandatory; JIT display-name reconciliation (Phase 9); amend/supersede semantics (Phase 7, done).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CONV-01 (#906) | Running a spec through the authoring funnel via skills (shape/specify/decompose/approve) records a conversation for every stage; recording enforced by protocol, not agent discretion. | Server-side coupling for shape/specify/decompose already shipped (#910/#917). This phase closes the two remaining holes (approve-accept, CLI synthetic placeholder), removes the skippable MCP `record` action (root cause), and adds the MCP-only e2e gate. All enforcement points, reusable validators, and test seams are mapped below. |
</phase_requirements>

## Summary

This is a **close-the-holes refactor**, not a from-scratch build. The heavy lifting — unconditional `authoring.ValidateExchanges` + atomic `RecordConversation` inside the same stage transaction — already shipped for Shape/Specify/Decompose in PRs #910/#917/#924 (verified in `git log`). Phase 8 collapses the surface to a single enforced recording path by: (1) making the approve **accept** path validate+record exchanges symmetric with the already-working reject path; (2) deleting the CLI synthetic-placeholder and replacing it with a real `--conversation` input; (3) removing the agent-facing MCP `conversation record` action (the #906 root-cause skip surface) while keeping `list`; and (4) updating skills + the `author` tool description and adding an MCP-only e2e gate.

The single most important research finding corrects a discretion item and reduces scope: **`ApproveRequest.conversation_exchanges` already exists as proto field 3** (currently documented "REQUIRED when action == APPROVE_ACTION_REJECT" and consumed by the reject branch). D-02 is therefore **not** a new-field addition — it is a comment update plus wiring the accept branch to the field that already exists. No new generated struct field; `GetConversationExchanges()` is already present in `gen/`.

Every enforcement point flows through one validator (`authoring.ValidateExchanges`) and one storage method (`Store.RecordConversation`, wrapped in `RunInTransaction`). The reject branch of `Approve` is a verbatim template for the accept branch. The CLI already has a JSON-exchange loader pattern (`conversationRecordInput` + `loadJSONFileRaw`) to reuse for `--conversation`.

**Primary recommendation:** Mirror the existing reject-branch validate+record pattern into the accept branch; reuse `ValidateExchanges`/`RecordConversation` verbatim; treat the proto change as a comment-only edit on the existing field 3; delete `cliSyntheticExchanges` and add a `--conversation` loader modeled on `conversationRecordInput`; delete only the `record` branch of `conversationTool` (keep `list`); prune the four `TestConversationTool_Record*` unit tests; gate with an MCP-only Ginkgo e2e in `e2e/api/`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Enforce non-empty exchanges per stage | API / Backend (`internal/server/authoring_handler.go`) | Validation lib (`internal/authoring/validate.go`) | Enforcement must be protocol-level (CONV-01, D-03) — the handler is the single gate all surfaces (MCP + CLI) pass through; the validator is the shared contract. |
| Atomic conversation persistence | Database / Storage (`internal/storage/postgres/conversation.go`) | API / Backend (tx orchestration) | `RecordConversation` runs inside `RunInTransaction` (ADR-004) so a stage save + conversation commit or roll back together — a missing conversation cannot silently pass. |
| Proto contract (`ApproveRequest.conversation_exchanges`) | Proto (`proto/specgraph/v1/authoring.proto`) → `gen/` | — | Field already exists (#1); only doc/comment reflects the new accept requirement. |
| MCP agent recording path | MCP tool surface (`internal/mcp/tools_authoring.go`) | API / Backend (ConnectRPC client) | Inline-with-save via `author` tool is the ONLY agent path after D-06; `conversation record` action removed to eliminate the skip surface. |
| CLI human/backfill recording | CLI (`cmd/specgraph/`) | API / Backend | `conversation record` stays (D-08); stage commands get real `--conversation` input (D-05); synthetic placeholder deleted (D-04). |
| Conversation retrieval (criterion #4) | MCP `conversation list` + CLI `conversation list` + `ListConversations` RPC | Storage `ListConversations` | Retrieval unchanged; only the `record` action is removed from MCP. |

## Standard Stack

No new external dependencies. This phase edits existing Go packages only.

### Core (existing, in-repo)
| Component | Location | Purpose | Why Standard |
|-----------|----------|---------|--------------|
| `authoring.ValidateExchanges` | `internal/authoring/validate.go:42` | Non-empty + role∈{probe,response} + content + stage-match + strict-increasing-sequence gate | The single validation contract already used by shape/specify/decompose; returns a `*ValidationError` the handlers wrap as `InvalidArgument`. Reuse verbatim for approve-accept and the CLI loader. |
| `Store.RecordConversation` | `internal/storage/postgres/conversation.go:78` | Inserts `conversation_logs` row + wires AUTHORED_VIA / CONTINUES / EXPLAINS edges, all in `RunInTransaction` | ADR-004 transaction pattern; already the persistence primitive for every recording path. Keep (D-07). |
| `runInTxOrSequential` + `buildConversationEntry`/`exchangesFromProto` | `internal/server/authoring_handler.go` (`buildConversationEntry` :828, `exchangesFromProto` :811) | Atomic "transition → store output → record conversation" orchestration + proto→domain mappers | Extend the approve-accept `runInTxOrSequential` block to add the record op. |
| `parseOptionalExchanges` | `internal/mcp/tools_authoring.go:71` | Parses the MCP `exchanges` JSON param (isolated to prevent JSON injection) | Reuse in `handleApprove` to thread exchanges into `ApproveRequest` (D-02). |
| `conversationRecordInput` + `loadJSONFileRaw` | `cmd/specgraph/conversation.go:60`, `cmd/specgraph/util.go:37` | Loads a `{"exchanges":[...]}` JSON file into `[]*ConversationExchange` | Model for the new `--conversation` loader (D-05) — see Pitfall #3 re: array-vs-object shape. |

**Installation:** none — `task proto` regenerates `gen/` after the (comment-only) `.proto` edit; `task build` rebuilds the binary.

## Package Legitimacy Audit

Not applicable — this phase installs no external packages. All work is inside existing first-party Go packages and the committed `gen/` tree.

## Architecture Patterns

### System Architecture Diagram

```
                        ┌─────────────────────────────────────────────┐
   MCP agent            │  ONLY recording path after D-06:            │
   (author tool) ──────▶│  author action=shape|specify|decompose|     │
                        │  approve  WITH exchanges (inline)           │
                        └───────────────┬─────────────────────────────┘
                                        │ ConnectRPC (exchanges in same request)
   CLI (human)                          ▼
   shape/specify/    ──▶ ConnectRPC ──▶ AuthoringHandler.{Shape,Specify,
   decompose/approve     (--conversation) Decompose,Approve}  (internal/server)
   --conversation file                   │
                                         │ 1. authoring.ValidateExchanges(exchanges, stage)
                                         │      └─ empty/invalid ─▶ connect.CodeInvalidArgument  (HARD REJECT, D-03)
                                         ▼
                              runInTxOrSequential (RunInTransaction, ADR-004)
                                         │  ├─ TransitionStage / StoreOutput
                                         │  └─ Store.RecordConversation(slug, entry)
                                         ▼
                              PostgreSQL: conversation_logs row
                                         + AUTHORED_VIA / CONTINUES / EXPLAINS edges
                                         │  (commit-or-rollback together)
                                         ▼
   Retrieval (criterion #4):  ListConversations RPC ◀── MCP `conversation list` / CLI `conversation list`

   REMOVED (D-06): MCP `conversation record` action  ─╳─  (skip surface eliminated)
   REMOVED (D-04): cliSyntheticExchanges placeholder  ─╳─
   KEPT (D-07/D-08): Store.RecordConversation (export + inline) ; CLI `conversation record` (human backfill)
```

### Pattern 1: Reject branch is the verbatim template for the accept branch
**What:** The `Approve` reject branch (`authoring_handler.go:487–543`) already does exactly what the accept branch must: `authoring.ValidateExchanges(req.Msg.GetConversationExchanges(), "approve")` → `exchangesFromProto` → build a `ConversationLogEntry` with `Stage = storage.SpecStageApproved` → `store.RecordConversation(txCtx, slug, entry)` inside `runInTxOrSequential`.
**When to use:** Adding the accept-branch recording (D-02).
**Example (existing reject branch — mirror into accept):**
```go
// Source: internal/server/authoring_handler.go:487-531 (reject branch)
if err := authoring.ValidateExchanges(req.Msg.GetConversationExchanges(), "approve"); err != nil {
    return nil, connect.NewError(connect.CodeInvalidArgument, err)
}
exchanges := exchangesFromProto(req.Msg.GetConversationExchanges())
entry := storage.ConversationLogEntry{
    Exchanges:     exchanges,
    ExchangeCount: safeInt32(len(exchanges)),
    IsAmend:       false,
}
// ... inside runInTxOrSequential, after setting entry.Stage = storage.SpecStageApproved:
if _, err := store.RecordConversation(txCtx, slug, entry); err != nil {
    return fmt.Errorf("record conversation: %w", err)
}
```
For the **accept** branch, add validate-first (before the tx) and add the `RecordConversation` op into the existing `runInTxOrSequential` block at `:457–472` (which already holds `TransitionStage`, `acceptLinkedDecisions`, `GetSpec`). Set `entry.Stage = storage.SpecStageApproved`.

### Pattern 2: Inline-with-save MCP threading
**What:** shape/specify/decompose handlers (`handleShape` :230, `handleSpecify` :264, `handleDecompose` :298) call `parseOptionalExchanges(params)` and pass the result as `ConversationExchanges` on the stage request.
**When to use:** `handleApprove` (`:332`) currently sends only `Slug` — add the same `parseOptionalExchanges` + threading (D-02). Note: for approve, exchanges are **required**, so treat an empty parse result as an error (the server will also reject, but a client-side message is friendlier).

### Pattern 3: Single-source-of-truth path retirement (Phase 7 precedent)
**What:** Phase 7 retired divergent/skippable paths (amend/supersede route through one `LifecycleService` gate). D-06 applies the same philosophy: delete the `record` branch of `conversationTool.handle` (`tools_authoring.go:447`) and `handleRecord` (`:456–485`), leaving `list` as the only action.

### Anti-Patterns to Avoid
- **Adding a new proto field for approve exchanges.** Field 3 already exists (Key Finding #1). Adding a new field would orphan the reject branch or duplicate the contract.
- **Auto-synthesizing exchanges on any agent path.** D-03 forbids silent synthesis; the whole point of #906 is that fidelity beats convenience. The CLI placeholder deletion (D-04) is the concrete instance.
- **Recording outside the stage transaction.** A conversation recorded in a separate call can be skipped or can commit while the stage rolls back — this is the exact failure #906 documents. Keep it inside `runInTxOrSequential`.
- **Leaving `TestConversationTool_Record*` tests behind after removing the action.** They will fail to compile / assert on a removed branch. Prune them (see Validation Architecture).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Exchange validation (non-empty, roles, sequence) | A second validator in the accept branch or CLI | `authoring.ValidateExchanges(..., "approve")` (`validate.go:42`) | One contract for MCP + CLI + all stages; the reject branch already calls it for "approve". |
| Atomic record + edge wiring | Inline INSERTs in the handler | `Store.RecordConversation` (`conversation.go:78`) inside `RunInTransaction` | Handles AUTHORED_VIA/CONTINUES/EXPLAINS edge chain + version lookup; ADR-004 compliance. |
| JSON exchange parsing (MCP) | Ad-hoc `json.Unmarshal` into a map | `parseOptionalExchanges` (`tools_authoring.go:71`) | Parses in isolation (`{"exchanges":` wrapping) to prevent JSON injection — a security control already in place. |
| JSON exchange file loading (CLI) | New bespoke parser | `loadJSONFileRaw` + a struct like `conversationRecordInput` (`conversation.go:60`) | Existing loader; reuse keeps one validation contract. Reconcile array-vs-object shape (Pitfall #3). |

**Key insight:** Every reusable primitive already exists and is battle-tested by the shipped shape/specify/decompose path. This phase is wiring and deletion, not invention.

## Runtime State Inventory

> This is a refactor/removal phase (deletes CLI synthetic-placeholder + MCP record action, changes enforcement). No rename. Inventory below.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `conversation_logs` table already holds rows, including synthetic-placeholder rows previously stamped by `cliSyntheticExchanges` ("authored via specgraph CLI (no interactive dialogue)"). Schema (`stage`, `exchanges` JSONB, `exchange_count`, edges) is unchanged by this phase. | **None (no migration).** Existing rows remain valid; deleting the placeholder helper only affects *future* CLI writes. No backfill/cleanup in scope (coverage-lint was explicitly deferred). |
| Live service config | None — no external service holds authoring state. | None. |
| OS-registered state | None. | None. |
| Secrets/env vars | None referenced by the changed code. | None. |
| Build artifacts | `gen/specgraph/v1/*.pb.go` are committed and regenerated by `task proto`. The `.proto` edit (comment on existing field 3) triggers a regen; `task proto:check` gates staleness. `author` tool `Description` and `SKILL.md` embedded via `//go:embed` → rebuilt by `task build`. | Run `task proto` after the `.proto` edit; run `task build`. Commit regenerated `gen/`. |

**Nothing found in categories:** Live service config, OS-registered state, Secrets/env vars — verified: the changed files touch only in-process handlers, MCP tools, CLI commands, proto, and the embedded skill.

## Common Pitfalls

### Pitfall 1: Treating D-02 as a new proto field
**What goes wrong:** Adding `conversation_exchanges` as a new field (e.g., field 4) when field 3 already exists and is consumed by the reject branch.
**Why it happens:** CONTEXT.md D-02 says "add `conversation_exchanges`"; the field predates this phase.
**How to avoid:** The field is `repeated ConversationExchange conversation_exchanges = 3;` (`authoring.proto:360`). Only update its doc comment (currently "REQUIRED when action == APPROVE_ACTION_REJECT") to reflect that it's now required for accept too. Verify `GetConversationExchanges()` already exists in `gen/` (it does — the reject branch calls it).
**Warning signs:** `task proto` reports a new field number; the reject branch stops compiling.

### Pitfall 2: Approve-accept records under the wrong stage
**What goes wrong:** Recording the accept conversation under `decompose` (the current stage before transition) or leaving `entry.Stage` empty.
**Why it happens:** The transition from decompose→approved happens inside the same tx; it's ambiguous which stage the conversation belongs to.
**How to avoid:** Mirror the reject branch, which explicitly sets `entry.Stage = storage.SpecStageApproved` (`authoring_handler.go:517`) so the conversation is associated with the approval gate. Do the same in accept.
**Warning signs:** e2e `ListConversations` filtered by `stage="approve"` returns empty after a successful accept.

### Pitfall 3: CLI `--conversation` file shape mismatch (array vs object)
**What goes wrong:** D-05 specifies the `--conversation` file is a **JSON array** of `ConversationExchange` objects, but the existing CLI `conversation record` loader (`conversationRecordInput`) expects an **object** `{"exchanges":[...]}`, and the MCP `parseOptionalExchanges` also wraps with `{"exchanges":`. A copy-paste reuse will reject the array form (or vice-versa).
**Why it happens:** Three subtly different accepted shapes across surfaces.
**How to avoid:** Decide one shape for `--conversation` (D-05 says array) and write/adapt the loader to that shape explicitly. Either `json.Unmarshal` into `[]exchangeStruct` directly, or wrap before feeding a shared loader. Keep the MCP contract (array in the `exchanges` string param) as the canonical "same shape" reference D-05 cites. Document the chosen shape in the flag help and error message.
**Warning signs:** `--conversation` rejects a payload that the MCP `author exchanges` param accepts, breaking the "single validation contract" intent.

### Pitfall 4: Handler error assertions using message strings
**What goes wrong:** Integration tests assert on error *message* text; handler errors are sanitized (AGENTS.md), so messages are not a stable contract.
**Why it happens:** Natural instinct to assert on the human-readable reason.
**How to avoid:** Assert on `connect.CodeInvalidArgument` (per AGENTS.md convention). Mock backends must return sentinel errors (`storage.ErrSpecNotFound`, etc.), not `fmt.Errorf`, so `errors.Is` checks in the handler fire.
**Warning signs:** Tests break when an unrelated message wording changes.

### Pitfall 5: Skill token drift breaks `TestContentProtoDrift`
**What goes wrong:** Editing `SKILL.md` (D-09) introduces a backticked snake_case token that no longer matches a proto field, failing `TestContentProtoDrift`, or leaves stale "not needed for approve" text.
**Why it happens:** The skill currently says (verified) approve "does not require an `output` or `exchanges` on a clean acceptance" (`SKILL.md:176-177`) and the `author` tool `Description` says exchanges are "not needed for approve" (`tools_authoring.go:148`). Both must flip.
**How to avoid:** Flip both the `author` `Description` (`:118-129`, `:146-155`) and `SKILL.md` (`:66-67`, `:176-177`); run `task skills:validate` and the content-drift test after editing. Update any other "then call conversation record" framing.
**Warning signs:** `task check` fails on `TestContentProtoDrift` or `task skills:validate`.

## Code Examples

### Threading exchanges into approve (MCP handleApprove)
```go
// Source: mirror internal/mcp/tools_authoring.go:248-257 (handleShape) into handleApprove (:332)
exchanges, exErr := parseOptionalExchanges(params)
if exErr != nil {
    return exErr, nil
}
if len(exchanges) == 0 {
    return errResult("exchanges is required for approve (JSON array of ConversationExchange)"), nil
}
resp, err := t.client.Authoring.Approve(ctx, connect.NewRequest(&specv1.ApproveRequest{
    Slug:                  slug,
    ConversationExchanges: exchanges,
    // Action defaults to ACCEPT (APPROVE_ACTION_UNSPECIFIED → accept, server-side).
}))
```

### Removing the MCP record action (conversationTool.handle)
```go
// Source: internal/mcp/tools_authoring.go:444-454 — delete the "record" case + handleRecord (:456-485)
func (t *conversationTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
    action := stringParam(params, "action")
    switch action {
    case "list":
        return t.handleList(ctx, params)
    default:
        return errResult(fmt.Sprintf("unknown action %q — valid: list", action)), nil
    }
}
// Also update def() Description ("Record and list…" → "List…") and drop "record" from the action stringProp + the exchanges/stage/is_amend params.
```

### Existing RecordConversation transaction (do not re-implement)
```go
// Source: internal/storage/postgres/conversation.go:78-194 (abridged) — already atomic via RunInTransaction
func (s *Store) RecordConversation(ctx context.Context, slug string, entry storage.ConversationLogEntry) (*storage.ConversationLogEntry, error) {
    return result, s.RunInTransaction(ctx, func(txCtx context.Context) error {
        // verify spec + version; find changelog (EXPLAINS) + prev log (CONTINUES);
        // INSERT conversation_logs; wire AUTHORED_VIA (first) / CONTINUES / EXPLAINS edges.
    })
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Conversation recorded via a separate `conversation record` call agents could skip | Atomic inline recording inside the stage transaction for shape/specify/decompose | PRs #910/#917/#924 (verified in git log) | Root cause of #906 already mitigated for 3 of 4 required stages; Phase 8 extends to approve + removes the residual skip surface. |
| Approve accept records nothing; only reject records | Approve accept validates + records exchanges symmetric with reject | This phase (D-02) | Closes the last stage hole. |
| CLI stamps `cliSyntheticExchanges` placeholder to pass the gate | CLI supplies real exchanges via `--conversation` | This phase (D-04/D-05) | Fidelity over non-interactive convenience (user-accepted tradeoff). |
| MCP `conversation record` action available to agents | Action removed; inline-with-save only | This phase (D-06) | Eliminates the #906 skip surface entirely. |

**Deprecated/outdated (in current tree, to be changed this phase):**
- `author` tool `Description`: "shape/specify/decompose also require `exchanges` … not needed for approve" (`tools_authoring.go:148`) — flip for approve.
- `SKILL.md`: "**Approve** needs only the `slug` … does not require an `output` or `exchanges` on a clean acceptance" (`:176-177`) — flip.
- `cliSyntheticExchanges` (`authoring_cli_exchanges.go`) — delete.
- `conversationTool` `record` action + `handleRecord` — delete.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The approve **accept** path should record the conversation under `SpecStageApproved` (mirroring reject), not `decompose`. | Pitfall #2 | Low — reject branch sets this explicitly with a rationale comment; e2e will catch a wrong stage. Confirm desired stage label with the planner if criterion #1 wording ("entry for every stage") implies a distinct "approve" stage string vs the "approved" enum. |
| A2 | `--conversation` file format is a JSON array (not the `{"exchanges":[...]}` object the existing CLI record loader uses). | Pitfall #3 | Medium — D-05 explicitly says "JSON array of `ConversationExchange` objects", but the reused loader expects an object. Planner must pin the exact shape + loader adaptation; a mismatch breaks the "single contract" goal. |
| A3 | The proto comment-only change to field 3 still requires `task proto` + committing regenerated `gen/` (descriptor rawDesc embeds comments). | Runtime State Inventory / Pitfall #1 | Low — running `task proto` is harmless even if the Go struct is unchanged; `task proto:check` gates staleness in CI. |

**If any assumption is wrong:** these are the items the planner/discuss should confirm before locking tasks. A1 and A2 are the two that materially affect task shape.

## Open Questions

1. **Does criterion #1 "records a conversation entry for every stage" require a literal per-stage `stage` string of "approve", or is "approved" (the enum/stage used by the reject branch) acceptable?**
   - What we know: reject records under `storage.SpecStageApproved`; `ValidateExchanges(..., "approve")` uses "approve" as the exchange-level `stage` match target.
   - What's unclear: whether the stored `conversation_logs.stage` should be "approve" or "approved" for the accept path.
   - Recommendation: Match the reject branch exactly (`SpecStageApproved`) for consistency; note the exchange-level `stage` field validates against "approve". Confirm during planning if e2e assertions need a specific value.

2. **Should `handleApprove` reject empty exchanges client-side, or rely solely on the server's `InvalidArgument`?**
   - What we know: server will reject via `ValidateExchanges`; shape/specify use `parseOptionalExchanges` (which returns nil on absent, letting the server enforce).
   - Recommendation: Add a client-side friendly error for approve (required stage) for a better MCP agent message, but the server remains the authority. Low-risk either way.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | build/test everything | ✓ | 1.26.5 (`go.mod` `go 1.26.5`) | — |
| task (Taskfile) | `task proto`, `task build`, `task check`, `task pr-prep` | ✓ | 3.50.0 | — |
| buf | `task proto` (proto codegen) | ✓ | 1.71.0 | — |
| Docker | integration tests + e2e (`task pr-prep`, testcontainers `pgvector/pgvector:pg18`) | ✓ installed, **✗ not running** | 29.6.1 | none — must start Docker before running the D-10 integration/e2e gate |

**Missing dependencies with no fallback:**
- **Docker daemon is installed but not running.** Unit tests + `task check` (which excludes Postgres integration/Docker) run without it, but the D-10 verification gate — handler/storage integration tests and the MCP-only Ginkgo e2e in `e2e/api/` — **require Docker running** (testcontainers spins up `pgvector/pgvector:pg18`, wait strategy `ForLog("database system is ready").WithOccurrence(2)`). The planner must include "start Docker" as a precondition for the verification wave, and CI's `task pr-prep` path assumes Docker-in-Docker availability.

## Validation Architecture

> `.planning/config.json` has no `workflow.nyquist_validation` key → treated as enabled.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (unit) + `testify` (assert/require) for handler/CLI/MCP; Ginkgo/Gomega for e2e (`//go:build e2e`) |
| Config file | `Taskfile.yml` (targets); build tags `integration` / `e2e` gate suites |
| Quick run command | `task test` (`go test ./...`, skips integration + e2e) |
| Full suite command | `task pr-prep` (check → `task test:integration` → `task test:e2e`, requires Docker) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CONV-01 | Approve-accept validates + records exchanges; empty → InvalidArgument | integration (handler) | `go test -tags integration ./internal/server/...` | ⚠️ extend `internal/server/authoring_handler_test.go` |
| CONV-01 | `RecordConversation` under approved stage persists + is retrievable | integration (storage) | `go test -tags integration ./internal/storage/postgres/...` | ⚠️ extend `internal/storage/postgres/conversation_test.go` |
| CONV-01 | MCP `author approve` threads exchanges; `conversation` tool no longer has `record` | unit | `go test ./internal/mcp/...` | ⚠️ update `internal/mcp/tools_authoring_test.go`; **remove** `TestConversationTool_Record`, `_InvalidJSON`, `_MissingSlug`, `_MissingStage`, `_MissingExchanges` (`:676-773`) |
| CONV-01 | CLI stage commands require `--conversation`; synthetic placeholder gone | unit | `go test ./cmd/specgraph/...` | ⚠️ update `cmd/specgraph/authoring_test.go`; remove synthetic-placeholder tests |
| CONV-01 (D-10) | Full funnel via MCP `author` records + retrieves a non-empty conversation at every required stage; omitting exchanges → InvalidArgument | e2e | `go test -tags e2e ./e2e/api/...` | ⚠️ add to `e2e/api/mcp_only_authoring_test.go` (MCP-only harness `mcpProjectClient` exists) or a new `mcp_only_conversation_test.go` |

### Sampling Rate
- **Per task commit:** `task test` (unit; no Docker) + `task lint`.
- **Per wave merge:** `task test:integration` (Docker) for handler/storage waves.
- **Phase gate:** `task pr-prep` full green (check + integration + e2e) before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `e2e/api/mcp_only_conversation_test.go` (or an added `It` in `mcp_only_authoring_test.go`) — MCP-only full-funnel conversation fidelity (D-10). Harness `mcpProjectClient` (`mcp_only_authoring_test.go:48`) and existing `conversation_test.go` / `mcp_only_authoring_test.go` patterns are the template.
- [ ] Extend `internal/server/authoring_handler_test.go` — approve-accept validate+record + `InvalidArgument` on empty; assert on `connect.CodeInvalidArgument` (not messages).
- [ ] Extend `internal/storage/postgres/conversation_test.go` — approve-stage record retrievable.
- [ ] **Remove** `TestConversationTool_Record*` (4-5 tests, `tools_authoring_test.go:676+`) and any synthetic-placeholder CLI tests — required by D-04/D-06 removals (else compile/assert failure).
- [ ] Add CLI `--conversation` unit tests (file + `-` stdin; missing-flag error for required stages).
- Framework install: none — all frameworks present.

## Security Domain

> `.planning/config.json` has no `security_enforcement` key → treated as enabled.

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Unchanged; MCP/ConnectRPC auth + `scopeStore` project header already enforced upstream. |
| V3 Session Management | no | N/A. |
| V4 Access Control | yes (inherited) | All write paths go through `scopeStore(ctx, h.scoper)` (project-scoped); the new approve-accept record path inherits this — do not bypass it. |
| V5 Input Validation | **yes** | `authoring.ValidateExchanges` (role/content/sequence/length ≤ `MaxExchangeContentLen` 4096, ≤ `MaxConversationExchanges` 100). MCP `parseOptionalExchanges` and CLI loader must parse exchanges in isolation (`{"exchanges":` wrapping) to prevent JSON injection — the established pattern. |
| V6 Cryptography | no | N/A. |

### Known Threat Patterns for Go / ConnectRPC + MCP
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| JSON injection via the exchanges string param | Tampering | Parse in isolation via a single field wrapper (`parseOptionalExchanges` pattern) — never splice untrusted JSON into a larger document. |
| Error-message information disclosure | Information Disclosure | Handler `stageError`/sanitization already in place; tests assert error **codes**, not messages (AGENTS.md). |
| CLI file-path read of untrusted input | Tampering / Info Disclosure | `loadJSONFileRaw` is CLI-only (path is user-supplied local); do NOT reuse the CLI loader in any server/network path (documented in `util.go`). |
| Unbounded exchange volume/size (DoS) | Denial of Service | Existing caps: `MaxConversationExchanges = 100`, `MaxExchangeContentLen = 4096` in `validate.go` + `maxElements` guard in the RPC handler. Reuse — do not relax. |

## Sources

### Primary (HIGH confidence — verified against on-disk source this session)
- `internal/server/authoring_handler.go` (Approve accept :453-485 / reject :487-543; `RecordConversation` RPC :612; `buildConversationEntry` :828; `exchangesFromProto` :811) — enforcement point.
- `internal/authoring/validate.go` (`ValidateExchanges` :42; caps :12-17) — validation contract.
- `internal/storage/postgres/conversation.go` (`RecordConversation` :78-194) — atomic persistence + edges.
- `internal/mcp/tools_authoring.go` (`authorTool.def` Description :115-177; `handleShape`/`handleSpecify`/`handleDecompose` :230-330; `handleApprove` :332-344; `parseOptionalExchanges` :71; `conversationTool` :420-500) — MCP surface.
- `proto/specgraph/v1/authoring.proto` (`ApproveRequest` :352-361 incl. existing field 3; `ConversationExchange` :411-421; `RecordConversationRequest` :434-439) — proto contract.
- `cmd/specgraph/authoring_cli_exchanges.go` (`cliSyntheticExchanges`), `shape.go`/`specify.go`/`decompose.go`/`spark.go`/`approve.go`, `conversation.go` (`conversationRecordInput` :60), `util.go` (`loadJSONFile*` :37/:48) — CLI surface.
- `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md` (exchanges :146-177; approve :176-177) — skill text to flip.
- `e2e/api/mcp_only_authoring_test.go` (`mcpProjectClient` harness :48), `e2e/api/conversation_test.go` — e2e templates.
- `git log` — confirms #910/#917/#924 (and #952 author-tool exchanges pass-through) already landed.
- `internal/export/engine.go:588` — `RecordConversation` consumer (reason to keep, D-07).

### Secondary (MEDIUM confidence)
- `AGENTS.md` — repo conventions (proto/`gen/` committed, `RunInTransaction`/ADR-004, sanitized errors + code assertions, sentinel errors, DCO/SPDX, `TestContentProtoDrift`, testcontainers `pgvector:pg18`, Docker gates).
- `docs/superpowers/specs/2026-03-26-conversation-recording-wiring-design.md` + plan (cited by CONTEXT.md; the coupling design the current handler comments reference — not re-read this session, referenced via CONTEXT.md provenance).

### Tertiary (LOW confidence)
- None — all load-bearing claims verified against source.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all primitives are first-party and read verbatim.
- Architecture / enforcement points: HIGH — reject branch is a verified in-repo template for accept; proto field existence confirmed.
- Pitfalls: HIGH — each is grounded in a specific verified line (proto field 3, stage label, loader shape mismatch, sanitized errors, skill drift test).
- Assumptions (A1/A2): flagged MEDIUM — stage-label wording and `--conversation` file shape need a planner/discuss confirmation.

**Research date:** 2026-07-15
**Valid until:** 2026-08-14 (30 days — stable in-repo Go codebase; re-verify if new PRs touch `authoring_handler.go` / `tools_authoring.go` / `authoring.proto`).

## Project Constraints (from AGENTS.md)

- **Proto workflow:** edit `.proto` sources (not `gen/`); run `task proto` (incremental, fingerprinted); `gen/` is committed; verify with `task proto:check`. For field removal use `reserved` — not applicable here (no removal).
- **Transactions:** all multi-query write paths use `RunInTransaction` with `txCtx` (ADR-004). The approve-accept record op joins the existing `runInTxOrSequential` block.
- **Error handling:** handler errors sanitized (`stageError`); tests assert `connect.Code*`, not message strings. Mock backends return sentinel errors (`storage.Err*`) so `errors.Is` fires.
- **Licensing/commits:** Apache-2.0 SPDX header on all `.go`/`.proto`; DCO `Signed-off-by:` on every commit (`git commit -s`); conventional-commit messages (cog).
- **Linting:** new packages need a `// Package ...` doc comment (revive); run `task check` before push (pre-push hook enforces); `task check` = fmt → license → lint → build → unit (no Docker).
- **Skills:** `SKILL.md` edits gated by `task skills:validate` and `TestContentProtoDrift` (backticked snake_case token drift). Skills are embedded canonicals served via MCP — edit the embedded copy.
- **Testing tiers:** `go test ./...` skips `integration`/`e2e` tags; integration + e2e require Docker (testcontainers `pgvector/pgvector:pg18`). Use `./internal/storage/` (no ellipsis) to avoid pulling `postgres/` integration into the unit step.
