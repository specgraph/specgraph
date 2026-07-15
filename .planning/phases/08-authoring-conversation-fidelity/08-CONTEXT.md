# Phase 8: Authoring Conversation Fidelity - Context

**Gathered:** 2026-07-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Make conversation recording a reliable, protocol-enforced property of the authoring funnel: every stage that should have a conversation records a **non-empty** one atomically as part of the stage save, so a missing conversation cannot silently pass and recording is never left to agent discretion.

**Requirement (fixed by ROADMAP.md — this discussion clarifies HOW, not WHAT):**
- **CONV-01 (#906):** Running a spec through the authoring funnel via skills (shape/specify/decompose/approve) records a conversation for every stage; recording is enforced by the protocol/handler, not a "should do" step agents skip.

**Success criteria (from ROADMAP.md):**
1. Funnel via skills records a conversation entry for every stage.
2. Recording is enforced by the stage handler, not by the agent choosing to record.
3. A completed stage has an associated, non-empty conversation record — a missing conversation cannot silently pass.
4. Recorded conversations are retrievable/queryable after the funnel completes.

**Key discovery — most of the server-side coupling already shipped.** The
"conversation-recording-wiring" work (PRs #910, #917, #924) already made
Shape/Specify/Decompose require non-empty exchanges (`authoring.ValidateExchanges`
called unconditionally → `InvalidArgument` on empty) and record the conversation
**atomically inside the same stage transaction** (`internal/server/authoring_handler.go`).
The #906 demo failures predate this wiring. **This phase closes the remaining
holes and collapses to a single enforced path** — it is NOT a from-scratch build.

**In scope:** close the approve-accept hole (require exchanges symmetric with the
reject path — proto + handler + MCP + CLI); remove the CLI synthetic-placeholder
escape hatch and give the CLI a real exchanges input; retire the agent-facing
standalone MCP record action so inline-with-save is the only MCP recording path;
update skills + the `author` tool description to teach the corrected model;
MCP-only funnel e2e + integration verification.

**Out of scope (new capabilities / other phases):** spark being made mandatory
(stays optional — see D-01); conversation editing/versioning; richer conversation
analytics or drift on conversations; amend/supersede semantics (Phase 7, done);
JIT display-name reconciliation (Phase 9).

</domain>

<decisions>
## Implementation Decisions

### Stage coverage
- **D-01:** **Required stages = shape, specify, decompose, and approve (accept).** Shape/specify/decompose already enforce non-empty exchanges. **Spark stays OPTIONAL** — a spark may be a bare `--seed` with no LLM dialogue, and criteria #1 does not list spark. Keep the existing conditional spark behavior (record only if exchanges present).
- **D-02:** **The approve ACCEPT path must record a non-empty conversation, symmetric with the existing reject path.** Today only reject records; accept records nothing. Add a `conversation_exchanges` field to the approve accept action and **require** it (non-empty), validated via `authoring.ValidateExchanges(..., "approve")`, recorded atomically in the approve transaction (same pattern as the reject branch already uses).
  - Requires: proto change to `ApproveRequest` (add `conversation_exchanges`); `task proto`; MCP `author` tool `approve` action must accept + thread `exchanges`; the tool `Description` (currently says approve needs no exchanges) must be corrected.

### Enforcement + failure mode
- **D-03:** **Enforcement is a hard reject** — a required stage with missing/empty exchanges fails with `InvalidArgument` (the existing shape/specify/decompose behavior). No silent auto-synthesis on any agent-driven path. This extends uniformly to the approve-accept path.
- **D-04:** **Remove the CLI synthetic-placeholder escape hatch.** Delete `cliSyntheticExchanges` (`cmd/specgraph/authoring_cli_exchanges.go`) and its use in the CLI stage commands. Real exchanges are required everywhere — the CLI no longer stamps a fake "authored via CLI" marker to pass the gate. (User explicitly accepted the tradeoff that non-interactive CLI authoring must now supply real exchanges.)
- **D-05:** **CLI supplies real exchanges via a `--conversation <file.json>` flag (and `--conversation -` for stdin).** The file is a JSON array of `ConversationExchange` objects — the same shape the MCP `author` tool accepts — so there is a single validation contract. Stage commands error if the flag is missing/empty for a required stage. Applies to shape/specify/decompose/approve; spark's `--conversation` is optional (D-01).

### Standalone record path
- **D-06:** **Retire the agent-facing standalone MCP record action.** Remove the MCP `conversation` tool's `record` action so an MCP agent's ONLY way to record is inline with the stage save (removes the skip surface #906 identified as root cause). **Keep the tool's `list` action** — agents still need it to satisfy criterion #4 (retrieval).
- **D-07:** **Keep the storage-level `Store.RecordConversation` method** — it is consumed by the export/import engine (`internal/export/engine.go`) and by the inline stage-handler recording. Do not remove it.
- **D-08:** **Keep the CLI `conversation record` command** for manual/backfill/out-of-band recording by humans. It is not the agent authoring path and is not the #906 skip surface.

### Skills & documentation
- **D-09:** **Update the MCP-first skills** (`specgraph-authoring`, and the approve guidance) so exchanges are taught as inline-with-save only, and so approve is taught as **now requiring** exchanges on clean acceptance (the current skill text says a clean acceptance needs no exchanges — flip it). Remove any lingering "then call conversation record" framing. `TestContentProtoDrift`-style checks gate backticked token drift.

### Verification
- **D-10:** **Verification gate = MCP-only funnel e2e + integration tests.** An MCP-only Ginkgo e2e (`e2e/api/`, Phase 6/7 style) drives a spec through the full funnel via the `author` tool and asserts a non-empty, retrievable conversation at every required stage (shape/specify/decompose/approve) plus correct `InvalidArgument` rejection when exchanges are omitted. Add handler/storage integration tests for the approve-accept coupling, the per-stage required-exchanges gate, and the new CLI `--conversation` input (placeholder removed).

### the agent's Discretion
- Exact proto field number for `ApproveRequest.conversation_exchanges` — enumerate against the current `.proto`.
- Whether the approve-accept exchanges validation reuses the existing reject-branch `ValidateExchanges(..., "approve")` call verbatim or is refactored into a shared helper.
- Precise `--conversation` flag ergonomics (file vs `-` stdin parsing, error messages) and whether it shares a loader with any existing JSON-exchange parsing.
- Exact wording of the corrected `author` tool `Description` and the skill edits.
- Whether removing the MCP `conversation` record action means deleting the action branch only or restructuring the `conversation` tool.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Driving requirement
- GitHub issue **#906** (`https://github.com/specgraph/specgraph/issues/906`) — root cause (skills teach a skippable separate `conversation record` step), evidence (Sonnet sessions with zero conversations), and the three remediation options (server-side / skill-side / validation).
- `.planning/ROADMAP.md` § "Phase 8: Authoring Conversation Fidelity" — locked goal + 4 success criteria.
- `.planning/REQUIREMENTS.md` § Authoring (CONV-01).

### Prior design (the coupling that already landed — reuse, do not reinvent)
- `docs/superpowers/specs/2026-03-26-conversation-recording-wiring-design.md` — the "§Conversation Recording Coupling" / "§Approve/spark coupling table" / "§Posture" design the current handler comments cite. Read before changing coupling.
- `docs/superpowers/plans/2026-03-26-conversation-recording-wiring.md` — the implementation plan (phase A #910 atomic recording, phase B #917 server-side polish, #924 composer).

### Server-side stage handlers (the enforcement point)
- `internal/server/authoring_handler.go` — `Shape`/`Specify`/`Decompose` (unconditional `ValidateExchanges` + atomic `RecordConversation` in `runInTxOrSequential`), `Spark` (conditional — leave optional per D-01), `Approve` (accept path records nothing today → add exchanges per D-02; reject path already records — mirror it), `buildConversationEntry`, `exchangesFromProto`, `stageError`.
- `internal/authoring/validate.go` — `ValidateExchanges` (rejects empty, validates role/content/sequence). The single validation contract for CLI + MCP.

### MCP surface
- `internal/mcp/tools_authoring.go` — `authorTool.def()` (tool `Description` + `approve` needs an `exchanges` param per D-02; `parseOptionalExchanges`), `handleApprove` (thread exchanges), and the `conversationTool` (~line 416; remove the `record` action per D-06, keep `list`).
- `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md` — MCP-first authoring skill; update approve/exchanges teaching per D-09.

### CLI surface
- `cmd/specgraph/authoring_cli_exchanges.go` — `cliSyntheticExchanges` (DELETE per D-04).
- `cmd/specgraph/shape.go`, `specify.go`, `decompose.go`, `approve.go`, `spark.go` — stage commands; replace synthetic stamp with `--conversation` file/stdin loader (D-05).
- `cmd/specgraph/conversation.go` — CLI `conversation record` command (KEEP per D-08); reuses `RecordConversation` RPC.

### Proto
- `proto/specgraph/v1/authoring.proto` — `ApproveRequest` (add `conversation_exchanges`, D-02); `ConversationExchange`, `RecordConversationRequest`. Regenerate with `task proto` (`gen/` is committed).

### Storage + export
- `internal/storage/postgres/conversation.go` — `Store.RecordConversation` (KEEP, D-07; AUTHORED_VIA / CONTINUES / EXPLAINS edge wiring), `ListConversations`, `ListAllConversations`.
- `internal/storage/conversation.go` — `ConversationLogEntry`, `ConversationExchange` domain types.
- `internal/export/engine.go` — consumes `RecordConversation` for import round-trip (reason to keep the storage method).

### Verification
- `e2e/api/` — Ginkgo/Gomega MCP-only e2e suite (`go test -tags e2e`); home for the full-funnel conversation e2e (D-10).
- `internal/server/authoring_handler_test.go`, `internal/storage/postgres/conversation_test.go`, `cmd/specgraph/conversation_test.go`, `cmd/specgraph/authoring_test.go` — existing patterns to extend / prune (remove synthetic-placeholder tests).

### Repo conventions (AGENTS.md)
- Proto removal/addition + `task proto` (`gen/` committed). All multi-query writes use `RunInTransaction` (ADR-004). Handler errors sanitized; test assertions use error codes (`connect.CodeInvalidArgument`), not message strings. Mock backends return sentinel errors. Apache-2.0 SPDX headers + DCO sign-off. Package doc comments required (revive). `TestContentProtoDrift` gates skill backticked snake_case tokens.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`authoring.ValidateExchanges`** (`internal/authoring/validate.go`) — the non-empty + role/content/sequence gate already used by shape/specify/decompose. Reuse verbatim for the approve-accept path (D-02) and the CLI `--conversation` loader (D-05) so there is one contract.
- **Reject-path recording in `Approve`** (`authoring_handler.go`) — already validates + records exchanges atomically under `SpecStageApproved`. The accept path can mirror this exact pattern.
- **`runInTxOrSequential` + `buildConversationEntry`** — the atomic "transition → store output → record conversation" pattern; extend the approve accept transaction to add the record op.
- **MCP `parseOptionalExchanges` + inline threading** — the shape/specify/decompose handlers show how to add `exchanges` to the approve action.

### Established Patterns
- Stage handlers enforce required exchanges via unconditional `ValidateExchanges` returning `InvalidArgument`; conversation persisted in the same `RunInTransaction`. Approve-accept must join this pattern.
- CLI stage commands currently build the request and stamp `cliSyntheticExchanges`; the `--conversation` loader replaces the stamp at that same seam.
- Phase 7 precedent for "retire the divergent/skippable path" (single source of truth) — D-06 applies the same philosophy to the standalone MCP record action.

### Integration Points
- `ApproveRequest.conversation_exchanges` (new) → MCP `handleApprove` / CLI `approve` → handler `ValidateExchanges` → `RecordConversation` → conversation_logs (+ edges). No storage-schema change (conversation_logs already carries stage/exchanges).
- CLI `--conversation` file → JSON array of `ConversationExchange` → same proto messages the MCP tool sends → same server validation.
- Removing the MCP `conversation` record action leaves `list` (retrieval, criterion #4) and the storage method (export) intact.

</code_context>

<specifics>
## Specific Ideas

- Milestone throughline (from Phases 6–7): make the MCP surface **impossible for an MCP-only agent to get wrong**. Retiring the standalone record action (D-06) and hard-requiring exchanges inline (D-03) serve that goal directly — there is no "forgot to record" path left.
- The user explicitly chose the **strict** options at every fork: require real exchanges everywhere (no synthetic placeholder), require approve-accept exchanges symmetric with reject, and retire the agent-facing standalone action. Fidelity is favored over non-interactive convenience.
- Spark is the deliberate exception: seed-only sparks with no dialogue stay valid (D-01).

</specifics>

<deferred>
## Deferred Ideas

- **Conversation-coverage lint/query** ("flag any completed spec with a stage output but no conversation" — #906's third option) — considered for the verification bar but not chosen for this phase; e2e + integration is the gate. Revisit as an ongoing guardrail in a future quality/observability phase if drift recurs.
- **Making spark mandatory** — explicitly rejected (D-01); seed-only sparks are valid.
- **Conversation editing / versioning / drift-on-conversations** — new capabilities, own phases.

None outside these — discussion stayed within phase scope.

</deferred>

---

*Phase: 8-Authoring Conversation Fidelity*
*Context gathered: 2026-07-15*
