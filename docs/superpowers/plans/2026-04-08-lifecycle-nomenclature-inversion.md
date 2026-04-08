# Lifecycle Nomenclature Inversion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the inverted amend/supersede semantics and remove the `amended` parking stage.

**Architecture:** Swap eligibility sets in storage layer (amend from `{approved, in_progress, review}`, supersede from `{done}` only), remove `SpecStageAmended` constant, require `re_entry_stage` on amend, and update handler/CLI/tests/docs to match.

**Tech Stack:** Go, Postgres (pgx v5), ConnectRPC, Cobra CLI, Ginkgo/Gomega E2E

---

### Task 1: Add New Error Sentinels and Update Domain Types

**Files:**
- Modify: `internal/storage/errors.go:47-68`
- Modify: `internal/storage/spec_domain.go:1-92`
- Create: `internal/storage/spec_domain_test.go`

- [ ] **Step 1: Write test for `IsAmendEligible` method**

In `internal/storage/spec_domain_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestSpecStage_IsAmendEligible(t *testing.T) {
	eligible := []storage.SpecStage{
		storage.SpecStageApproved,
		storage.SpecStageInProgress,
		storage.SpecStageReview,
	}
	for _, s := range eligible {
		require.True(t, s.IsAmendEligible(), "stage %q should be amend-eligible", s)
	}

	ineligible := []storage.SpecStage{
		storage.SpecStageSpark,
		storage.SpecStageShape,
		storage.SpecStageSpecify,
		storage.SpecStageDecompose,
		storage.SpecStageDone,
		storage.SpecStageSuperseded,
		storage.SpecStageAbandoned,
	}
	for _, s := range ineligible {
		require.False(t, s.IsAmendEligible(), "stage %q should not be amend-eligible", s)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/ -run TestSpecStage_IsAmendEligible -v`
Expected: FAIL with `IsAmendEligible` not found.

- [ ] **Step 3: Add `ErrSpecNotAmendable` and `ErrReEntryStageRequired` sentinels**

In `internal/storage/errors.go`, add to the lifecycle errors var block:

```go
// ErrSpecNotAmendable is returned when amend is attempted on a spec not in an eligible stage.
ErrSpecNotAmendable = errors.New("spec must be in approved, in_progress, or review stage to amend")
// ErrReEntryStageRequired is returned when amend is called without a re-entry stage.
ErrReEntryStageRequired = errors.New("re_entry_stage is required for amend")
```

- [ ] **Step 4: Remove `SpecStageAmended` and add `IsAmendEligible`**

In `internal/storage/spec_domain.go`:

1. Delete `SpecStageAmended SpecStage = "amended"` constant
2. Remove `SpecStageAmended` from `allSpecStages` slice
3. Remove `SpecStageAmended` from `ExcludesReEntry()` switch — keep `SpecStageDone`, `SpecStageSuperseded`, `SpecStageAbandoned`
4. Remove `SpecStageAmended` from `IsValid()` switch
5. Update type doc comment to remove "amended" from the list
6. Add method:

```go
// IsAmendEligible reports whether s is a stage from which amend is allowed.
// Only execution-adjacent stages (approved, in_progress, review) qualify.
func (s SpecStage) IsAmendEligible() bool {
	switch s {
	case SpecStageApproved, SpecStageInProgress, SpecStageReview:
		return true
	default:
		return false
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/storage/ -run TestSpecStage_IsAmendEligible -v`
Expected: PASS

- [ ] **Step 6: Note compilation errors (expected — fixed in later tasks)**

Run: `go build ./...`
Expected: Fails — other files still reference `SpecStageAmended`. Note them for Tasks 3-11.

- [ ] **Step 7: Commit**

```bash
git add internal/storage/errors.go internal/storage/spec_domain.go internal/storage/spec_domain_test.go
git commit -m "feat(storage): add IsAmendEligible, remove SpecStageAmended, add new error sentinels"
```

---

### Task 2: Update Storage Backend — `LifecycleAmendSpec`

**Files:**
- Modify: `internal/storage/postgres/lifecycle.go:32-102`
- Modify: `internal/storage/lifecycle.go:106-109`
- Modify: `internal/storage/postgres/lifecycle_test.go`

- [ ] **Step 1: Rewrite amend integration tests**

In `internal/storage/postgres/lifecycle_test.go`, replace the amend-related tests inside `TestLifecycle`:

Replace `AmendSpec_HappyPath` — use `in_progress` as source:

```go
t.Run("AmendSpec_HappyPath", func(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "amend-me", "Test spec", "p1", "medium")
	require.NoError(t, err)
	inProgressStage := "in_progress"
	_, err = store.UpdateSpec(ctx, "amend-me", nil, &inProgressStage, nil, nil, nil)
	require.NoError(t, err)

	amended, err := store.LifecycleAmendSpec(ctx, "amend-me", "Mobile needs offline refresh", "shape")
	require.NoError(t, err)
	require.Equal(t, storage.SpecStage("shape"), amended.Stage)
	require.Equal(t, int32(3), amended.Version)

	fetched, err := store.GetSpec(ctx, "amend-me")
	require.NoError(t, err)
	require.Equal(t, storage.SpecStage("shape"), fetched.Stage)
	require.Equal(t, int32(3), fetched.Version)
})
```

Replace `AmendSpec_DefaultToAmended` with `AmendSpec_RequiresReEntryStage`:

```go
t.Run("AmendSpec_RequiresReEntryStage", func(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "amend-no-reentry", "Test spec", "p1", "medium")
	require.NoError(t, err)
	inProgressStage := "in_progress"
	_, err = store.UpdateSpec(ctx, "amend-no-reentry", nil, &inProgressStage, nil, nil, nil)
	require.NoError(t, err)

	_, err = store.LifecycleAmendSpec(ctx, "amend-no-reentry", "needs rework", "")
	require.ErrorIs(t, err, storage.ErrReEntryStageRequired)
})
```

Replace `AmendSpec_NotDone` with two tests:

```go
t.Run("AmendSpec_NotAmendable_Spark", func(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "not-amendable", "Test spec", "p1", "medium")
	require.NoError(t, err)
	_, err = store.LifecycleAmendSpec(ctx, "not-amendable", "reason", "shape")
	require.ErrorIs(t, err, storage.ErrSpecNotAmendable)
})

t.Run("AmendSpec_NotAmendable_Done", func(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "amend-done", "Test spec", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "amend-done", nil, &doneStage, nil, nil, nil)
	require.NoError(t, err)

	_, err = store.LifecycleAmendSpec(ctx, "amend-done", "reason", "shape")
	require.ErrorIs(t, err, storage.ErrSpecNotAmendable)
})
```

Add test for all eligible stages:

```go
t.Run("AmendSpec_AllEligibleStages", func(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	for _, stage := range []string{"approved", "in_progress", "review"} {
		slug := "amend-from-" + stage
		_, err := store.CreateSpec(ctx, slug, "Test spec", "p1", "medium")
		require.NoError(t, err)
		_, err = store.UpdateSpec(ctx, slug, nil, &stage, nil, nil, nil)
		require.NoError(t, err)

		amended, err := store.LifecycleAmendSpec(ctx, slug, "reason for "+stage, "spark")
		require.NoError(t, err, "amend from %q should succeed", stage)
		require.Equal(t, storage.SpecStage("spark"), amended.Stage)
	}
})
```

Update `AmendSpec_InvalidReEntryStage` — remove "amended", use `in_progress` source:

```go
t.Run("AmendSpec_InvalidReEntryStage", func(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	for _, stage := range []string{"done", "superseded", "abandoned"} {
		slug := "amend-reentry-" + stage
		_, err := store.CreateSpec(ctx, slug, "Test spec", "p1", "medium")
		require.NoError(t, err)
		inProgressStage := "in_progress"
		_, err = store.UpdateSpec(ctx, slug, nil, &inProgressStage, nil, nil, nil)
		require.NoError(t, err)

		_, err = store.LifecycleAmendSpec(ctx, slug, "reason", stage)
		require.ErrorIs(t, err, storage.ErrInvalidReEntryStage, "stage %q should be rejected", stage)
	}
})
```

Update `AmendSpec_VersionGuard` — use `in_progress` source, `ErrSpecNotAmendable`:

```go
t.Run("AmendSpec_VersionGuard", func(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "toctou-amend", "Test spec", "p1", "medium")
	require.NoError(t, err)
	inProgressStage := "in_progress"
	_, err = store.UpdateSpec(ctx, "toctou-amend", nil, &inProgressStage, nil, nil, nil)
	require.NoError(t, err)

	const n = 2
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			_, aErr := store.LifecycleAmendSpec(ctx, "toctou-amend", "concurrent amend", "shape")
			errs <- aErr
		}()
	}

	var succeeded, failed int
	for i := 0; i < n; i++ {
		if e := <-errs; e != nil {
			failed++
			require.True(t,
				errors.Is(e, storage.ErrConcurrentModification) || errors.Is(e, storage.ErrSpecNotAmendable),
				"unexpected error: %v", e)
		} else {
			succeeded++
		}
	}
	require.Equal(t, 1, succeeded, "exactly one amend should succeed")
	require.Equal(t, 1, failed, "exactly one amend should fail")
})
```

Delete `AmendedSpec_CanBeAbandoned` and `AmendedSpec_CanBeSuperseded` tests entirely.

Delete `AcknowledgeDrift_AmendedStage` test entirely.

- [ ] **Step 2: Implement `LifecycleAmendSpec` changes**

In `internal/storage/postgres/lifecycle.go`, replace the `LifecycleAmendSpec` method (lines 32-101):

```go
// LifecycleAmendSpec transitions an in-flight spec back into an earlier authoring stage.
// The spec must be in an amend-eligible stage (approved, in_progress, review).
// reEntryStage is required — one of: spark, shape, specify, decompose.
// Returns ErrReEntryStageRequired if reEntryStage is empty,
// ErrSpecNotAmendable if the spec is not in an eligible stage, and
// ErrSpecNotFound if the spec does not exist.
func (s *Store) LifecycleAmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*storage.Spec, error) {
	if reEntryStage == "" {
		return nil, fmt.Errorf("amend spec %q: %w", slug, storage.ErrReEntryStageRequired)
	}
	targetStage := storage.SpecStage(reEntryStage)
	if targetStage.ExcludesReEntry() {
		return nil, fmt.Errorf("amend spec %q: re_entry_stage %q: %w", slug, reEntryStage, storage.ErrInvalidReEntryStage)
	}

	var result *storage.Spec
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		spec, getErr := s.GetSpec(txCtx, slug)
		if getErr != nil {
			return fmt.Errorf("postgres: amend spec: pre-read %q: %w", slug, getErr)
		}

		// Version guard: only proceed if spec is in an amend-eligible stage.
		tag, execErr := s.exec(txCtx,
			`UPDATE specs SET stage = $1, version = version + 1, updated_at = $2
			 WHERE slug = $3 AND project_slug = $4 AND version = $5
			   AND stage IN ('approved', 'in_progress', 'review')`,
			string(targetStage), s.now(), slug, s.project, spec.Version,
		)
		if execErr != nil {
			return fmt.Errorf("postgres: amend spec: %w", execErr)
		}
		if tag.RowsAffected() == 0 {
			return s.preconditionError(txCtx, slug, "amend spec", func(current *storage.Spec) error {
				if current.Version != spec.Version {
					return fmt.Errorf("amend spec %q: %w", slug, storage.ErrConcurrentModification)
				}
				if !current.Stage.IsAmendEligible() {
					return fmt.Errorf("amend spec %q (stage=%s): %w", slug, current.Stage, storage.ErrSpecNotAmendable)
				}
				return nil
			})
		}

		if hashErr := s.recomputeContentHash(txCtx, slug); hashErr != nil {
			return hashErr
		}
		freshSpec, getErr := s.GetSpec(txCtx, slug)
		if getErr != nil {
			return getErr
		}

		summary := fmt.Sprintf("Amended from %s, re-entering at: %s", spec.Stage, targetStage)
		deltas := []storage.FieldChange{{Field: "stage", OldValue: string(spec.Stage), NewValue: string(targetStage)}}
		clEntry := &storage.ChangeLogEntry{
			Version:     freshSpec.Version,
			Stage:       string(freshSpec.Stage),
			ContentHash: freshSpec.ContentHash,
			Checkpoint:  true,
			Summary:     summary,
			Reason:      reason,
			Date:        freshSpec.UpdatedAt,
		}
		if clErr := s.createChangeLog(txCtx, slug, clEntry, deltas); clErr != nil {
			return clErr
		}
		result = freshSpec
		return nil
	})
	return result, err
}
```

- [ ] **Step 3: Update `LifecycleBackend` interface comment**

In `internal/storage/lifecycle.go`, update:

```go
// LifecycleAmendSpec transitions an in-flight spec back into authoring.
// The spec must be in an amend-eligible stage (approved, in_progress, review).
// reEntryStage is required (spark, shape, specify, decompose).
// Returns ErrSpecNotFound, ErrSpecNotAmendable, ErrReEntryStageRequired, or ErrSpecTerminal.
LifecycleAmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*Spec, error)
```

- [ ] **Step 4: Run integration tests**

Run: `go test ./internal/storage/postgres/ -tags integration -run TestLifecycle -v -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/lifecycle.go internal/storage/postgres/lifecycle_test.go internal/storage/lifecycle.go
git commit -m "feat(storage): amend from in-flight stages, require re_entry_stage"
```

---

### Task 3: Update Storage Backend — `LifecycleSupersedeSpec`

**Files:**
- Modify: `internal/storage/postgres/lifecycle.go:104-238`
- Modify: `internal/storage/postgres/lifecycle_test.go`

- [ ] **Step 1: Update supersede tests**

In `internal/storage/postgres/lifecycle_test.go`:

Update `SupersedeSpec_HappyPath` — advance old spec to `done`:

```go
t.Run("SupersedeSpec_HappyPath", func(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "old-lifecycle", "Old spec", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "old-lifecycle", nil, &doneStage, nil, nil, nil)
	require.NoError(t, err)

	_, err = store.CreateSpec(ctx, "new-lifecycle", "New spec", "p1", "medium")
	require.NoError(t, err)

	old, newSpec, err := store.LifecycleSupersedeSpec(ctx, "old-lifecycle", "new-lifecycle")
	require.NoError(t, err)
	require.Equal(t, storage.SpecStageSuperseded, old.Stage)
	require.Equal(t, "new-lifecycle", old.SupersededBy)
	require.Equal(t, "old-lifecycle", newSpec.Supersedes)
})
```

Update `SupersedeSpec_EdgePersists` — advance old spec to `done`:

```go
t.Run("SupersedeSpec_EdgePersists", func(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "edge-old", "Old spec", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "edge-old", nil, &doneStage, nil, nil, nil)
	require.NoError(t, err)

	_, err = store.CreateSpec(ctx, "edge-new", "New spec", "p1", "medium")
	require.NoError(t, err)

	_, _, err = store.LifecycleSupersedeSpec(ctx, "edge-old", "edge-new")
	require.NoError(t, err)

	edges, err := store.ListEdges(ctx, "edge-new", storage.EdgeTypeSupersedes)
	require.NoError(t, err)
	require.Len(t, edges, 1)
	require.Equal(t, "edge-new", edges[0].FromID)
	require.Equal(t, "edge-old", edges[0].ToID)
})
```

Add `SupersedeSpec_NotDone` — supersede from non-done fails:

```go
t.Run("SupersedeSpec_NotDone", func(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "not-done-old", "Old spec", "p1", "medium")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "not-done-new", "New spec", "p1", "medium")
	require.NoError(t, err)

	_, _, err = store.LifecycleSupersedeSpec(ctx, "not-done-old", "not-done-new")
	require.ErrorIs(t, err, storage.ErrSpecNotDone)
})
```

Update `SupersedeSpec_TerminalState` — expect `ErrSpecNotDone`:

```go
t.Run("SupersedeSpec_TerminalState", func(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "terminal-old", "Old spec", "p1", "medium")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "replacement", "New spec", "p1", "medium")
	require.NoError(t, err)

	_, err = store.LifecycleAbandonSpec(ctx, "terminal-old", "abandoned")
	require.NoError(t, err)

	_, _, err = store.LifecycleSupersedeSpec(ctx, "terminal-old", "replacement")
	require.ErrorIs(t, err, storage.ErrSpecNotDone)
})
```

- [ ] **Step 2: Implement `LifecycleSupersedeSpec` changes**

In `internal/storage/postgres/lifecycle.go`, in the `LifecycleSupersedeSpec` method:

Replace the old spec eligibility check:
```go
// Before:
if terminalStages[oldCheck.Stage] {
	return fmt.Errorf("supersede spec %q (stage=%s): %w", oldSlug, oldCheck.Stage, storage.ErrSpecTerminal)
}

// After:
if oldCheck.Stage != storage.SpecStageDone {
	return fmt.Errorf("supersede spec %q (stage=%s): %w", oldSlug, oldCheck.Stage, storage.ErrSpecNotDone)
}
```

Replace the old spec UPDATE query to use `AND stage = 'done'` instead of `AND stage NOT IN (...)`:
```go
oldTag, oldErr := s.exec(txCtx,
	`UPDATE specs SET stage = $1, superseded_by = $2, version = version + 1, updated_at = $3
	 WHERE slug = $4 AND project_slug = $5 AND version = $6 AND stage = 'done'`,
	string(storage.SpecStageSuperseded), newSlug, now,
	oldSlug, s.project, oldCheck.Version,
)
```

- [ ] **Step 3: Run integration tests**

Run: `go test ./internal/storage/postgres/ -tags integration -run TestLifecycle -v -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/storage/postgres/lifecycle.go internal/storage/postgres/lifecycle_test.go
git commit -m "feat(storage): supersede restricted to done-only specs"
```

---

### Task 4: Update `TestLifecycle_AmendRefreshesEdgeHash`

**Files:**
- Modify: `internal/storage/postgres/lifecycle_test.go`

- [ ] **Step 1: Update the test to use `in_progress` as amend source**

Replace the `TestLifecycle_AmendRefreshesEdgeHash` function:

```go
func TestLifecycle_AmendRefreshesEdgeHash(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "upstream-hash", "Upstream spec", "p1", "medium")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "downstream-hash", "Downstream spec", "p1", "medium")
	require.NoError(t, err)

	_, err = store.AddEdge(ctx, "downstream-hash", "upstream-hash", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	initialDeps, err := store.GetDependenciesWithEdgeData(ctx, "downstream-hash")
	require.NoError(t, err)
	require.Len(t, initialDeps, 1)
	initialHashAtLink := initialDeps[0].ContentHashAtLink

	// Advance upstream to in_progress so it can be amended.
	_, err = store.UpdateSpec(ctx, "upstream-hash", nil, strPtr("in_progress"), nil, nil, nil)
	require.NoError(t, err)

	// Amend the upstream spec back to shape.
	_, err = store.LifecycleAmendSpec(ctx, "upstream-hash", "needs revision", "shape")
	require.NoError(t, err)

	upstream, err := store.GetSpec(ctx, "upstream-hash")
	require.NoError(t, err)
	require.NotEqual(t, initialHashAtLink, upstream.ContentHash, "amend should change the upstream content hash")

	// Advance upstream back to done — this should refresh the edge hash.
	_, err = store.UpdateSpec(ctx, "upstream-hash", nil, strPtr("done"), nil, nil, nil)
	require.NoError(t, err)

	refreshedDeps, err := store.GetDependenciesWithEdgeData(ctx, "downstream-hash")
	require.NoError(t, err)
	require.Len(t, refreshedDeps, 1)
	require.NotEqual(t, initialHashAtLink, refreshedDeps[0].ContentHashAtLink,
		"ContentHashAtLink should be refreshed after upstream is re-completed")
}
```

- [ ] **Step 2: Run test**

Run: `go test ./internal/storage/postgres/ -tags integration -run TestLifecycle_AmendRefreshesEdgeHash -v -count=1`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/storage/postgres/lifecycle_test.go
git commit -m "test(storage): update AmendRefreshesEdgeHash for new amend eligibility"
```

---

### Task 5: Update Drift Engine and Drift Acknowledgment

**Files:**
- Modify: `internal/drift/drift.go`
- Modify: `internal/storage/postgres/lifecycle.go` (LifecycleAcknowledgeDrift)
- Modify: `internal/storage/errors.go`

- [ ] **Step 1: Update drift engine — remove amended eligibility**

In `internal/drift/drift.go`:

Update the comment on `maxSpecsPerCheck`:
```go
// maxSpecsPerCheck limits the number of specs returned per ListSpecs call.
const maxSpecsPerCheck = 10000
```

In the single-spec check path, replace:
```go
if spec.Stage != storage.SpecStageDone && spec.Stage != storage.SpecStageAmended {
```
With:
```go
if spec.Stage != storage.SpecStageDone {
```

In the all-specs path, remove the `amendedSpecs` fetch and append. The code should go from fetching `doneSpecs` directly to the `allSpecs` count:
```go
specs = append(specs, doneSpecs...)
// Remove the amendedSpecs fetch/append entirely
```

Update the `SkippedCount` field comment in `CheckResult`:
```go
SkippedCount int32 // specs not in done stage (all-specs mode only)
```

- [ ] **Step 2: Update `LifecycleAcknowledgeDrift` eligible stages**

In `internal/storage/postgres/lifecycle.go`, in `LifecycleAcknowledgeDrift`:

Replace:
```go
eligibleStages := []string{string(storage.SpecStageDone), string(storage.SpecStageAmended)}
```
With:
```go
eligibleStages := []string{string(storage.SpecStageDone)}
```

- [ ] **Step 3: Update `ErrSpecIneligibleForDrift` message**

In `internal/storage/errors.go`:
```go
ErrSpecIneligibleForDrift = errors.New("spec is not eligible for drift checking (must be done)")
```

- [ ] **Step 4: Run drift tests**

Run: `go test ./internal/drift/ -v`
Expected: PASS (or fix any tests referencing `amended`)

- [ ] **Step 5: Commit**

```bash
git add internal/drift/drift.go internal/storage/postgres/lifecycle.go internal/storage/errors.go
git commit -m "feat(drift): remove amended stage from drift eligibility"
```

---

### Task 6: Update Handler and Error Mapping

**Files:**
- Modify: `internal/server/lifecycle_handler.go`

- [ ] **Step 1: Require `re_entry_stage` in handler**

In `TransitionAmend` (after the `validateRequiredField("reason", ...)` block), add:

```go
if msg.ReEntryStage == "" {
	return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("re_entry_stage is required"))
}
```

- [ ] **Step 2: Update `lifecycleError` error mappings**

Add new mappings and update existing messages:

```go
if errors.Is(err, storage.ErrSpecNotAmendable) {
	return connect.NewError(connect.CodeFailedPrecondition, errors.New(specMsg(slug, "is not in an amend-eligible stage (must be approved, in_progress, or review); use supersede for completed specs")))
}
if errors.Is(err, storage.ErrReEntryStageRequired) {
	return connect.NewError(connect.CodeInvalidArgument, errors.New("re_entry_stage is required — one of: spark, shape, specify, decompose"))
}
```

Update `ErrSpecNotDone` message:
```go
if errors.Is(err, storage.ErrSpecNotDone) {
	return connect.NewError(connect.CodeFailedPrecondition, errors.New(specMsg(slug, "must be in done stage; use amend for in-flight specs")))
}
```

Update `ErrSpecIneligibleForDrift` message:
```go
if errors.Is(err, storage.ErrSpecIneligibleForDrift) {
	return connect.NewError(connect.CodeFailedPrecondition, errors.New(specMsg(slug, "is not eligible for drift checking (must be done)")))
}
```

- [ ] **Step 3: Update handler doc comments**

```go
// TransitionAmend handles the TransitionAmend RPC, transitioning an in-flight spec
// (approved, in_progress, or review) to an earlier authoring stage. re_entry_stage is required.
```

Also update `CheckDrift` doc comment:
```go
// CheckDrift handles the CheckDrift RPC, returning drift reports for a spec.
// An empty slug checks all eligible (done) specs.
```

- [ ] **Step 4: Run handler tests**

Run: `go test ./internal/server/ -v`
Expected: Check for failures referencing `amended` or old semantics; fix as needed.

- [ ] **Step 5: Commit**

```bash
git add internal/server/lifecycle_handler.go
git commit -m "feat(handler): require re_entry_stage, add cross-referencing error messages"
```

---

### Task 7: Fix All Remaining `SpecStageAmended` References

**Files:**
- Various — any file still referencing `SpecStageAmended` or `"amended"` in Go code

- [ ] **Step 1: Find all remaining references**

Run: `grep -rn 'SpecStageAmended\|"amended"' --include='*.go' .`

Common locations to fix:
- `internal/drift/drift_test.go` — update tests referencing amended
- `internal/server/lifecycle_handler_test.go` — update handler test mocks
- `internal/server/error_mapper_internal_test.go` — update error mapping tests
- Any render files referencing amended

For each file: remove or update the `amended` reference to match new semantics.

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: PASS — no compilation errors.

- [ ] **Step 3: Run all unit tests**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "fix: remove all remaining SpecStageAmended references"
```

---

### Task 8: Update CLI

**Files:**
- Modify: `cmd/specgraph/lifecycle.go`

- [ ] **Step 1: Make `--re-entry` required and update descriptions**

In `cmd/specgraph/lifecycle.go`:

Update `amendCmd`:
```go
var amendCmd = &cobra.Command{
	Use:   "amend <slug>",
	Short: "Amend an in-flight spec, returning it to an earlier authoring stage",
	Args:  cobra.ExactArgs(1),
	RunE:  runAmend,
}
```

Update `supersedeCmd`:
```go
var supersedeCmd = &cobra.Command{
	Use:   "supersede <slug>",
	Short: "Supersede a completed (done) spec with a new one",
	Args:  cobra.ExactArgs(1),
	RunE:  runSupersede,
}
```

In `init()`, update the `--re-entry` flag:
```go
amendCmd.Flags().StringVar(&amendReEntry, "re-entry", "", "authoring stage to re-enter (spark|shape|specify|decompose)")
cobra.CheckErr(amendCmd.MarkFlagRequired("re-entry"))
```

- [ ] **Step 2: Build**

Run: `go build ./cmd/specgraph/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add cmd/specgraph/lifecycle.go
git commit -m "feat(cli): require --re-entry on amend, update command descriptions"
```

---

### Task 9: Update Proto Comments

**Files:**
- Modify: `proto/specgraph/v1/lifecycle.proto`

- [ ] **Step 1: Update `TransitionAmendRequest` comment**

```protobuf
message TransitionAmendRequest {
  string slug = 1;
  // Required. Reason for amending the spec. Maximum 10000 characters.
  string reason = 2;
  // Required. Authoring stage to re-enter: spark, shape, specify, or decompose.
  // The spec must be in an amend-eligible stage (approved, in_progress, review).
  // Terminal states (superseded, abandoned) and done are rejected.
  string re_entry_stage = 3;
}
```

- [ ] **Step 2: Update drift comments — remove "amended"**

`DriftCheckRequest.slug`:
```protobuf
// Empty slug checks all eligible (done) specs.
string slug = 1;
```

`DriftCheckResponse.skipped_count`:
```protobuf
// Number of specs skipped because they are not in done stage.
// Only set for all-specs checks (empty slug); zero for single-spec checks.
int32 skipped_count = 2;
```

- [ ] **Step 3: Regenerate proto**

Run: `task proto`

- [ ] **Step 4: Commit**

```bash
git add proto/specgraph/v1/lifecycle.proto gen/specgraph/v1/
git commit -m "docs(proto): update lifecycle.proto comments for amend/supersede inversion"
```

---

### Task 10: Add Postgres Migration

**Files:**
- Create: `internal/storage/postgres/migrations/005_remove_amended_stage.sql`

- [ ] **Step 1: Write the migration**

```sql
-- +goose Up
-- Move any specs in the removed "amended" stage to "spark" (safe re-entry default).
UPDATE specs SET stage = 'spark' WHERE stage = 'amended';

-- +goose Down
-- No-op: cannot restore the original stage since we don't know what it was.
```

- [ ] **Step 2: Commit**

```bash
git add internal/storage/postgres/migrations/005_remove_amended_stage.sql
git commit -m "feat(migration): move amended specs to spark stage"
```

---

### Task 11: Update E2E Tests

**Files:**
- Modify: `e2e/api/lifecycle_test.go`

- [ ] **Step 1: Rewrite "Amend flow" — amend from `in_progress`**

```go
Describe("Amend flow", func() {
	const amendSlug = "lifecycle-amend-spec"

	It("creates a spec and advances to in_progress", func() {
		_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   amendSlug,
			Intent: "Test amend lifecycle flow",
		}))
		Expect(err).NotTo(HaveOccurred())

		Expect(advanceStage(ctx, amendSlug, "in_progress")).To(Succeed())

		resp, err := specClient.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: amendSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetSpec().GetStage()).To(Equal("in_progress"))
	})

	It("amends the in-flight spec back into authoring with re-entry stage", func() {
		resp, err := lifecycleClient.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
			Slug:         amendSlug,
			Reason:       "Requirements changed during implementation",
			ReEntryStage: "shape",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetSpec().GetSlug()).To(Equal(amendSlug))
		Expect(resp.Msg.GetSpec().GetStage()).To(Equal("shape"))
		Expect(resp.Msg.GetSpec().GetVersion()).To(BeNumerically(">=", int32(2)))
	})

	It("verifies changelog has a checkpoint entry with reason and stage delta", func() {
		resp, err := specClient.ListChanges(ctx, connect.NewRequest(&specv1.ListChangesRequest{
			Slug:            amendSlug,
			CheckpointsOnly: true,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetEntries()).NotTo(BeEmpty())

		var amendEntry *specv1.ChangeLogEntry
		for _, e := range resp.Msg.GetEntries() {
			if e.GetCheckpoint() && e.GetReason() == "Requirements changed during implementation" {
				amendEntry = e
				break
			}
		}
		Expect(amendEntry).NotTo(BeNil())

		var stageChange *specv1.FieldChange
		for _, c := range amendEntry.GetChanges() {
			if c.GetField() == "stage" {
				stageChange = c
				break
			}
		}
		Expect(stageChange).NotTo(BeNil())
		Expect(stageChange.GetOldValue()).To(Equal("in_progress"))
		Expect(stageChange.GetNewValue()).To(Equal("shape"))
	})
})
```

- [ ] **Step 2: Delete "Amend flow (default stage)" Describe block entirely**

Remove the entire `Describe("Amend flow (default stage)", ...)` block.

- [ ] **Step 3: Update "Supersede flow" — advance old spec to `done`**

In the first It block, advance old spec to done before superseding:

```go
It("creates two specs and advances old to done", func() {
	_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:   oldSlug,
		Intent: "Original spec to be superseded",
	}))
	Expect(err).NotTo(HaveOccurred())

	Expect(advanceStage(ctx, oldSlug, "done")).To(Succeed())

	_, err = specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:   newSlug,
		Intent: "Replacement spec",
	}))
	Expect(err).NotTo(HaveOccurred())
})
```

- [ ] **Step 4: Update error paths**

Add test for missing `re_entry_stage`:
```go
It("rejects amend without re_entry_stage with InvalidArgument", func() {
	errSlug := "lifecycle-err-amend-noreentry-" + time.Now().Format("150405")
	_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:   errSlug,
		Intent: "Test amend without re-entry",
	}))
	Expect(err).NotTo(HaveOccurred())

	Expect(advanceStage(ctx, errSlug, "in_progress")).To(Succeed())

	_, err = lifecycleClient.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:   errSlug,
		Reason: "should fail",
	}))
	Expect(err).To(HaveOccurred())
	Expect(connect.CodeOf(err)).To(Equal(connect.CodeInvalidArgument))
})
```

Add test for amend-on-done:
```go
It("rejects amend on a done spec with FailedPrecondition", func() {
	errSlug := "lifecycle-err-amend-done-" + time.Now().Format("150405")
	_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:   errSlug,
		Intent: "Test amend on done spec",
	}))
	Expect(err).NotTo(HaveOccurred())

	Expect(advanceStage(ctx, errSlug, "done")).To(Succeed())

	_, err = lifecycleClient.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:         errSlug,
		Reason:       "should fail on done",
		ReEntryStage: "shape",
	}))
	Expect(err).To(HaveOccurred())
	Expect(connect.CodeOf(err)).To(Equal(connect.CodeFailedPrecondition))
})
```

Add test for supersede-on-non-done:
```go
It("rejects supersede on a non-done spec with FailedPrecondition", func() {
	errSlug := "lifecycle-err-supersede-notdone-" + time.Now().Format("150405")
	_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:   errSlug,
		Intent: "Test supersede on non-done spec",
	}))
	Expect(err).NotTo(HaveOccurred())

	_, err = lifecycleClient.TransitionSupersede(ctx, connect.NewRequest(&specv1.TransitionSupersedeRequest{
		Slug:    errSlug,
		NewSlug: "nonexistent-spec-xyz",
	}))
	Expect(err).To(HaveOccurred())
	Expect(connect.CodeOf(err)).To(Equal(connect.CodeFailedPrecondition))
})
```

Update existing "rejects amend on spark" and "rejects amend on shape" tests to include `ReEntryStage: "shape"` in the request (handler now requires it, so without it you get InvalidArgument instead of FailedPrecondition). Update the existing "rejects amend on a superseded (terminal) spec" test similarly.

- [ ] **Step 5: Commit**

```bash
git add e2e/api/lifecycle_test.go
git commit -m "test(e2e): update lifecycle E2E tests for amend/supersede inversion"
```

---

### Task 12: Update Plugin/Skill

**Files:**
- Modify: `plugin/specgraph/skills/specgraph/SKILL.md`

- [ ] **Step 1: Extend stage-routing table**

In Step 3A routing table, after the `approved` row, add:

```markdown
| in_progress | "Work is underway. Need to amend the spec?" | `specgraph amend <slug> --re-entry <stage> --reason "..."` |
| review | "In review. Need to amend the spec?" | `specgraph amend <slug> --re-entry <stage> --reason "..."` |
| done | "This spec is complete. Supersede or start something new?" | `specgraph supersede <slug> --with <new-slug>` |
```

- [ ] **Step 2: Commit**

```bash
git add plugin/specgraph/skills/specgraph/SKILL.md
git commit -m "docs(skill): extend router table for execution/lifecycle stages"
```

---

### Task 13: Update Site Documentation

**Files:**
- Modify: `site/docs/concepts/lifecycle.md`
- Modify: `site/docs/concepts/authoring.md`
- Scan: remaining doc files

- [ ] **Step 1: Rewrite `lifecycle.md`**

Key changes:
- Decision tree: amend from in-flight stages, supersede from done
- Eligibility table: amend `{approved, in_progress, review}`, supersede `{done}`
- State diagram: remove `done --> amended`, add `approved/in_progress/review --> [authoring stage]`
- Remove all `amended` stage references
- Update "when to use" examples

- [ ] **Step 2: Update `authoring.md` state diagram**

Remove `done --> amended` transition. Add amend paths from execution stages.

- [ ] **Step 3: Scan and fix remaining files**

Run: `grep -rn "amended" site/docs/ --include="*.md"`

Fix each reference.

- [ ] **Step 4: Commit**

```bash
git add site/docs/
git commit -m "docs(site): rewrite lifecycle docs for amend/supersede inversion"
```

---

### Task 14: Run Full Quality Gates

- [ ] **Step 1: Run `task check`**

Run: `task check`
Expected: PASS

- [ ] **Step 2: Run `task pr-prep`**

Run: `task pr-prep`
Expected: PASS

- [ ] **Step 3: Fix any failures and commit**
