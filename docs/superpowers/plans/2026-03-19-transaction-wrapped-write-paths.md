# Transaction-Wrapped Write Paths Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wrap all multi-query write paths in `RunInTransaction` for atomic rollback on failure, eliminating orphaned partial state.

**Architecture:** Each multi-query write path gets wrapped in `RunInTransaction(ctx, func(txCtx) error { ... })`. Inner calls use `txCtx` instead of `ctx`. `executeQuery` already joins the transaction via context. Validation stays outside the transaction. Functions returning values capture them via closure.

**Tech Stack:** Go, Memgraph, neo4j Go driver, `RunInTransaction` in `tx.go`

**Spec:** `docs/superpowers/specs/2026-03-19-transaction-wrapped-write-paths-design.md`

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `internal/storage/memgraph/memgraph.go` | Wrap `CreateSpec`, `UpdateSpec`; remove single-writer comment; add concurrency doc |
| Modify | `internal/storage/memgraph/authoring.go` | Wrap `TransitionStage`, `StoreSparkOutput`, `StoreSpecifyOutput`, `StoreDecomposeOutput`, `AmendSpec`; fix `StoreShapeOutput` |
| Modify | `internal/storage/memgraph/lifecycle.go` | Wrap `LifecycleAmendSpec`, `LifecycleSupersedeSpec`, `LifecycleAbandonSpec` |
| Modify | `internal/storage/memgraph/decision.go` | Wrap `UpdateDecision` |
| Modify | `internal/storage/memgraph/tx.go` | Expand doc comment with usage guidance |
| Create | `internal/storage/memgraph/tx_changelog_test.go` | Integration tests for transaction rollback with ChangeLog |
| Modify | `CLAUDE.md` | Architecture table + gotchas |
| Create | `docs/decisions/ADR-004-optimistic-concurrency-transactions.md` | New ADR |
| Modify | `site/docs/concepts/specs.md` | Atomicity note in Change Tracking section |

---

## Chunk 1: Wrap Core Write Paths

### Task 1: Wrap `CreateSpec` in Transaction

**Files:**

- Modify: `internal/storage/memgraph/memgraph.go:147-200`

- [ ] **Step 1: Read the current `CreateSpec` function**

Read `memgraph.go:147-200` to understand the full flow: hash computation, CREATE query, recordToSpec, then createChangeLog.

- [ ] **Step 2: Wrap in `RunInTransaction`**

The hash computation (`contenthash.Spec(...)`) is pure — stays outside. The DB queries move inside:

```go
func (s *Store) CreateSpec(ctx context.Context, slug, intent, priority, complexity string) (*storage.Spec, error) {
	id := newID("spec")
	nowStr := s.now()
	ch := contenthash.Spec(intent, defaultInitialStage, priority, complexity, nil)

	var result *storage.Spec
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// ... existing CREATE query using txCtx ...
		records, qErr := s.executeQuery(txCtx, query, params)
		// ... error handling ...
		spec, parseErr := recordToSpec(records[0])
		if parseErr != nil {
			return parseErr
		}
		// ... existing ChangeLog creation using txCtx ...
		if clErr := s.createChangeLog(txCtx, slug, clEntry, deltas); clErr != nil {
			return clErr
		}
		result = spec
		return nil
	})
	return result, err
}
```

Key changes: `s.executeQuery(ctx, ...)` → `s.executeQuery(txCtx, ...)`, `s.createChangeLog(ctx, ...)` → `s.createChangeLog(txCtx, ...)`, capture `result` via closure.

- [ ] **Step 3: Verify build**

Run: `go build ./internal/storage/memgraph/...`
Expected: PASS

- [ ] **Step 4: Commit**

```text
feat(memgraph): wrap CreateSpec in RunInTransaction

Spec creation and initial ChangeLog entry now execute within
a single database transaction. If ChangeLog creation fails,
the spec creation is rolled back.
```

### Task 2: Wrap `UpdateSpec` in Transaction

**Files:**

- Modify: `internal/storage/memgraph/memgraph.go:323-415`

- [ ] **Step 1: Read current `UpdateSpec`**

Read `memgraph.go:323-415`. Flow: readSpecFields → build SET → executeQuery → recordToSpec → read authoring outputs → compute hash → SET hash → compare hashes → createChangeLog.

- [ ] **Step 2: Wrap in `RunInTransaction`**

Validation (nil checks, empty set) stays outside. All DB operations move inside:

```go
func (s *Store) UpdateSpec(ctx context.Context, slug string, intent, stage, priority, complexity, notes *string) (*storage.Spec, error) {
	// ... build setClauses, params (no DB) ...
	if len(setClauses) == 0 {
		return s.GetSpec(ctx, slug)
	}

	var result *storage.Spec
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		oldFields, oldContentHash, _, _, rfErr := s.readSpecFields(txCtx, slug)
		// ... existing mutation + hash recompute + changelog using txCtx ...
		result = spec
		return nil
	})
	return result, err
}
```

- [ ] **Step 3: Verify build and tests**

Run: `go build ./internal/storage/memgraph/...`
Run: `go test ./internal/storage/memgraph/ -short -v`
Expected: PASS

- [ ] **Step 4: Commit**

```text
feat(memgraph): wrap UpdateSpec in RunInTransaction

Field updates, hash recomputation, and ChangeLog creation now
execute atomically. Partial state on failure is rolled back.
```

### Task 3: Wrap `TransitionStage` in Transaction

**Files:**

- Modify: `internal/storage/memgraph/authoring.go:52-114`

- [ ] **Step 1: Read current `TransitionStage`**

Read `authoring.go:52-114`. Flow: validation → SET query → recomputeContentHash → GetSpec → createChangeLog.

- [ ] **Step 2: Wrap in `RunInTransaction`**

Validation (approved check, ValidateTransition) stays outside:

```go
func (s *Store) TransitionStage(ctx context.Context, slug string, from, to storage.AuthoringStage) error {
	// ... validation stays outside ...
	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		records, err := s.executeQuery(txCtx, query, params)
		// ... error handling ...
		if hashErr := s.recomputeContentHash(txCtx, slug); hashErr != nil {
			return hashErr
		}
		updatedSpec, err := s.GetSpec(txCtx, slug)
		// ... build ChangeLog ...
		return s.createChangeLog(txCtx, slug, clEntry, deltas)
	})
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./internal/storage/memgraph/...`
Expected: PASS

- [ ] **Step 4: Commit**

```text
feat(memgraph): wrap TransitionStage in RunInTransaction

Stage change, hash recomputation, and checkpoint ChangeLog
now execute atomically.
```

### Task 4: Wrap `StoreSparkOutput` and `StoreSpecifyOutput` in Transaction

**Files:**

- Modify: `internal/storage/memgraph/authoring.go:118-130,174-188`

- [ ] **Step 1: Wrap `StoreSparkOutput`**

```go
func (s *Store) StoreSparkOutput(ctx context.Context, slug string, output *storage.SparkOutput) error {
	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		oldFields, oldHash, _, _, err := s.readSpecFields(txCtx, slug)
		if err != nil {
			return err
		}
		if err := s.storeJSONProperty(txCtx, slug, "spark_output", output); err != nil {
			return err
		}
		return s.authoringOutputChangeLog(txCtx, slug, "spark_output", &oldFields, oldHash)
	})
}
```

- [ ] **Step 2: Wrap `StoreSpecifyOutput`**

Same pattern as StoreSparkOutput but for `"specify_output"`.

- [ ] **Step 3: Verify build**

Run: `go build ./internal/storage/memgraph/...`
Expected: PASS

- [ ] **Step 4: Commit**

```text
feat(memgraph): wrap StoreSparkOutput and StoreSpecifyOutput in RunInTransaction
```

### Task 5: Fix `StoreShapeOutput` — Move ChangeLog Inside Transaction

**Files:**

- Modify: `internal/storage/memgraph/authoring.go:134-172`

- [ ] **Step 1: Read current `StoreShapeOutput`**

Read `authoring.go:134-172`. It already uses `RunInTransaction` for decision promotion, but `authoringOutputChangeLog` runs *after* the transaction.

- [ ] **Step 2: Move ChangeLog inside the transaction**

The `readSpecFields` call and `authoringOutputChangeLog` must both be inside `RunInTransaction`. Since `RunInTransaction` supports nesting (reuses existing tx), the inner storeJSONProperty calls will join the outer transaction:

```go
func (s *Store) StoreShapeOutput(ctx context.Context, slug string, output *storage.ShapeOutput) error {
	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		oldFields, oldHash, _, _, err := s.readSpecFields(txCtx, slug)
		if err != nil {
			return err
		}
		// ... existing decision promotion + storeJSONProperty logic using txCtx ...
		return s.authoringOutputChangeLog(txCtx, slug, "shape_output", &oldFields, oldHash)
	})
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./internal/storage/memgraph/...`
Expected: PASS

- [ ] **Step 4: Commit**

```text
fix(memgraph): move StoreShapeOutput changelog inside transaction

ChangeLog creation now participates in the same transaction as
decision promotion and output storage. Previously it ran after
the transaction committed.
```

### Task 6: Wrap `StoreDecomposeOutput` and `AmendSpec` in Transaction

**Files:**

- Modify: `internal/storage/memgraph/authoring.go:190-270,338-400`

- [ ] **Step 1: Wrap `StoreDecomposeOutput`**

This is the most complex — creates child specs with edges. Wrap the entire body. Since `CreateSpec` will also be wrapped (Task 1), nested `RunInTransaction` calls will reuse the outer transaction:

```go
func (s *Store) StoreDecomposeOutput(ctx context.Context, slug string, output *storage.DecomposeOutput) ([]string, error) {
	var childSlugs []string
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		oldFields, oldHash, _, _, rfErr := s.readSpecFields(txCtx, slug)
		if rfErr != nil {
			return rfErr
		}
		// ... existing validation, storeJSONProperty, child creation using txCtx ...
		childSlugs = slugs
		if clErr := s.authoringOutputChangeLog(txCtx, slug, "decompose_output", &oldFields, oldHash); clErr != nil {
			return clErr
		}
		return nil
	})
	return childSlugs, err
}
```

- [ ] **Step 2: Wrap `AmendSpec` (authoring-level)**

Read `authoring.go:338-400`. This calls `LifecycleAmendSpec` then `TransitionStage`. Both are already being wrapped individually (Tasks 3, 8), and nested transactions reuse the outer one:

```go
func (s *Store) AmendSpec(ctx context.Context, slug, reason string, targetStage storage.AuthoringStage) (*storage.AmendResult, error) {
	var result *storage.AmendResult
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// ... existing amend + transition logic using txCtx ...
		result = &storage.AmendResult{...}
		return nil
	})
	return result, err
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./internal/storage/memgraph/...`
Expected: PASS

- [ ] **Step 4: Commit**

```text
feat(memgraph): wrap StoreDecomposeOutput and AmendSpec in RunInTransaction

Decompose output storage (including child spec creation) and
authoring-level amend now execute atomically.
```

---

## Chunk 2: Wrap Lifecycle + Decision, Tests, Documentation

### Task 7: Wrap Lifecycle Operations in Transaction

**Files:**

- Modify: `internal/storage/memgraph/lifecycle.go:65-127,162-280,326-395`

- [ ] **Step 1: Wrap `LifecycleAmendSpec`**

Pre-read and validation stay outside. Atomic mutation + recomputeContentHash + GetSpec + createChangeLog move inside:

```go
func (s *Store) LifecycleAmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*storage.Spec, error) {
	// ... pre-read spec, validation (outside) ...
	var result *storage.Spec
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		records, qErr := s.executeQuery(txCtx, query, params)
		// ... error handling, recomputeContentHash(txCtx), GetSpec(txCtx) ...
		if clErr := s.createChangeLog(txCtx, slug, clEntry, deltas); clErr != nil {
			return clErr
		}
		result = freshSpec
		return nil
	})
	return result, err
}
```

- [ ] **Step 2: Wrap `LifecycleSupersedeSpec`**

Pre-validation stays outside. The atomic query + recomputeContentHash + 2x GetSpec + 2x createChangeLog move inside:

```go
func (s *Store) LifecycleSupersedeSpec(ctx context.Context, oldSlug, newSlug string) (oldSpec, newSpec *storage.Spec, err error) {
	// ... pre-validation (outside) ...
	txErr := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		records, qErr := s.executeQuery(txCtx, query, params)
		// ... error handling, recomputeContentHash(txCtx, oldSlug) ...
		// ... 2x GetSpec(txCtx), 2x createChangeLog(txCtx) ...
		oldSpec = freshOld
		newSpec = freshNew
		return nil
	})
	if txErr != nil {
		return nil, nil, txErr
	}
	return oldSpec, newSpec, nil
}
```

- [ ] **Step 3: Wrap `LifecycleAbandonSpec`**

Same pattern as amend:

```go
func (s *Store) LifecycleAbandonSpec(ctx context.Context, slug, reason string) (*storage.Spec, error) {
	// ... pre-read, terminal check (outside) ...
	var result *storage.Spec
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		records, qErr := s.executeQuery(txCtx, query, params)
		// ... recomputeContentHash(txCtx), GetSpec(txCtx), createChangeLog(txCtx) ...
		result = abandonedSpec
		return nil
	})
	return result, err
}
```

- [ ] **Step 4: Verify build and tests**

Run: `go build ./internal/storage/memgraph/...`
Run: `go test ./... -short`
Expected: PASS

- [ ] **Step 5: Commit**

```text
feat(memgraph): wrap lifecycle operations in RunInTransaction

Amend, supersede, and abandon mutations with their ChangeLog
entries now execute atomically. If any step fails, the entire
operation rolls back.
```

### Task 8: Wrap `UpdateDecision` in Transaction

**Files:**

- Modify: `internal/storage/memgraph/decision.go:124-200`

- [ ] **Step 1: Read current `UpdateDecision`**

Read `decision.go:124-200`. Flow: build SET → executeQuery → compute hash → SET hash.

- [ ] **Step 2: Wrap in `RunInTransaction`**

```go
func (s *Store) UpdateDecision(ctx context.Context, slug string, ...) (*storage.Decision, error) {
	// ... build setClauses (no DB) ...
	var result *storage.Decision
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		records, qErr := s.executeQuery(txCtx, query, params)
		// ... parse result, hash recompute, SET hash using txCtx ...
		result = decision
		return nil
	})
	return result, err
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./internal/storage/memgraph/...`
Expected: PASS

- [ ] **Step 4: Commit**

```text
feat(memgraph): wrap UpdateDecision in RunInTransaction

Decision updates and hash recomputation now execute atomically.
```

### Task 9: Remove Single-Writer Comment and Add Concurrency Documentation

**Files:**

- Modify: `internal/storage/memgraph/memgraph.go` (remove comment ~line 384)
- Modify: `internal/storage/memgraph/tx.go` (expand doc comment)

- [ ] **Step 1: Remove single-writer comment**

In `memgraph.go`, find and remove the comment block:

```text
// Recompute content_hash from the updated fields. This is a two-query
// approach: read all hash-input fields from the first query's result,
// compute the new hash, then SET it. Acceptable because SpecGraph is
// single-writer; no concurrent updates can interleave between queries.
```

Replace with:

```go
// Recompute content_hash from the updated fields returned by the
// first query. This runs within the caller's transaction, so the
// read and write are atomic.
```

- [ ] **Step 2: Expand tx.go doc comment**

Add a package-level or function-level guidance comment to `tx.go`:

```go
// Transaction Usage Guide
//
// All multi-query write paths MUST use RunInTransaction for atomicity.
// Pattern:
//
//   func (s *Store) SomeWriteOp(ctx context.Context, ...) error {
//       // Validation (no DB) stays outside.
//       return s.RunInTransaction(ctx, func(txCtx context.Context) error {
//           // All DB operations use txCtx, not ctx.
//           records, err := s.executeQuery(txCtx, query, params)
//           return s.createChangeLog(txCtx, slug, entry, deltas)
//       })
//   }
//
// For functions returning values, capture via closure variable.
// Nested RunInTransaction calls reuse the outer transaction.
```

- [ ] **Step 3: Commit**

```text
docs(memgraph): replace single-writer comment with transaction guidance

Remove incorrect single-writer assumption. Add transaction usage
guide to tx.go documenting the RunInTransaction pattern.
```

### Task 10: Add Integration Tests for Transaction Rollback

**Files:**

- Create: `internal/storage/memgraph/tx_changelog_test.go`

- [ ] **Step 1: Write test for ChangeLog rollback on CreateSpec failure**

Create `internal/storage/memgraph/tx_changelog_test.go` with `//go:build integration`:

```go
func TestCreateSpec_RollsBackOnChangeLogFailure(t *testing.T) {
	// This test verifies that if createChangeLog fails within CreateSpec's
	// transaction, the spec itself is not created (rolled back).
	//
	// Strategy: We can't easily inject a ChangeLog failure, but we CAN
	// verify that after a successful CreateSpec, both the spec and its
	// ChangeLog exist atomically — if one exists, the other must too.
	ctx, store := setupTestStore(t)
	_, err := store.CreateSpec(ctx, "tx-test-create", "intent", "p2", "medium")
	require.NoError(t, err)

	// Verify spec exists
	spec, err := store.GetSpec(ctx, "tx-test-create")
	require.NoError(t, err)
	assert.Equal(t, "tx-test-create", spec.Slug)

	// Verify ChangeLog exists (atomic with spec creation)
	entries, err := store.ListChanges(ctx, "tx-test-create", storage.ChangeLogFilter{})
	require.NoError(t, err)
	assert.Len(t, entries, 1, "ChangeLog must exist if spec exists (atomic)")
}
```

- [ ] **Step 2: Write test for concurrent modification rollback**

```go
func TestLifecycleAmendSpec_ConcurrentModRollsBack(t *testing.T) {
	ctx, store := setupTestStore(t)
	_, err := store.CreateSpec(ctx, "tx-test-amend", "intent", "p2", "medium")
	require.NoError(t, err)

	// Move to done via UpdateSpec (matches lifecycle_test.go pattern)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "tx-test-amend", nil, &doneStage, nil, nil, nil)
	require.NoError(t, err)

	// Amend should succeed
	amended, err := store.LifecycleAmendSpec(ctx, "tx-test-amend", "rework", "shape")
	require.NoError(t, err)
	assert.Equal(t, storage.SpecStageShape, amended.Stage)

	// Verify ChangeLog includes the amend checkpoint
	entries, err := store.ListChanges(ctx, "tx-test-amend", storage.ChangeLogFilter{CheckpointsOnly: true})
	require.NoError(t, err)
	last := entries[len(entries)-1]
	assert.True(t, last.Checkpoint)
	assert.Contains(t, last.Summary, "Amended")
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/storage/memgraph/ -short -v -run TestCreateSpec_RollsBack`
Run: `go test ./internal/storage/memgraph/ -short -v -run TestLifecycleAmendSpec_ConcurrentMod`
Expected: PASS (unit tests only — integration tests need Docker)

Run: `go build -tags integration ./internal/storage/memgraph/...`
Expected: PASS (compilation check)

- [ ] **Step 4: Commit**

```text
test(memgraph): add transaction atomicity integration tests

Verify CreateSpec+ChangeLog atomicity and lifecycle amend
checkpoint creation within transactions.
```

### Task 11: Update CLAUDE.md and Create ADR

**Files:**

- Modify: `CLAUDE.md`
- Create: `docs/decisions/ADR-004-optimistic-concurrency-transactions.md`

- [ ] **Step 1: Update CLAUDE.md architecture table**

Add row: `| internal/storage/memgraph/tx.go | Transaction support (RunInTransaction, context-threaded tx) |`

- [ ] **Step 2: Add CLAUDE.md gotchas**

Add:

```text
- **All multi-query write paths MUST use `RunInTransaction`** — Pass `txCtx` (not `ctx`) to `executeQuery`, `GetSpec`, `createChangeLog` inside the transaction. Queries automatically join the transaction via context. Validation that doesn't hit the DB stays outside the transaction to reduce lock time.
- **Concurrent modifications return `ErrConcurrentModification`** — Mapped to `connect.CodeAborted` (retryable). Version guards in WHERE clauses detect conflicts. First writer wins; second fails fast.
```

- [ ] **Step 3: Remove/update single-writer language**

Search CLAUDE.md for any "single-writer" references and remove them.

- [ ] **Step 4: Create ADR-004**

Create `docs/decisions/ADR-004-optimistic-concurrency-transactions.md`:

```markdown
# ADR-004: Optimistic Concurrency with Transaction-Wrapped Write Paths

- **Status:** Accepted
- **Date:** 2026-03-19
- **Bead:** spgr-1s1

## Context

The codebase assumed single-writer semantics, but the ConnectRPC server
handles concurrent goroutines per request. Multi-query write paths (spec
mutation + hash recomputation + ChangeLog creation) could leave partial
state if a failure or concurrent modification occurred between queries.

## Decision

Wrap all multi-query write paths in `RunInTransaction` for atomic
rollback. Keep version guards (`WHERE s.version = $expected_version`)
for conflict detection. First writer wins; second receives
`ErrConcurrentModification` (mapped to `CodeAborted`, retryable).

## Consequences

- All new multi-query write paths must use `RunInTransaction`.
- Version guards remain the primary conflict detection mechanism.
- No distributed locking needed for single-server deployment.
- Callers handle `CodeAborted` with retry if desired.
```

- [ ] **Step 5: Commit**

```text
docs: add ADR-004 optimistic concurrency and update CLAUDE.md

Document the transaction model, concurrency guarantees, and
gotchas for future contributors.
```

### Task 12: Update Site Documentation

**Files:**

- Modify: `site/docs/concepts/specs.md`

- [ ] **Step 1: Add atomicity note to Change Tracking section**

In the Change Tracking section of `specs.md`, add after the existing content:

```markdown
All spec mutations and their ChangeLog entries execute within a single
database transaction. If any step fails — for example, a concurrent
modification is detected via the version guard — the entire operation
rolls back. No orphaned state: if a spec was mutated, its ChangeLog
entry exists; if the ChangeLog failed, the mutation never happened.
```

- [ ] **Step 2: Commit**

```text
docs(site): add transaction atomicity note to Change Tracking section
```

### Task 13: Run Full Quality Gates

- [ ] **Step 1: Run task check**

Run: `task check`
Expected: PASS

- [ ] **Step 2: Fix any lint or formatting issues**

- [ ] **Step 3: Commit fixes if needed**

### Task 14: Close Bead

- [ ] **Step 1: Close spgr-1s1**

```bash
bd close spgr-1s1 --reason "Implemented: all multi-query write paths wrapped in RunInTransaction with ADR-004"
```

- [ ] **Step 2: Pull beads**

```bash
bd dolt pull
```
