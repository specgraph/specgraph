# Requirements: SpecGraph — v0.14.0 Authoring Surface Correctness

**Defined:** 2026-07-13
**Core Value:** Specs stay live and queryable as a graph — with locked architectural decisions, drift detection, and a durable storage/query layer — so both humans and agent-based execution engines can trust the spec graph as ground truth instead of static, decaying markdown.

**Source:** Open GitHub backlog (issue-first). Each requirement carries its originating issue number for traceability. Version aligns GSD tracking to the cog-managed release line (v0.13.0 released → v0.14.0 next).

## v1 Requirements

Requirements for milestone v0.14.0. Each maps to a roadmap phase once `gsd-roadmapper` runs.

### MCP Authoring Surface

- [x] **MCP-01**: An agent in a fresh MCP-only project (created by `specgraph init` — `.mcp.json` + managed files, no source, no local CLI) can author the project constitution to completion using only `specgraph://prime` and the MCP-served skills, with no out-of-band CLI/YAML knowledge (#1002)

### Authoring Lifecycle

- [x] **LIFE-01**: A user can `amend` a spec while it is in flight (`>= approved` and `< done`) to send it back to authoring, and can `supersede` only a `done` spec — matching natural lifecycle semantics (#900)
- [x] **LIFE-02**: After a user amends a spec with `--re-entry <stage>`, they can immediately re-author at that stage (e.g. run `shape`) without hitting an `invalid stage transition` no-op (#899)

### Authoring Fidelity

- [ ] **CONV-01**: When a user runs a spec through the authoring funnel via skills (shape/specify/decompose/approve), the conversation is recorded for every stage — recording is enforced by the protocol, not left to agent discretion (#906)

### Identity

- [ ] **AUTH-06**: On each successful login, a JIT-provisioned user's `display_name` is reconciled against a usable `name`/`preferred_username` claim and updated when a better value becomes available, replacing a stale subject-hash fallback (#994)

## v2 Requirements

Deferred to a future release. Full catalog carried over from the beads migration lives in `.planning/milestones/v0.12.0-REQUIREMENTS.md` (§ v2 Requirements): REL-02, CFG-03..05, DRFT-02, DEC-01..04, HRNS-01..03, DX-01..02, SCALE-01..03, EXPL-01..05, INTG-02, UI-01..02.

## Out of Scope

Explicitly excluded from v0.14.0. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Fix Confluence comment polling pagination (`spgr-jwbj` / #901, INTG-01) | The Confluence poller/adapter code does not live in this repo (only stale references in `cmd/specgraph/nudge.go` and a test). Re-homed to backlog Phase 999.2; first step on promotion is to locate the owning repository. |
| Other open GitHub issues not in the priority hotlist (design/task/`priority::none` items) | Deferred backlog; not blocking. Parked for a later milestone. |

## Traceability

Populated by `gsd-roadmapper` during roadmap creation. See `.planning/ROADMAP.md` for full phase
detail (goals, success criteria, dependencies).

| Requirement | Issue | Phase | Status |
|-------------|-------|-------|--------|
| MCP-01 | #1002 | Phase 6 | Pending |
| LIFE-01 | #900 | Phase 7 | Pending |
| LIFE-02 | #899 | Phase 7 | Pending |
| CONV-01 | #906 | Phase 8 | Pending |
| AUTH-06 | #994 | Phase 9 | Pending |

**Coverage:**

- v1 requirements: 5 total
- Mapped to phases: 5/5 ✓
- Unmapped: 0

---
*Requirements defined: 2026-07-13*
*Last updated: 2026-07-13 during milestone v0.14.0 start — scope sourced from the open GitHub backlog priority hotlist (#1002 critical, #900/#899 high, #906/#994 medium); #901 kept out of scope (code not in this repo).*
