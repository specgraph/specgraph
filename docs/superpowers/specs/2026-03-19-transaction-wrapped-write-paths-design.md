# Transaction-Wrapped Write Paths for Atomicity

**Status:** Approved
**Date:** 2026-03-19
**Bead:** spgr-1s1
**Depends on:** PR #41 (ChangeLog graph nodes)

## Problem

Multi-query write paths (spec mutation + hash recomputation + ChangeLog creation) execute as separate auto-committed queries. If a failure or concurrent modification occurs between queries, the system can be left in inconsistent state:

- A spec mutation commits but its ChangeLog entry is never created (orphaned audit gap)
- A content hash is recomputed but the ChangeLog records the old hash
- In `LifecycleSupersedeSpec`, one side's ChangeLog exists without the other

The codebase comments claim "single-writer" (`memgraph.go`), but the ConnectRPC server handles concurrent goroutines per request. Two simultaneous requests modifying the same spec can interleave their multi-query sequences.

Version guards (`WHERE s.version = $expected_version`) detect conflicts but don't prevent partial state — the mutation query succeeds, then the ChangeLog query fails, leaving the spec without its audit trail.

## Decision

Wrap all multi-query write paths in `RunInTransaction` (which already exists in `tx.go` and is used by `StoreShapeOutput`). This ensures that if any step fails, the entire operation rolls back. Keep version guards for conflict detection — concurrent writers receive `ErrConcurrentModification` (mapped to `CodeAborted`, retryable by the client).

This is optimistic concurrency control: first writer wins, second fails fast.

## Design

### Write Paths to Wrap

| Path | File | Currently transactional? | Change |
|---|---|---|---|
| `CreateSpec` | `memgraph.go` | No | Wrap mutation + ChangeLog in `RunInTransaction` |
| `UpdateSpec` | `memgraph.go` | No | Wrap readSpecFields + mutation + hash recompute + ChangeLog |
| `TransitionStage` | `authoring.go` | No | Wrap stage SET + recomputeContentHash + ChangeLog |
| `StoreSparkOutput` | `authoring.go` | No | Wrap storeJSONProperty + authoringOutputChangeLog |
| `StoreShapeOutput` | `authoring.go` | Partial | Move `authoringOutputChangeLog` inside existing transaction |
| `StoreSpecifyOutput` | `authoring.go` | No | Wrap storeJSONProperty + authoringOutputChangeLog |
| `StoreDecomposeOutput` | `authoring.go` | No | Wrap parent output + child specs + edges + ChangeLog |
| `AmendSpec` | `authoring.go` | No | Wrap mutation + recomputeContentHash |
| `LifecycleAmendSpec` | `lifecycle.go` | No | Wrap mutation + recomputeContentHash + ChangeLog |
| `LifecycleSupersedeSpec` | `lifecycle.go` | No | Wrap mutation + 1x recomputeContentHash (old spec only) + 2x ChangeLog |
| `LifecycleAbandonSpec` | `lifecycle.go` | No | Wrap mutation + recomputeContentHash + ChangeLog |
| `UpdateDecision` | `decision.go` | No | Wrap mutation + hash recompute (Decision, not Spec — same vulnerability) |

### Implementation Pattern

Each wrapping follows the same shape:

```go
func (s *Store) TransitionStage(ctx context.Context, slug string, from, to storage.AuthoringStage) error {
    // Validation (no DB queries) stays outside the transaction.
    if from == storage.AuthoringStage(authoring.StageApproved) {
        return storage.ErrSpecAlreadyApproved
    }
    if err := authoring.ValidateTransition(...); err != nil {
        return err
    }

    return s.RunInTransaction(ctx, func(txCtx context.Context) error {
        records, err := s.executeQuery(txCtx, query, params)
        // ... error handling ...
        if hashErr := s.recomputeContentHash(txCtx, slug); hashErr != nil {
            return hashErr  // entire transaction rolls back
        }
        // ... build ChangeLog ...
        return s.createChangeLog(txCtx, slug, clEntry, deltas)
    })
}
```

**Key rules:**

1. Pass `txCtx` (not `ctx`) to all DB operations inside the callback
2. `executeQuery` automatically joins the transaction via context (already implemented in `tx.go`)
3. Validation that doesn't hit the DB stays outside the transaction (reduces lock time)
4. Functions returning values capture them via closure variable

**For `StoreShapeOutput`:** Move `authoringOutputChangeLog` call inside the existing `RunInTransaction` callback. Currently it runs after the transaction commits.

**For functions returning values** (e.g., `LifecycleAmendSpec`):

```go
func (s *Store) LifecycleAmendSpec(ctx context.Context, ...) (*storage.Spec, error) {
    var result *storage.Spec
    err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
        // ... all queries with txCtx ...
        result = freshSpec
        return nil
    })
    return result, err
}
```

### Nested Transaction Handling

`RunInTransaction` already handles nested calls (`tx.go:43-44`). If the context already carries a transaction, the inner call reuses it instead of creating a nested session. This means:

- `StoreDecomposeOutput` can call `CreateSpec` (for child specs) inside its transaction
- `authoringOutputChangeLog` can call `createChangeLog` inside the caller's transaction
- No code changes needed to inner functions — they work with both transactional and non-transactional contexts

### Error Semantics

No new error types or retry logic. The existing model:

- **Version guard returns 0 rows** → `preconditionError` distinguishes cause → `ErrConcurrentModification` (retryable) or operation-specific error
- **`createChangeLog` version guard fails** → `ErrConcurrentModification`
- **Any DB error inside transaction** → `RunInTransaction` returns error, neo4j driver auto-rolls back

Handlers already map `ErrConcurrentModification` to `connect.CodeAborted` (retryable per gRPC semantics).

### What Changes

**Code:**

- `memgraph.go`: Wrap `CreateSpec`, `UpdateSpec` in `RunInTransaction`
- `authoring.go`: Wrap `TransitionStage`, `StoreSparkOutput`, `StoreSpecifyOutput`, `StoreDecomposeOutput`. Move `StoreShapeOutput`'s ChangeLog inside existing transaction.
- `lifecycle.go`: Wrap `LifecycleAmendSpec`, `LifecycleSupersedeSpec`, `LifecycleAbandonSpec`
- Remove "single-writer" comment from `memgraph.go`
- Add concurrency model doc comment on `Store` type

**Documentation:**

- `CLAUDE.md`:
  - Architecture table: add `internal/storage/memgraph/tx.go` row
  - New gotcha: all multi-query write paths MUST use `RunInTransaction`
  - Update/remove any "single-writer" language

- New ADR (next available number): "Optimistic concurrency with transaction-wrapped write paths"
  - Context: single-writer assumption invalid under concurrent ConnectRPC requests
  - Decision: `RunInTransaction` for atomicity, version guards for conflict detection
  - Consequences: new write paths must use `RunInTransaction`

- `site/docs/concepts/specs.md`:
  - Add note in Change Tracking section: mutations and ChangeLog entries are atomic within a database transaction

- `site/docs/concepts/specs.md` or new section:
  - Brief "Consistency Model" section: optimistic concurrency, version guards, retryable errors

- `internal/storage/memgraph/tx.go`:
  - Expand doc comment explaining the pattern, when to use, how `txCtx` threading works

### Testing

**New integration tests:**

1. **Concurrent modification rolls back cleanly** — Start an amend inside a transaction, simulate a concurrent version bump (via a separate connection), verify the entire amend + ChangeLog is rolled back (no partial state)

2. **StoreShapeOutput ChangeLog inside transaction** — Verify that if ChangeLog creation fails (e.g., version mismatch), the shape output is also rolled back

**Existing tests:** `tx_test.go` already covers `RunInTransaction` commit/rollback mechanics. No changes needed.

## Alternatives Considered

### Per-spec write serialization (mutexes/channels)

Rejected. Adds deadlock risk (supersede touches two specs), doesn't survive scale-out to multiple server instances, and serializes non-conflicting writes unnecessarily. SpecGraph's conflict rate is low — optimistic concurrency is the right fit.

### Automatic server-side retry

Rejected. Adds complexity and latency. The caller (CLI, skill, agent) is in a better position to decide whether to retry — they have context about what they're doing. Returning `CodeAborted` (retryable) is the standard gRPC approach.

### Single Cypher query per operation

Rejected for write paths involving hash recomputation. The content hash is computed application-side (Murmur3 in Go), so the read → compute → write cycle can't be a single Cypher query. Transactions provide the same atomicity guarantee.
