<!--
SPDX-License-Identifier: Apache-2.0
-->

# Spec Provenance Model — Design

**Date:** 2026-05-20 (revised twice after adversarial review)
**Status:** Draft — pending SpecGraph spec authoring
**Supersedes:** Abandoned spec `specgraph-retroactive-lifecycle`; parked Spark-stage spec `specgraph-spec-provenance`

## Context

Two discoveries during spec work surfaced a model coherence issue:

1. **`GetReady` implementation gap.** The query in `internal/storage/postgres/graph.go` filters only on `stage <> 'done'`. It does not consult `lifecycle`. As a result, `lifecycle=living` specs incorrectly surface in `specgraph ready` despite the original design intent (per `docs/plans/2026-03-07-slice-5-spec-lifecycle-revised-plan.md`). The query also surfaces `superseded` and `abandoned` specs as "ready," which is also wrong.

2. **Conceptual fuzziness between `task/living/done`.** Investigation revealed the lifecycle field tries to encode two orthogonal axes — "how does this spec relate to executed work" and "where in the funnel content is this" — and does both badly. After a TASK reaches `done`, it serves the same role as a LIVING spec: drift target, dependency anchor, contract under amendment/supersession. The distinction is vestigial post-`done`.

The original framing (`specgraph-retroactive-lifecycle`, since abandoned) proposed adding `SPEC_LIFECYCLE_RETROACTIVE` to solve the retroactive-spec use case. Brainstorming exposed that this would have created a third lifecycle behaviorally identical to LIVING, distinguished only by provenance — confirming the field itself is the problem.

**Clean-break license.** SpecGraph is pre-1.0 with no production data. This design treats the change as a wire-and-data clean break: the proto field is repurposed, the column is dropped without preserving values, no transitional flags. The codebase already has migration precedent: `internal/storage/postgres/migrations/005_remove_amended_stage.sql` removed a stage outright. `proto/specgraph/v1/authoring.proto:135-141` documents a wire-break on `SpecifyOutput` field numbers ("Intentional at 0.2.0-dev (no production data). Field numbers reused because semantic intent is preserved.") — the same posture applies here.

## Goals

- **Replace** the `SpecLifecycle` enum with a single `SpecProvenance` field that records how a spec entered the graph
- **Fix** the `specgraph ready` query to accurately surface "approved, unclaimed, dependency-satisfied" work only
- **Unify** the post-`done` behavior across all specs (drift, dependency anchoring) regardless of how they got there
- **Enable** retroactive-from-PR and human-declared spec creation paths cleanly
- **Establish** clean break: no transitional flags, no backwards compatibility, no field deprecation tail, no data preservation

## Non-Goals

- No spec-to-code drift detection (existing spec-to-spec drift mechanism stays as-is)
- No automatic provenance inference (callers set explicitly)
- No new MCP tool or RPC — extend existing `spec create/update` paths
- No SvelteKit / web UI changes
- No data migration (none exists; column is dropped without preserving values)
- No Memgraph backend changes (postgres is the active backend)

## The full stage taxonomy (correction from prior draft)

Before the model: this design's prior draft missed half the stages. The full taxonomy per `internal/storage/spec_domain.go:14-25` is:

```text
Authoring:  spark → shape → specify → decompose → approved
Execution:  approved → in_progress → review → done
Terminal:   superseded | abandoned   (set by supersede / abandon CLI actions)
```

`in_progress` and `review` are claimed-and-executing stages; `RecordCompletion` (`internal/storage/postgres/execution.go:113`) transitions to `done`. `superseded` and `abandoned` are terminal lifecycle states set explicitly via the supersede/abandon paths (`internal/storage/postgres/lifecycle.go`).

`GenerateBundle` (`execution.go:28-30`) requires `stage IN (approved, in_progress)` — non-AUTHORED specs born at `done` will **not** be eligible for bundle generation, which is correct (there's no execution to bundle for; the work was done outside the funnel).

## Amend vs supersede (correction from prior draft)

Prior draft incorrectly stated that `amend` could roll a `done` spec back to an earlier stage. **It cannot.** Per `spec_domain.go:79-86`, `IsAmendEligible` returns true only for `approved`, `in_progress`, and `review`. Done specs use **supersede** for updates.

In the new model this simplifies cleanly:

- **Amend** is for in-flight rework. Only applicable to AUTHORED specs at `approved`/`in_progress`/`review`, because RETROACTIVE/DECLARED specs never visit those stages.
- **Supersede** is for replacing a `done` spec — regardless of provenance — with a new spec. The new spec gets a fresh provenance (set freshly at creation; not inherited). The old spec keeps its provenance and stage transitions to `superseded`.
- **Provenance is immutable** under both operations. Amend never crosses provenance because amend only sees AUTHORED. Supersede creates a fresh spec; no inheritance.

This is a tighter story than the prior draft and matches how the existing lifecycle code already works.

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
  // ... existing fields up to conversation_count = 21 ...

  // WIRE-BREAK: field 10 was `SpecLifecycle lifecycle`. Repurposed at pre-1.0
  // per the same posture as the SpecifyOutput field-renumber in
  // authoring.proto (no production data; semantic intent preserved — it's
  // still "how should the funnel treat this spec," just with cleaner axes).
  SpecProvenance provenance_type = 10;

  oneof provenance_detail {
    AuthoredProvenance          authored             = 22;
    RetroactiveFromPrProvenance retroactive_from_pr  = 23;
    DeclaredProvenance          declared             = 24;
  }
}
```

**Field-number notes (correcting prior draft):** Fields 17–20 are already used for `spark_output`, `shape_output`, `specify_output`, `decompose_output`; field 21 for `conversation_count`. The oneof variants start at field 22 to avoid collision.

The redundancy between `provenance_type` and the populated `provenance_detail` variant is enforced server-side at create/update: a mismatched pair (e.g., `provenance_type=DECLARED` with `retroactive_from_pr` set) is rejected with `INVALID_ARGUMENT` and the sentinel `storage.ErrProvenanceMismatch`. The enum tag is the fast filter for indexing; the oneof is the type-specific structured payload.

### Storage-side domain type

Per constitution principle p-2 (storage uses domain types, not proto), the domain shape sits in `internal/storage/spec_domain.go`:

```go
// SpecProvenanceType is the string-typed discriminator, mirroring the
// existing SpecLifecycle pattern in this file.
type SpecProvenanceType string

const (
    SpecProvenanceAuthored           SpecProvenanceType = "authored"
    SpecProvenanceRetroactiveFromPR  SpecProvenanceType = "retroactive_from_pr"
    SpecProvenanceDeclared           SpecProvenanceType = "declared"
)

func (p SpecProvenanceType) IsValid() bool { /* ... */ }

// SpecProvenanceDetail is the structured payload. Exactly one of the
// embedded pointers is non-nil; nil pointers are valid (AUTHORED has no
// detail). The detail must match the discriminator — enforced at the
// server boundary, then carried domain-side as a single struct.
type SpecProvenanceDetail struct {
    RetroactiveFromPR *RetroactivePRProvenance // populated when type == retroactive_from_pr
    Declared          *DeclaredProvenance      // populated when type == declared
    // AUTHORED has no detail; both pointers nil.
}

type RetroactivePRProvenance struct {
    URL      string
    SHA      string
    MergedAt time.Time
    Title    string
}

type DeclaredProvenance struct {
    DeclaredBy string
    Note       string
}

// Spec struct adds:
type Spec struct {
    // ... existing fields, with Lifecycle field REMOVED ...
    Provenance       SpecProvenanceType
    ProvenanceDetail SpecProvenanceDetail
}
```

The domain layer does **not** use a Go-side `oneof` (Go interfaces would be the analog) because the variants are closed and the discriminator is the load-bearing key. A struct with type-tag-plus-optional-pointers is the idiomatic shape and matches how the existing `SpecLifecycle` field is handled.

### JSONB envelope (postgres storage)

`provenance_detail` is stored in a single JSONB column. The envelope is always present (even for AUTHORED specs, where the body is empty), to keep marshaling deterministic:

```json
{ "type": "authored",            "data": null }
{ "type": "retroactive_from_pr", "data": { "url": "https://...", "sha": "abc123", "merged_at": "2026-05-21T01:18:19Z", "title": "fix: ..." } }
{ "type": "declared",            "data": { "declared_by": "example-user", "note": "..." } }
```

The `type` key in the JSON envelope **must equal** the `provenance_type` enum column for a given row; the server enforces this at insert/update time via the provenance-mismatch validator (W2 acknowledgment: the `provenance_type` column is what `GetReady` filters on, not the JSONB body, because column predicates are cheap and indexable while `data->>'type'` would force JSONB extraction on every row).

Sentinel: `storage.ErrProvenanceMismatch` for envelope/discriminator mismatch.

### Stage progressions per provenance

```text
AUTHORED:             spark → shape → specify → decompose → approved → in_progress → review → done
RETROACTIVE_FROM_PR:                                                                            done  (born here)
DECLARED:                                                                                       done  (born here)
```

`in_progress` and `review` are stages an AUTHORED spec passes through during claimed execution. They are explicitly skipped by RETROACTIVE / DECLARED specs (born at done). If any subsystem hooks `in_progress` or `review` transitions (none currently do, but worth noting), it will not fire for non-AUTHORED specs by design — the work happened outside the funnel and there is no corresponding execution event.

**Create-time invariants** — validated server-side at create; reject with `INVALID_ARGUMENT` and provenance-specific sentinels otherwise:

| Provenance | Required at create | Initial stage | Sentinel on violation |
|---|---|---|---|
| `AUTHORED` | Spark output only | `spark` | `ErrAuthoredRequiresSparkOnly` if other stage outputs are set |
| `RETROACTIVE_FROM_PR` | All four funnel outputs + non-empty url + non-empty sha | `done` | `ErrRetroactiveRequiresAllOutputs`, `ErrRetroactiveRequiresPRRef` |
| `DECLARED` | All four funnel outputs + non-empty declared_by | `done` | `ErrDeclaredRequiresAllOutputs`, `ErrDeclaredRequiresDeclaredBy` |

For RETROACTIVE/DECLARED specs born at `done`, the create handler computes `content_hash` **before insert** (using `contenthash.Spec` with all four stage outputs populated, identical to the post-completion recomputation that `RecordCompletion` performs) and inserts in a single transaction. This means a born-at-done create is one round-trip to the DB, not the typical insert-then-`recomputeContentHash` pattern.

**Conversation logs for non-AUTHORED specs:** the `conversation_logs` field is **empty** for RETROACTIVE and DECLARED creates. Conversation logs are an artifact of multi-turn authoring dialog through the funnel; born-at-done specs don't have that artifact and shouldn't fake one. The audit signal lives in `provenance_detail` (URL/SHA/declared_by/note), which is more structured anyway.

The existing `ValidateExchanges` (`internal/authoring/validate.go:42`) rejects roles other than `probe`/`response`; it's invoked from Shape/Specify/Decompose author RPCs but **not** from `CreateSpec`. The extended `CreateSpec` path for non-AUTHORED creates therefore must not invoke `ValidateExchanges` on creation. AUTHORED specs reach Shape/Specify/Decompose via the existing funnel handlers and continue to require validated exchanges there.

**Claim and completion are AUTHORED-only:**

- `claim` returns `INVALID_ARGUMENT` + `ErrClaimRequiresAuthored` for any spec with `provenance_type != AUTHORED`
- `report-completion` returns the same with `ErrCompletionRequiresAuthored`
- These are sentinel errors named explicitly to satisfy the constitution antipattern (mocks must return sentinels, not `fmt.Errorf`)

### `specgraph ready` semantics

Query in `internal/storage/postgres/graph.go`:

```sql
WHERE stage = 'approved'
  AND provenance_type = 'authored'
  AND NOT EXISTS (active CLAIMED_BY edge to a non-expired claim)
  AND NOT EXISTS (incomplete DEPENDS_ON to a non-done spec)
  AND NOT EXISTS (incomplete BLOCKS edge from a non-done spec)
```

Three filters do real work; the design owes a coherent rationale for each:

- **`stage = 'approved'`** — the load-bearing change. Excludes spark/shape/specify/decompose (mid-design), in_progress/review (someone else is working), done (already finished), and superseded/abandoned (terminal). Today's query (`stage <> 'done'`) misses everything except done.
- **`provenance_type = 'authored'`** — **explicit predicate**, self-documenting. Not "defense in depth" — that framing was muddled. The predicate makes the query's intent legible to a reader: `ready` means work that was forward-authored and is awaiting execution. RETROACTIVE/DECLARED specs never reach `approved` by design, so the predicate is also redundant in practice — but redundancy in a self-documenting query is feature, not bug.
- **`NOT EXISTS active claim`** — someone else is on it; not available.

A `DEPENDS_ON` is satisfied when the upstream is at `stage = done`, regardless of upstream provenance. A DECLARED spec at `done` properly satisfies dependencies — the contract it describes already exists.

This is a behavior change for any existing caller. Specs at `spark/shape/specify/decompose` stop appearing in `specgraph ready`. This is the intended fix (today they appear incorrectly).

### Drift behavior

- Drift detection runs on specs at `stage = 'done'`, regardless of provenance — matching the existing eligibility check in `internal/drift/drift.go:62-64`
- Pre-`done` specs are not drift-checked
- `content_hash_at_link` on outgoing `DEPENDS_ON` edges is baselined:
  - AUTHORED: at the `RecordCompletion` call (existing behavior)
  - RETROACTIVE / DECLARED: at spec creation (the create call inserts at `done` directly with hash pre-computed)
  - Refreshed on `done`-transition after any amend cycle (AUTHORED only — non-AUTHORED specs aren't amend-eligible by virtue of never being at amend-eligible stages)
- During amend (AUTHORED only), drift suspends and resumes when the spec returns to `done` via `RecordCompletion`

**Subtlety for RETROACTIVE_FROM_PR:** the spec content describes reality at the moment of import, not the historical PR-as-merged state. `provenance_detail.retroactive_from_pr` carries the PR URL/SHA as *origin metadata*, not as a temporal snapshot to drift against. Re-running an import skill on the same PR weeks later either overwrites (hand-edit-guard semantics from `specgraph-adr-from-spec`) or supersedes.

**Content-hash inputs:** `provenance_type` and `provenance_detail` are **not** included in `contenthash.Spec`. The hash continues to cover `(intent, stage, priority, complexity, outputs)` — the existing inputs, unchanged. Lifecycle was never hashed and provenance follows the same posture: hash represents the spec's authored content, not its provenance metadata. Since provenance is immutable in this design, including it in the hash would be a no-op in practice anyway.

### Render output

Provenance is rendered prominently — right after the spec title/slug, before stage details. Always shown (never silent-default for AUTHORED) to eliminate "is this missing or AUTHORED?" reader ambiguity.

```text
provenance:   AUTHORED
```

```text
provenance:   RETROACTIVE_FROM_PR
              https://github.com/specgraph/specgraph/pull/952
              merged 2026-05-21 (commit b0684373)
```

```text
provenance:   DECLARED
              declared by example-user: "describes pre-SpecGraph OAuth flow"
```

Stage progression display adapts: AUTHORED shows the full funnel timeline if timestamps are present; non-AUTHORED collapses to a single `created_at` line (no fake progression).

`--json` output includes `provenance_type` as the proto-encoded enum string and the populated `provenance_detail` variant.

## Migration / clean-break footprint

Single coherent PR. Wire-breaking proto change. No data migration (no production data per project status). All callers updated atomically.

| Layer | Files | Change |
|---|---|---|
| Proto | `proto/specgraph/v1/spec.proto` | Remove `SpecLifecycle` enum + `lifecycle` field. Add `SpecProvenance`, three detail messages, `provenance_type` at field 10 (with WIRE-BREAK comment), `provenance_detail` oneof at fields 22–24 |
| Generated | `gen/specgraph/v1/...` | `task proto`; commit per ADR-002 |
| Domain | `internal/storage/spec_domain.go` | Remove `SpecLifecycle`. Add `SpecProvenanceType` + constants + `SpecProvenanceDetail` struct + variant structs. Update `Spec` struct fields |
| Server convert | `internal/server/convert_spec.go` | Replace `lifecycleToProtoMap` with provenance mapping (both directions, both type tag and oneof variant) |
| Server handlers | `internal/server/spec_handler.go`, `claim_handler.go`, `execution_handler.go` | Validate provenance-vs-content at create with the new sentinels. Gate `claim` + `report-completion` on `provenance_type = AUTHORED` |
| Postgres migration | `internal/storage/postgres/migrations/007_spec_provenance.sql` (new) | Precondition guard: `DO $$ BEGIN IF (SELECT count(*) FROM specs) > 0 THEN RAISE EXCEPTION 'migration 007 refuses to run on a non-empty specs table; clean-break design assumes no data'; END IF; END $$;` Then: `ALTER TABLE specs DROP COLUMN lifecycle; ALTER TABLE specs ADD COLUMN provenance_type TEXT NOT NULL; ALTER TABLE specs ADD COLUMN provenance_detail JSONB NOT NULL;` — no data preservation per clean-break license; precondition prevents accidental data loss if the assumption is ever violated |
| Proto comment fix | `proto/specgraph/v1/spec.proto:27` | Update `string stage = 4;` comment to enumerate all eight stages: `spark | shape | specify | decompose | approved | in_progress | review | done | superseded | abandoned`. The current comment misses `review`, `superseded`, `abandoned` — a doc-string drift this design's stage-taxonomy rework is a natural moment to fix |
| Postgres queries | `internal/storage/postgres/spec.go`, `graph.go` | Update insert/select. Update `GetReady` per design |
| Render | `internal/render/spec.go` | Replace `SpecLifecycle_SPEC_LIFECYCLE_LIVING` switch (line 61) with provenance-aware render |
| CLI | `cmd/specgraph/spec.go`, `create.go` | Replace `--lifecycle` with `--provenance` + provenance-detail flags (`--pr-url`, `--pr-sha`, `--declared-by`, `--declared-note`) |
| MCP `spec` tool | `internal/mcp/tools_spec.go` | Replace lifecycle param with provenance + detail params. Extend `spec create` action to accept all four funnel outputs in a single call when provenance is RETROACTIVE/DECLARED |
| Linter | `internal/linter/schema.go` (line 71–76 enforces `spec.Lifecycle.IsValid()`) | Replace lifecycle validation with provenance + provenance-detail validation; sentinel: `lint warning if provenance_type == UNSPECIFIED on a non-spark-stage spec` |
| Export | `internal/export/engine.go` (line 439, 448–451 references lifecycle) | Replace lifecycle serialization with provenance serialization in import path; export already serializes `Spec` proto so JSONB payload comes along for the ride |
| Sentinel errors | `internal/storage/errors.go` (or wherever sentinels live) | Add `ErrProvenanceMismatch`, `ErrAuthoredRequiresSparkOnly`, `ErrRetroactiveRequiresAllOutputs`, `ErrRetroactiveRequiresPRRef`, `ErrDeclaredRequiresAllOutputs`, `ErrDeclaredRequiresDeclaredBy`, `ErrClaimRequiresAuthored`, `ErrCompletionRequiresAuthored` |
| Tests | `*_test.go` across all layers | Update existing lifecycle tests. Add tests for three creation paths + `GetReady` new predicate + sentinel rejection paths |
| ADR | `docs/decisions/ADR-006-spec-provenance-model.md` (new) | Records the wire-break, the JSONB envelope, and the immutability-via-supersede decision |
| Docs | `CHANGELOG.md`, `site/docs/concepts/spec-graph.md`, possibly `site/docs/concepts/example-spec.md` | CHANGELOG breaking-change entry. Concept doc loses task/living row, gains provenance section |

### Implementation order and the broken-build window

1. Commit 1 on the branch: proto changes + `task proto` regenerate + ADR-006 stub. **The build breaks intentionally here** — every Go file referencing `Lifecycle` or `SpecLifecycle` fails to compile. This is the type-checker's "find every caller for me" pass.
2. Commit 2: domain types update.
3. Commit 3: storage migration + postgres queries.
4. Commit 4: server handlers + sentinel errors.
5. Commit 5: CLI + MCP plumbing.
6. Commit 6: render + linter + export.
7. Commit 7: tests across layers.
8. Commit 8: CHANGELOG + concept doc + ADR-006 finalize.

`task check` will not pass until commit 7 (or later). Reviewers pulling intermediate commits see a broken tree by design. This is the cost of the clean break — acknowledge in the PR description.

## Testing

- Unit tests at every layer mirror the existing lifecycle test coverage
- Integration tests for each provenance creation path:
  - AUTHORED: full funnel happy path including claim + completion
  - RETROACTIVE_FROM_PR: born-at-done create with valid detail; rejection on missing url/sha; rejection on attempting claim
  - DECLARED: born-at-done create with valid detail; rejection on missing declared_by; rejection on attempting claim
- `GetReady` integration test with mixed-provenance + mixed-stage seed data (including `superseded` and `abandoned` rows); assert only AUTHORED-at-approved-unclaimed specs appear
- Supersede test: supersede an AUTHORED done spec with a DECLARED spec; assert old spec stage = superseded, new spec stage = done, provenance differs
- Drift baseline test: create RETROACTIVE spec, assert `content_hash_at_link` on outgoing `DEPENDS_ON` matches at creation time
- Linter test: invalid provenance_type rejected
- Conversation-log handling test: AUTHORED specs require exchanges at Shape/Specify/Decompose (existing behavior); RETROACTIVE/DECLARED creates accept empty `conversation_logs` and round-trip through `GetSpec` unchanged
- Migration precondition test: migration 007 fails fast with a clear error when run against a non-empty specs table
- Mock-backend conformance test: assert all new sentinels are exported and that mocks return sentinels (not `fmt.Errorf`)

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| Wire break affects external consumers | None exist (pre-1.0); CHANGELOG entry as belt-and-suspenders |
| Build is broken across most commits of the PR | Acknowledged. Reviewers + CI see green only at end. PR description names this explicitly |
| JSONB envelope drift between writer + reader | Server enforces `provenance_detail.type == provenance_type` invariant; sentinel `ErrProvenanceMismatch` |
| Implementation order missteps surface partway through | Type-checker is the gate: proto + domain first; subsequent layers fail to compile until updated. Commit order in PR description makes the dependency explicit |
| Render output change confuses CLI users | Provenance line is always-shown; CHANGELOG calls it out |
| Amend semantics on RETROACTIVE/DECLARED feel surprising (they're not amend-eligible at all) | Documented in ADR-006 and CHANGELOG: done specs use supersede, not amend. Same as the existing semantics for AUTHORED done specs — this design just makes it universal |
| Linter rejects existing specs whose provenance was inferred wrong from `lifecycle=task` (legacy column) on a brownfield deployment | Not applicable — clean break + no production data. Linter runs against the new schema only |

## References

- ADR-002: Stable ULID IDs with Murmur3-128 Content Hash (`docs/decisions/ADR-002-stable-ulid-ids-content-hash.md`) — content_hash semantics
- ADR-004: Optimistic Concurrency with Transaction-Wrapped Write Paths (`docs/decisions/ADR-004-optimistic-concurrency-transactions.md`) — atomic create handlers
- Migration precedent: `internal/storage/postgres/migrations/005_remove_amended_stage.sql` — prior stage taxonomy change
- Wire-break precedent: `proto/specgraph/v1/authoring.proto:135-141` — SpecifyOutput field renumber rationale at pre-1.0
- Original lifecycle plan: `docs/plans/2026-03-07-slice-5-spec-lifecycle-revised-plan.md`
- Abandoned predecessor spec: `specgraph-retroactive-lifecycle` (graph node)
- Parked spark-stage spec: `specgraph-spec-provenance` (graph node; will be superseded by the spec authored from this doc)
- Parked spark-stage spec: `specgraph-spec-from-pr` (graph node; depends on this design landing)
- Implementation files referenced in the touch surface (all paths verified against the current tree)
