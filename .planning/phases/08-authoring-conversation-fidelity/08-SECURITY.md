---
phase: 08
slug: authoring-conversation-fidelity
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-15
---

# Phase 08 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| MCP/CLI client → ConnectRPC Approve handler | Untrusted `conversation_exchanges` payload crosses into server validation/persistence | Conversation exchange text (untrusted) |
| Handler → PostgreSQL (RunInTransaction) | Domain conversation entry crosses into durable storage | Conversation entry (validated) |
| MCP agent → author/conversation tool params | Untrusted `exchanges` string param crosses into JSON parsing before ConnectRPC | JSON string (untrusted) |
| Local user → CLI `--conversation` file/stdin | User-supplied local file path / stdin content crosses into JSON parsing then ConnectRPC | JSON array file / stdin (local, untrusted) |
| e2e MCP client → in-process server → handlers → testcontainers Postgres | Test-only; exercises the same scopeStore/validation path as production | Test fixtures |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-08-01 | Tampering / DoS | `conversation_exchanges` volume/size on Approve | medium | mitigate | `authoring.ValidateExchanges(...,"approve")` reuses caps (MaxConversationExchanges=100, MaxExchangeContentLen=4096) — `internal/server/authoring_handler.go:478,533,692` | closed |
| T-08-02 | Information Disclosure | Approve handler error surface | low | mitigate | Tx errors routed through `h.stageError(ctx, err)` sanitization; tests assert `connect.Code*` not messages | closed |
| T-08-03 | Elevation / Access Control | New accept record path | medium | mitigate | Accept branch inherits `scopeStore(ctx, h.scoper)` project scoping — `authoring_handler.go:459` | closed |
| T-08-04 | Tampering | Partial-write (stage approved but no conversation) | high | mitigate | `RecordConversation` op runs inside the accept `runInTxOrSequential` (ADR-004); approval + conversation commit or roll back together — `authoring_handler.go:489`. Rollback proven by `TestAuthoringHandler_Approve_AcceptRecordConversationFailureRollsBack` | closed |
| T-08-05 | Tampering | JSON injection via `exchanges` string param in handleApprove | medium | mitigate | `parseOptionalExchanges` wraps the raw value as a single `{"exchanges":…}` field; never splices untrusted JSON into a larger document — `internal/mcp/tools_authoring.go:71` | closed |
| T-08-06 | Repudiation | Removing the standalone record action | low | accept | `list` retrieval + storage method retained (D-06/D-07); inline recording is now the only agent path, increasing auditability. Accepted as an improvement. | closed |
| T-08-07 | Information Disclosure | MCP error results | low | mitigate | Errors returned via `errResult`/`connectErrResult` sanitization; no raw parser internals surfaced | closed |
| T-08-08 | Tampering / Info Disclosure | CLI file-path read of `--conversation` input | low | mitigate | `loadJSONFileRaw` used CLI-only (local user value); not reused in any server/network path per the `util.go` security note | closed |
| T-08-09 | DoS | Oversized `--conversation` payload | low | accept | Server-side `ValidateExchanges` caps reject oversized input at the enforcement point. **Additionally mitigated** during code review (WR-02): `readBoundedConversation` caps stdin/file input to 1 MiB + early 100-exchange count check — `cmd/specgraph/conversation_flag.go:42,48` | closed |
| T-08-10 | Tampering | Malformed JSON array | low | mitigate | `loadConversationFlag` returns a wrapped parse error; command aborts before sending. Array-only contract documented in flag help + tested | closed |
| T-08-11 | Tampering (test integrity) | Cross-test project-state bleed | low | mitigate | Dedicated per-Describe project slug + isolated `mcpConvProjectClient` — `e2e/api/mcp_only_conversation_test.go:37` | closed |
| T-08-12 | Repudiation | Missing conversation passing silently | high | mitigate | Negative spec asserts `res.IsError` / `InvalidArgument` when exchanges omitted — the backstop that a missing conversation cannot silently pass — `e2e/api/mcp_only_conversation_test.go:156,183` (Docker-gated; runtime confirmation via CI) | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-08-01 | T-08-06 | Standalone MCP `conversation record` action removed; `list` retrieval and storage method retained. Inline-with-save is now the only recording path, which increases auditability. | Phase 08 plan (D-06/D-07) | 2026-07-15 |
| AR-08-02 | T-08-09 | CLI is a thin local client; server-side `ValidateExchanges` caps are the enforcement point. (Note: a client-side 1 MiB / 100-exchange bound was subsequently added in code review, so this risk is now also actively mitigated.) | Phase 08 plan (D-05) | 2026-07-15 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-15 | 12 | 12 | 0 | gsd-secure-phase (L1 grep-depth, register authored at plan time) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-15
