# Doc Ingest Synthesis Summary

**Mode:** new (bootstrap; no prior PROJECT.md/ROADMAP.md/REQUIREMENTS.md existed)
**Classifications consumed:** 50 / 50 (`.planning/intel/classifications/*.json`)
**Cycle detection:** run over the full cross_refs graph (50 nodes); one 2-node
cycle found (Identity Storage ↔ Identity Bootstrap & UX docs), assessed as a
benign companion-doc mutual reference and downgraded to WARNING rather than a
synthesis-halting BLOCKER — see `INGEST-CONFLICTS.md` for full reasoning. No
other cycles found; traversal completed well under the 50-hop cap.

## Doc counts by type

| Type | Count | Notes |
|---|---|---|
| ADR | 8 | 5 real Accepted (locked) in `docs/decisions/`; 3 historical drafts (not locked) in `docs/initial-design-session/` |
| SPEC | 41 | Design docs across `docs/plans/` and `docs/superpowers/specs/` |
| DOC | 1 | `docs/initial-design-session/specgraph-v1.0-draft-roadmap.md` |
| PRD | 0 | None in this batch — `requirements.md` is consequently empty |

## Decisions locked

5 locked decisions, all Accepted status, all from `docs/decisions/`:

- ADR-001 — Principle field naming (`statement`, not `principle`)
- ADR-002 — Stable ULID IDs + Murmur3-128 content hash (rejects content-addressable IDs)
- ADR-004 — Optimistic concurrency, transaction-wrapped write paths
- ADR-005 — No native Windows support (WSL required)
- ADR-006 — Spec provenance model (replaces SpecLifecycle)

3 additional ADR-type documents are historical/non-locked drafts, retained for
provenance but not authoritative — see `decisions.md`.

## Requirements extracted

0 (no PRD documents in this ingest batch). See `requirements.md` for the
rationale and a pointer to `constraints.md` as the practical requirements
source for this design-doc-driven corpus.

## Constraints

41 entries in `constraints.md`, one per SPEC document, organized by theme:
overall architecture (4), storage/transaction layer (3), decisions domain (1),
authoring funnel & personas (6), spec lifecycle (3), constitution (2), MCP
server & harness integration (7), CLI lifecycle & config (3), identity/authn/authz
(12), initial-design-session foundational scaffolding (1, partially superseded).

## Context topics

2 primary topics in `context.md`: the original v1.0-draft implementation
roadmap (superseded but historically framing), plus 7 cross-cutting lineage
notes compiled while reading the full corpus (storage-backend generations,
lifecycle-field removal, amend/supersede eligibility flip, documents
referenced-but-not-classified in this batch, harness/plugin delivery
generations, and the Identity epic's internal sequencing).

## Conflicts

- **0 blockers**
- **2 warnings** (competing/ambiguous — need user attention before routing):
  1. Draft ADR-003's decision-ID scheme (content-addressable hash) directly contradicts locked ADR-002 (stable ULIDs) — never formally reconciled in-source.
  2. A cross-ref cycle between two Identity-epic companion docs — assessed benign, both synthesized, flagged for human override authority.
- **9 auto-resolved / informational** (self-documented supersessions, stale-but-superseded field references, editorial self-annotations) — see `INGEST-CONFLICTS.md` for full detail on each.

Full detail: `../INGEST-CONFLICTS.md`

## Per-type intel files

- `decisions.md` — 8 ADR entries (5 locked + 3 historical/non-locked)
- `requirements.md` — empty (no PRDs); pointer to constraints.md
- `constraints.md` — 41 SPEC entries, themed
- `context.md` — 1 DOC entry + cross-cutting lineage notes

## Status for downstream (`gsd-roadmapper`)

**STATUS: AWAITING USER** — 2 warnings require explicit resolution/approval
before this intel is routed into PROJECT.md/REQUIREMENTS.md/ROADMAP.md
generation, per the doc-conflict-engine safety gate (WARNING requires
approve-revise-abort before writing destination files; it does not block
reading/using this intel for review). No BLOCKERs are present, so the
orchestrator MAY proceed once the two warnings are acknowledged.
