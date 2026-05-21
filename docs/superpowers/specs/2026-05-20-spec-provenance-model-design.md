<!--
SPDX-License-Identifier: Apache-2.0
-->

# Spec Provenance Model — Design

**Date:** 2026-05-20
**Status:** Draft — pending SpecGraph spec authoring
**Supersedes:** Abandoned spec `specgraph-retroactive-lifecycle`; parked Spark-stage spec `specgraph-spec-provenance`

## Context

Two discoveries during attempted spec work surfaced a model coherence issue:

1. **`GetReady` implementation gap.** The query in `internal/storage/postgres/graph.go` filters only on `stage <> 'done'`. It does not consult `lifecycle`. As a result, `lifecycle=living` specs incorrectly surface in `specgraph ready` despite the original design intent (per `docs/plans/2026-03-07-slice-5-spec-lifecycle-revised-plan.md`).

2. **Conceptual fuzziness between `task/living/done`.** Investigation revealed the lifecycle field tries to encode two orthogonal axes — "how does this spec relate to executed work" and "where in the funnel content is this" — and does both badly. After a TASK reaches `done`, it serves the same role as a LIVING spec: drift target, dependency anchor, contract under amendment. The distinction is vestigial post-`done`.

The original framing (`specgraph-retroactive-lifecycle`, since abandoned) proposed adding `SPEC_LIFECYCLE_RETROACTIVE` to solve the retroactive-spec use case. Brainstorming exposed that this would have created a third lifecycle behaviorally identical to LIVING, distinguished only by provenance — confirming the field itself is the problem.

## Goals

- **Replace** the `SpecLifecycle` enum with a single `SpecProvenance` field that records how a spec entered the graph
- **Fix** the `specgraph ready` query to accurately surface "approved, unclaimed, dependency-satisfied" work only
- **Unify** the post-`done` behavior across all specs (drift, dependency anchoring) regardless of how they got there
- **Enable** retroactive-from-PR and human-declared spec creation paths cleanly
- **Establish** clean break: no transitional flags, no backwards compatibility, no field deprecation tail

## Non-Goals

- No spec-to-code drift detection (existing spec-to-spec drift mechanism stays as-is)
- No automatic provenance inference (callers set explicitly)
- No new MCP tool or RPC — extend existing `spec create/update` paths
- No SvelteKit / web UI changes
- No migration tooling for external consumers (clean break per project status)
- No Memgraph backend changes (postgres is the active backend)

## The Model

### Provenance enum (proto)

```proto
enum SpecProvenance {
  SPEC_PROVENANCE_UNSPECIFIED         = 0;
  SPEC_PROVENANCE_AUTHORED            = 1;  // Forward-authored via the funnel
  SPEC_PROVENANCE_RETROACTIVE_FROM_PR = 2;  // Imported from a merged PR/commit
  SPEC_PROVENANCE_DECLARED            = 3;  // Human-declared as describing existing reality
}

message AuthoredProvenance {
  // Empty — audit trail lives in stage outputs and conversation log.
}

message RetroactiveFromPrProvenance {
  string url       = 1;  // PR URL
  string sha       = 2;  // merge commit SHA
  google.protobuf.Timestamp merged_at = 3;
  string title     = 4;  // PR title at import time
}

message DeclaredProvenance {
  string declared_by = 1;  // Human or system identifier
  string note        = 2;  // Free-text rationale
}

message Spec {
  // ... existing fields, lifecycle field 10 REPURPOSED ...
  SpecProvenance provenance_type = 10;
  oneof provenance_detail {
    AuthoredProvenance          authored             = 17;
    RetroactiveFromPrProvenance retroactive_from_pr  = 18;
    DeclaredProvenance          declared             = 19;
  }
}
```

The redundancy between `provenance_type` and the populated `provenance_detail` variant is enforced server-side at create/update. Mismatched → `INVALID_ARGUMENT`. The enum tag is the fast filter for indexing; the oneof is the type-specific structured payload.

### Stage progressions per provenance

```
AUTHORED:             spark → shape → specify → decompose → approved → (claim) → done
RETROACTIVE_FROM_PR:                                                              done  (born here)
DECLARED:                                                                         done  (born here)
```

**Create-time invariants:**

| Provenance | Required at create | Initial stage |
|---|---|---|
| AUTHORED | Spark output only | `spark` |
| RETROACTIVE_FROM_PR | All four funnel outputs + non-empty url+sha | `done` |
| DECLARED | All four funnel outputs + non-empty declared_by | `done` |

Forward-authored specs use the existing `author spark/shape/specify/decompose` handlers. Non-AUTHORED specs use a single create call that inserts at `done` directly with all funnel content populated atomically. The author calls themselves remain provenance-agnostic; they just won't be called for non-AUTHORED specs.

**Claim and completion are AUTHORED-only.** Server rejects `claim` or `report-completion` on a non-AUTHORED spec with `INVALID_ARGUMENT`.

**Amend semantics:**

- `amend target_stage=<X>` rolls a spec back to stage X. **Provenance is immutable** — never changes through amend.
- AUTHORED amend cycles re-progress through `approved` + claim + completion to reach `done`.
- RETROACTIVE / DECLARED amend cycles re-progress directly to `done` (skip `approved` + claim + completion).
- To change provenance kind, **supersede** with a new spec carrying the new provenance.

### `specgraph ready` semantics

Query:

```sql
WHERE stage = 'approved'
  AND provenance_type = 'authored'
  AND NOT EXISTS (active CLAIMED_BY edge to a non-expired claim)
  AND NOT EXISTS (incomplete DEPENDS_ON to a non-done spec)
  AND NOT EXISTS (incomplete BLOCKS edge from a non-done spec)
```

The `provenance_type = 'authored'` clause is defense-in-depth — non-AUTHORED specs never reach `approved` by design, but explicit filter prevents bugs if invariants are ever violated. This is a behavior change: specs at spark/shape/specify/decompose stages stop appearing in `ready`. They are mid-design, not ready for execution.

A `DEPENDS_ON` is satisfied when the upstream is at `stage = 'done'`, regardless of upstream provenance. A DECLARED spec at `done` properly satisfies dependencies — the contract it describes already exists.

### Drift behavior

- Drift detection runs on all specs at `stage = 'done'`, regardless of provenance.
- Pre-`done` specs are not drift-checked.
- `content_hash_at_link` on outgoing `DEPENDS_ON` edges is baselined:
  - AUTHORED: at the `RecordCompletion` call
  - RETROACTIVE / DECLARED: at spec creation (which inserts at `done` directly)
  - Refreshed on `done`-transition after any amend cycle
- During amend, drift suspends. Resumes when the spec returns to `done`.

**Subtlety for RETROACTIVE_FROM_PR:** the spec content describes reality at the moment of import, not the historical PR-as-merged state. `provenance_detail.retroactive_from_pr` carries the PR URL/SHA as *origin metadata*, not as a temporal snapshot to drift against. Re-running an import skill on the same PR weeks later may produce different content if code has evolved; the second invocation either overwrites with hand-edit-guard semantics or supersedes.

### Render output

Provenance is rendered prominently — right after the spec title/slug, before stage details. Always shown (never silent-default for AUTHORED) to eliminate "is this missing or AUTHORED?" reader ambiguity.

Per-provenance display:

```
provenance:   AUTHORED
```

```
provenance:   RETROACTIVE_FROM_PR
              https://github.com/specgraph/specgraph/pull/952
              merged 2026-05-21 (commit b0684373)
```

```
provenance:   DECLARED
              declared by sebrandt: "describes pre-SpecGraph OAuth flow"
```

Stage progression display adapts: AUTHORED shows the full funnel timeline if timestamps are present; non-AUTHORED collapses to a single `created_at` line (no fake progression).

`--json` output includes `provenance_type` as the proto-encoded enum string and the populated `provenance_detail` variant. Machine-readable shape unchanged.

## Migration / clean-break footprint

Single coherent PR. Wire-breaking proto change. Postgres column drop + add. All callers updated atomically.

| Layer | Files | Change |
|---|---|---|
| Proto | `proto/specgraph/v1/spec.proto` | Remove `SpecLifecycle` enum + `lifecycle` field. Add `SpecProvenance`, three detail messages, `provenance_type` field, `provenance_detail` oneof |
| Generated | `gen/specgraph/v1/...` | `task proto`; commit per ADR-002 |
| Domain | `internal/storage/spec_domain.go` | Remove `SpecLifecycle`. Add `SpecProvenance` + constants + detail variants. Update `Spec` struct |
| Server convert | `internal/server/convert_spec.go` | Replace lifecycle mapping with provenance mapping (both directions) |
| Server handlers | `internal/server/spec_handler.go`, `claim_handler.go`, `execution_handler.go` | Validate provenance-vs-content at create. Gate claim + completion on `AUTHORED` |
| Postgres migration | `internal/storage/postgres/migrations/NNNN_provenance.sql` (new) | DROP `lifecycle`; ADD `provenance_type TEXT NOT NULL DEFAULT 'authored'`; ADD `provenance_detail JSONB NOT NULL DEFAULT '{}'` |
| Postgres queries | `internal/storage/postgres/spec.go`, `graph.go` | Update insert/select. Update `GetReady` per Section 3 |
| Render | `internal/render/spec.go` | Replace lifecycle switch with provenance-aware render |
| CLI | `cmd/specgraph/spec.go`, `create.go` | Replace `--lifecycle` with `--provenance` + provenance-detail flags |
| MCP `spec` tool | `internal/mcp/tools_spec.go` | Replace lifecycle param with provenance + detail params. Extend `spec create` action to accept all four funnel outputs in a single call when provenance is RETROACTIVE_FROM_PR or DECLARED |
| Tests | `*_test.go` across all layers | Update existing lifecycle tests. Add tests for three creation paths + `GetReady` new predicate |
| Docs | `CHANGELOG.md`, `site/docs/concepts/spec-graph.md`, possibly `site/docs/concepts/example-spec.md` | CHANGELOG breaking-change entry. Concept doc loses task/living row, gains provenance section |

**Implementation order:**

1. Proto + `task proto` regenerate (everything else compiles against the new shape)
2. Domain types (type-checker becomes the caller finder)
3. Storage migration + queries
4. Server handlers (claim/completion gating, create validation)
5. CLI + MCP plumbing
6. Render
7. Tests across layers; integration tests for the three creation paths
8. Docs

## Testing

- Unit tests at every layer mirror the existing lifecycle test coverage.
- Integration tests for each provenance creation path:
  - AUTHORED: full funnel happy path including claim + completion
  - RETROACTIVE_FROM_PR: born-at-done create with valid detail; rejection on missing url/sha; rejection on attempting claim
  - DECLARED: born-at-done create with valid detail; rejection on missing declared_by; rejection on attempting claim
- `GetReady` integration test with mixed-provenance + mixed-stage seed data; assert only AUTHORED-at-approved-unclaimed specs appear.
- Amend cycle test: amend a DECLARED spec back to `shape`, re-progress, assert returns to `done` without visiting `approved`.
- Drift baseline test: create RETROACTIVE spec, assert `content_hash_at_link` on outgoing `DEPENDS_ON` matches at creation time.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Wire break affects external consumers | Clean-break per project status; no current external consumers; CHANGELOG entry as belt-and-suspenders |
| JSONB `provenance_detail` makes queries harder | Postgres JSONB indexing supports `->>` predicates; the `provenance_type` enum column is the primary filter; oneof details are read-mostly |
| Implementation order missteps break the build mid-PR | Type-checker is the gate: proto + domain first means subsequent layers fail to compile until updated; build never reaches a green state until all layers are done |
| Render output change confuses CLI users | Provenance line is always-shown; CHANGELOG calls it out; help text in `spec show` documents the format |
| Amend semantics on non-AUTHORED specs feel surprising | Documentation: amend cycles preserve provenance, non-AUTHORED skips approved+claim on re-progression; same as creation behavior |

## References

- ADR-002: Stable ULID IDs with Murmur3-128 Content Hash (`docs/decisions/ADR-002-stable-ulid-ids-content-hash.md`)
- ADR-004: Optimistic Concurrency with Transaction-Wrapped Write Paths (`docs/decisions/ADR-004-optimistic-concurrency-transactions.md`)
- Original lifecycle plan: `docs/plans/2026-03-07-slice-5-spec-lifecycle-revised-plan.md`
- Abandoned predecessor spec: `specgraph-retroactive-lifecycle` (graph node, abandoned with reshape rationale)
- Parked spark-stage spec: `specgraph-spec-provenance` (graph node, will be superseded by the spec authored from this doc)
- Parked spark-stage spec: `specgraph-spec-from-pr` (graph node, the original retroactive-import skill that motivated this work; depends on this design landing)
