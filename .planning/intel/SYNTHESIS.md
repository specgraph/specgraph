# Doc Ingest Synthesis Summary

**Mode:** new (bootstrap; no prior PROJECT.md/ROADMAP.md/REQUIREMENTS.md existed)
**Classifications consumed:** 177 / 177 (`.planning/intel/classifications/*.json`)
**This synthesis supersedes** the prior 50-document-only round; all four per-type intel
files and the conflicts report were fully regenerated from the complete corpus, not merged
incrementally.

**Cycle detection:** run over the full cross_refs graph (177 nodes, DFS with three-color
marking), well under the 50-hop cap. Several citation cycles were found; all were
individually inspected and confirmed benign (index-hub page, companion plan/design mutual
references, one classifier self-reference artifact, one descriptive-name mutual reference
between two explicitly-sequenced epic-sibling docs). None gated synthesis — see
`INGEST-CONFLICTS.md` for full detail.

## Doc counts by type

| Type | Count | Notes |
|---|---|---|
| ADR | 14 | 5 domain-architecture (locked, `docs/decisions/`); 5 process/tooling (locked, `docs/superpowers/specs/`, release engineering & repo housekeeping — see INFO entry in conflicts report); 4 historical/proposed drafts (not locked) |
| SPEC | 63 | Design docs across `docs/plans/` and `docs/superpowers/specs/` |
| PRD | 2 | One docs/release-gate PRD, one external Confluence-template PRD; no scope overlap |
| DOC | 98 | Overwhelmingly implementation plans companion to the SPECs/ADRs above, plus 3 post-ship verification artifacts and a handful of foundational/tracking docs |

## Decisions locked

10 locked ADR-typed documents, split into two populations (see `decisions.md` and the
process-vs-domain INFO entry in `INGEST-CONFLICTS.md`):

**5 domain-architecture ADRs** (`docs/decisions/`, load-bearing for the spec-graph model):
- ADR-001 — Principle field naming (`statement`, not `principle`)
- ADR-002 — Stable ULID IDs + Murmur3-128 content hash (rejects content-addressable IDs)
- ADR-004 — Optimistic concurrency, transaction-wrapped write paths
- ADR-005 — No native Windows support (WSL required)
- ADR-006 — Spec provenance model (replaces SpecLifecycle)

**5 process/tooling ADRs** (`docs/superpowers/specs/`, release engineering & repo housekeeping):
- Release Infrastructure: release-please + goreleaser + cocogito — **superseded**
- Release Tooling Migration: git-cliff + goreleaser v2 — **superseded**
- Release Pipeline: Single-Job, GoReleaser-Owns-Release — **currently active**
- Repository Organization Move
- Idempotent Push: FindOrCreate for Sync Adapters

4 additional ADR-type documents are historical/non-locked drafts or a design-adjustment note,
retained for provenance but not authoritative — see `decisions.md`.

## Requirements extracted

2 PRDs, 2 `REQ-*` entries (`REQ-quickstart-docs-overhaul`, `REQ-confluence-specgraph-design-bridge`).
No competing acceptance-criteria variants — see `requirements.md`. Note: this corpus remains
overwhelmingly design-doc-driven; `constraints.md` and `decisions.md` carry most of the
functional/behavioral intent for the core product, while these two PRDs cover narrower
documentation/external-template scope.

## Constraints

63 entries in `constraints.md`, one per SPEC document, organized by theme: overall
architecture & storage backend (5), storage transaction/consistency layer (4), decisions
domain (1), authoring funnel & personas (8), spec lifecycle (3), constitution (2), MCP
server & harness integration (12), CLI lifecycle & config (7), web UI (4), data
lifecycle/operations (3), identity/authn/authz (13), initial-design-session foundational
scaffolding (1, partially superseded).

## Context topics

98 DOC entries in `context.md`, grouped to mirror the constraints.md themes (foundational
roadmap/tracking, vertical-slice plans, storage/E2E plans, authoring/decisions plans,
lifecycle plans, constitution plans, MCP/harness plans, CLI/config plans, web UI plans,
data-ops plans, identity plans, release-engineering plans, site-docs plans, and 3 post-ship
verification artifacts), plus 13 cross-cutting lineage notes compiled while reading the full
corpus (storage-backend generations, lifecycle-field removal, amend/supersede eligibility
flip, harness/plugin delivery generations, the Identity epic's internal sequencing, and the
citation-cycle assessment).

## Conflicts

- **0 blockers**
- **0 warnings**
- **14 auto-resolved / informational** — self-documented supersessions, ADR-over-lower-precedence
  auto-resolutions, benign citation cycles, and sequencing/awareness notes. Two items the
  prior 50-document round flagged as WARNING (draft-ADR-003 vs ADR-002 on Decision-ID
  scheme; the Identity Storage ↔ Bootstrap-UX cross-ref cycle) are reclassified to INFO on
  the full corpus — both resolve cleanly via standard precedence with no genuine ambiguity
  requiring a user pick.

Full detail: `../INGEST-CONFLICTS.md`

## Per-type intel files

- `decisions.md` — 14 ADR entries (5 domain-locked + 5 process-locked + 4 historical/non-locked)
- `requirements.md` — 2 PRD entries (`REQ-quickstart-docs-overhaul`, `REQ-confluence-specgraph-design-bridge`)
- `constraints.md` — 63 SPEC entries, themed
- `context.md` — 98 DOC entries, themed, plus cross-cutting lineage notes

## Status for downstream (`gsd-roadmapper`)

**STATUS: READY** — no BLOCKERs and no WARNINGs are present, so this intel may be routed
into `PROJECT.md`/`REQUIREMENTS.md`/`ROADMAP.md` generation without further user gating,
per the doc-conflict-engine safety gate. Downstream planning should still read the INFO
entries in `INGEST-CONFLICTS.md` before drafting the roadmap, in particular:
- The process-vs-domain ADR split (10 locked ADRs total, only 5 govern the domain model).
- The three-generation storage-backend lineage (pure Postgres, generation 3, is authoritative).
- The `lifecycle` → `SpecProvenance` field replacement (ADR-006 supersedes all earlier schema mentions).
- The three-generation harness/plugin delivery lineage (embed-and-write, generation 3, is authoritative).
