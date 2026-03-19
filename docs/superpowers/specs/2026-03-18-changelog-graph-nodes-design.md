# ChangeLog Graph Nodes for Version Tracking

**Status:** Approved
**Date:** 2026-03-18
**Bead:** spgr-3uh (decision), spgr-1p6 (design task)
**Depends on:** spgr-lmt (content hash computation)

## Problem

SpecGraph needs to:

1. Detect when a spec has materially changed.
2. Identify what is impacted by that change.
3. Propagate changes and notify impacted specs/systems.
4. Support point-in-time views at stage transitions (checkpoints) and understand what exactly changed between them.

The existing `history_json` field on Spec nodes is a JSON blob storing lifecycle events. It is opaque to Cypher, cannot be queried across specs without full deserialization, and does not capture field-level deltas.

## Decision

Introduce `ChangeLog` as a first-class graph node linked to specs via `HAS_CHANGE` edges. Each material mutation to a spec creates a ChangeLog node containing the content hash, a checkpoint flag, and a serialized array of field-level deltas. Remove the existing `history_json` property and all related code.

## Design

### Data Model

**New proto message — `FieldChange`:**

```protobuf
message FieldChange {
  string field     = 1;  // top-level field name: "intent", "priority", "spark_output", etc.
  string old_value = 2;  // previous value (serialized string)
  string new_value = 3;  // new value (serialized string)
}
```

**New graph node — `ChangeLog`:**

| Property | Type | Description |
|---|---|---|
| `id` | string (ULID) | Unique identifier |
| `version` | int32 | Spec version at time of change |
| `stage` | string | Spec stage at time of change |
| `content_hash` | string | Spec content hash after this change |
| `checkpoint` | bool | True at stage transitions |
| `summary` | string | Human-readable description of the change |
| `reason` | string | Why the change was made |
| `changes_json` | string | Standard `encoding/json` marshaled `[]FieldChange` |
| `date` | string | Timestamp (sortableRFC3339Nano format) |

**New edge — `HAS_CHANGE`:**

Direction: `(:Spec)-[:HAS_CHANGE]->(:ChangeLog)`

Internal infrastructure edge — not added to the proto `EdgeType` enum, not exposed via `AddEdge`/`RemoveEdge` RPCs.

**Field granularity:** Top-level fields only. Authoring outputs (`spark_output`, `shape_output`, `specify_output`, `decompose_output`) are recorded as whole values. Consumers can diff the JSON content if sub-field detail is needed.

### Write Path

Every mutation where `content_hash` changes creates a ChangeLog node. The write is atomic — a single Cypher query updates the Spec and creates the ChangeLog node:

```cypher
MATCH (s:Spec {slug: $slug, version: $expected_version})
SET s.version = s.version + 1,
    s.content_hash = $new_hash,
    s.updated_at = $now,
    s.intent = $intent
CREATE (s)-[:HAS_CHANGE]->(:ChangeLog {
  id: $changelog_id,
  version: $new_version,
  stage: $stage,
  content_hash: $new_hash,
  checkpoint: $is_checkpoint,
  summary: $summary,
  reason: $reason,
  changes_json: $changes_json,
  date: $now
})
RETURN s
```

**Delta computation:** Before the mutation query, read the current spec. After computing new values, diff old vs new for each substantive field. Only fields where the value changed are included in `changes_json`.

**No-op guard:** If `content_hash` does not change (e.g., updating `notes`, which is excluded from the hash), no ChangeLog node is created.

**Mutation hook points:**

| Mutation | Location | Checkpoint? |
|---|---|---|
| `CreateSpec` | `memgraph.go` | Yes (initial) |
| `UpdateSpec` | `memgraph.go` | No |
| `TransitionStage` | `authoring.go` | Yes |
| `StoreSparkOutput` | `authoring.go` | No |
| `StoreShapeOutput` | `authoring.go` | No |
| `StoreSpecifyOutput` | `authoring.go` | No |
| `StoreDecomposeOutput` | `authoring.go` | No |
| `LifecycleAmendSpec` | `lifecycle.go` | Yes |
| `LifecycleSupersedeSpec` | `lifecycle.go` | Yes |
| `LifecycleAbandonSpec` | `lifecycle.go` | Yes |

### Read Path

**Storage interface:**

```go
type ChangeLogEntry struct {
    ID          string
    Version     int32
    Stage       SpecStage
    ContentHash string
    Checkpoint  bool
    Summary     string
    Reason      string
    Changes     []FieldChange
    Date        time.Time
}

type FieldChange struct {
    Field    string
    OldValue string
    NewValue string
}

type ChangeLogFilter struct {
    CheckpointsOnly bool
    SinceVersion    int32
    Limit           int  // 0 means no limit (return all matching entries)
}

type ChangeLogBackend interface {
    // ListChanges returns changelog entries for a spec, ordered by version.
    // Returns an empty slice (not an error) if the spec has no changelog entries.
    // Returns storage.ErrSpecNotFound if the spec slug does not exist.
    ListChanges(ctx context.Context, slug string, opts ChangeLogFilter) ([]*ChangeLogEntry, error)
}
```

**Core queries:**

Single spec history:

```cypher
MATCH (s:Spec {slug: $slug})-[:HAS_CHANGE]->(c:ChangeLog)
RETURN c ORDER BY c.version
```

Checkpoints only:

```cypher
MATCH (s:Spec {slug: $slug})-[:HAS_CHANGE]->(c:ChangeLog {checkpoint: true})
RETURN c ORDER BY c.version
```

Changes since a version:

```cypher
MATCH (s:Spec {slug: $slug})-[:HAS_CHANGE]->(c:ChangeLog)
WHERE c.version > $since_version
RETURN c ORDER BY c.version
```

Cross-spec recent changes:

```cypher
MATCH (s:Spec)-[:HAS_CHANGE]->(c:ChangeLog)
WHERE c.date > $since
RETURN s.slug, c ORDER BY c.date DESC
LIMIT $limit
```

**Indexes:** Create label-property indexes for cross-spec queries:

```cypher
CREATE INDEX ON :ChangeLog(version);
CREATE INDEX ON :ChangeLog(date);
```

**No new RPC** in this work. The `ChangeLogBackend` is internal. Exposing via ConnectRPC is a future step when there is a consumer.

### Migration: Removing `history_json`

Clean removal — no backward compatibility needed.

**Proto:**

- Remove `HistoryEntry` message from `spec.proto`
- Remove `repeated HistoryEntry history = 13` from `Spec` message
- Reserve field number 13 and name `"history"`
- Add `FieldChange` message

**Domain types (`internal/storage/`):**

- Remove `History []HistoryEntry` from `Spec` struct
- Remove `HistoryEntry` struct
- Add `ChangeLogEntry` and `FieldChange` structs
- Add `ChangeLogBackend` interface

**Memgraph (`internal/storage/memgraph/`):**

- Remove `history_json` from all Cypher queries (CREATE, RETURN, SET)
- Remove `marshalHistory`, `unmarshalHistory`, `appendHistory`, `historyEntryJSON`
- Remove `maxHistoryEntries` constant
- Remove `history_json` from `scanSpec` helper
- Add ChangeLog node creation to each mutation path
- Add `ListChanges` implementation

**Server/handlers:**

- Remove history mapping in proto-to-domain converters

**Tests:**

- Remove all `history_json` / `unmarshalHistory` unit tests
- Update integration tests that assert on `spec.History`
- Add tests for ChangeLog creation and querying
- No "prove history_json is gone" tests — just remove the old code

**Unchanged:**

- `version` (int32) stays on Spec — still used for optimistic locking
- `content_hash` stays on Spec — still the current fingerprint
- All existing edge types and graph queries

### Impact Analysis

Impact analysis uses the existing `GetImpact()` graph traversal:

```cypher
MATCH (s:Spec {slug: $slug})<-[:DEPENDS_ON*]-(impacted:Spec)
RETURN DISTINCT impacted.slug, impacted.stage
```

The ChangeLog tells you *what* changed on a spec. The graph traversal tells you *who is affected*. These compose naturally: detect change via ChangeLog, walk edges to find impact.

### Drift Detection

The existing drift detection in `internal/drift/` uses `UpdatedAt` timestamps. This design enables a future upgrade (tracked as spgr-3ei) to use content hashes:

- ChangeLog nodes with `checkpoint=true` at stage transitions provide comparison points
- `content_hash` on the Spec node provides current state
- If `current hash != approved checkpoint hash` → spec drifted post-approval

No changes to drift detection in this work.

### Event Propagation

This design enables but does not build event propagation. The ChangeLog node is the event record. A future mechanism (event hook, polling, or Memgraph trigger) would:

1. Observe ChangeLog creation
2. Walk `DEPENDS_ON` edges via `GetImpact()` to find affected specs
3. Notify consumers (sync adapters, Gastown, external webhooks)

### Future: Compaction

Compaction is deferred but has a clear path:

1. Select two consecutive checkpoint ChangeLog nodes for a spec
2. Query intermediate non-checkpoint nodes between them
3. Merge `changes_json` arrays — for duplicate fields, collapse `A→B, B→C` into `A→C`
4. Create one compacted ChangeLog node with merged changeset
5. `DETACH DELETE` the intermediate nodes

Checkpoints are never deleted during compaction — they serve as stable boundary markers. A `compacted` bool can be added to ChangeLog when compaction is built.

### Documentation Updates

**Site docs (`site/docs/concepts/`):**

- `specs.md` — Add a "Change Tracking" section explaining that every material change to a spec creates a ChangeLog node in the graph. Cover: content hash for change detection, field-level deltas, checkpoints at stage transitions. Update the Identity fields table to remove any reference to `history` and note the `HAS_CHANGE` edge type.
- `authoring.md` — Add a brief note in the "Why structured outputs?" section that stage transitions create checkpoint ChangeLog nodes, linking the authoring funnel to the change tracking mechanism.

**Internal docs (`docs/`):**

- `decisions/ADR-002-stable-ulid-ids-content-hash.md` — Add a forward-reference noting that content hash is now consumed by ChangeLog nodes for field-level change tracking (this design).

**CLAUDE.md:**

- Update the Architecture table to mention ChangeLog as a graph node type.
- Add a gotcha about `HAS_CHANGE` being internal-only (not in EdgeType enum, not exposed via AddEdge/RemoveEdge RPCs).

## Alternatives Considered

### Option A: `last_synced_hash` on SyncMapping only

Too narrow — only covers sync adapters, not core version tracking. Still needed for sync but not sufficient alone.

### Option B: Edge versioning (`established_at_version` property)

Deferred. Edge staleness is derivable from spec hash changes without storing version on edges.

### Option C: SpecVersion vertex nodes (full snapshots)

Deferred to Phase 3/4. Heaviest approach — full git-for-specs. Only needed if full content snapshots at every version become a requirement. ChangeLog field deltas are lossless and lighter.

### Field deltas on `HistoryEntry` JSON blob (original Approach 1)

Rejected after analysis. Cross-spec queries require full deserialization of every spec's `history_json`. ChangeLog as graph nodes are natively queryable in Cypher, independently indexable, and eliminate the `maxHistoryEntries` cap.

### Full snapshots on every HistoryEntry

Rejected. Storage bloat for data that is reconstructible from field deltas. Deltas are not reconstructible from snapshots.

### FieldChange as separate graph nodes

Considered but deferred. Makes individual field changes queryable across specs (`MATCH ... (f:FieldChange {field: "intent"})`), but the common query pattern is per-spec history, which works well with `changes_json`. Promotion to nodes is a mechanical migration if cross-spec field-level queries become needed.
