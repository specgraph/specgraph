# Phase 8: Authoring Conversation Fidelity - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-15
**Phase:** 8-Authoring Conversation Fidelity
**Areas discussed:** Stage coverage, Enforcement + failure mode, Retire standalone record path, Skills + verification

---

## Stage coverage (which stages MUST record)

| Option | Description | Selected |
|--------|-------------|----------|
| Shape/specify/decompose + approve; spark stays optional | Enforce non-empty for shape/specify/decompose (done) + approve-accept; spark optional (seed-only allowed) | ✓ |
| All five stages required (incl. spark) | Require conversation at every stage including spark | |
| Just fix approve-accept | Only close the approve-accept hole | |

**User's choice:** Shape/specify/decompose + approve; spark stays optional.

### Approve accept-path conversation source

| Option | Description | Selected |
|--------|-------------|----------|
| Require exchanges on accept (like reject) | Add `conversation_exchanges` to approve accept + require non-empty, mirror reject path (proto + MCP + CLI) | ✓ |
| Auto-synthesize approval marker | Server stamps a minimal marker if none provided | |
| Require for MCP, synthetic for CLI | Hard bar for MCP, placeholder for CLI | |

**User's choice:** Require exchanges on accept, symmetric with reject.

---

## Enforcement + failure mode (CLI placeholder policy)

| Option | Description | Selected |
|--------|-------------|----------|
| Keep + extend CLI synthetic placeholder | Keep `cliSyntheticExchanges`, extend to approve/spark | |
| No placeholder — require real exchanges everywhere | Drop synthetic placeholder; require real exchanges including CLI | ✓ |
| Opt-in placeholder via explicit flag | Placeholder only when a `--no-conversation` flag is passed | |

**User's choice:** No placeholder — require real exchanges everywhere (accepted the tradeoff of breaking silent non-interactive CLI authoring).

### CLI exchanges input mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| `--conversation` JSON file/stdin flag | Load a JSON array of ConversationExchange (same shape as MCP); error if missing/empty | ✓ |
| Interactive TTY prompt | Prompt for probe/response pairs in a TTY | |
| Flag + interactive fallback | Both file/stdin and TTY prompt | |

**User's choice:** `--conversation <file.json>` (and `-` for stdin).

---

## Retire the standalone record path

| Option | Description | Selected |
|--------|-------------|----------|
| Retire MCP standalone action; keep storage method + CLI | Remove MCP `conversation` record action (keep `list`); keep storage method (export) + CLI command | ✓ |
| Retire MCP action AND CLI command | Remove both agent + human standalone recording surfaces | |
| Keep all, rely on skills only | No code change; steer via skills | |

**User's choice:** Retire the agent-facing MCP record action; keep the storage-level method (export/import) and the CLI `conversation record` command (manual/backfill).

---

## Skills + verification gate

| Option | Description | Selected |
|--------|-------------|----------|
| MCP-only funnel e2e + integration tests | Full-funnel MCP e2e asserting non-empty retrievable conversation per required stage + integration tests | ✓ |
| Integration tests only | Handler/storage tests, no full e2e | |
| e2e + integration + coverage lint | Add a queryable conversation-coverage check | |

**User's choice:** MCP-only funnel e2e + integration tests (skills also updated to teach inline-only + approve-now-requires-exchanges).

---

## the agent's Discretion

- Exact proto field number for `ApproveRequest.conversation_exchanges`.
- Whether approve-accept reuses the reject-branch `ValidateExchanges` call or a shared helper.
- `--conversation` flag ergonomics (file vs stdin parsing, error messages, shared loader).
- Exact wording of the corrected `author` tool description and skill edits.
- Deleting only the MCP `conversation` record action branch vs. restructuring the tool.

## Deferred Ideas

- Conversation-coverage lint/query (#906 option 3) — deferred to a future quality phase.
- Making spark mandatory — explicitly rejected.
- Conversation editing/versioning/drift-on-conversations — new capabilities, own phases.
