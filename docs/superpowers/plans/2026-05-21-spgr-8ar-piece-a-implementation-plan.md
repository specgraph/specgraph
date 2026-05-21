# spgr-8ar Piece A — Storage Gap + Export Round-Trip Fix

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the storage-direct constitution-read gaps (PrimeData + export.engine) by routing both through a new `GetMergedConstitution` method, thread constitution provenance through `PrimeData`, and fix the export round-trip bug by bumping export schema to v2 with a list of layers.

**Architecture:** One new storage interface method (`GetMergedConstitution`) implemented as `GetAllLayers` + `merge.Layers`. `PrimeData` gains a `ConstitutionProvenance` field (domain only — no proto change in Piece A). Export bumps `CurrentSchemaVersion` from 1 to 2 with a `Constitutions []` field replacing the single `Constitution` field; import handles both schema versions with cross-field validation that rejects mismatched documents.

**Tech Stack:** Go, pgx v5, ConnectRPC, existing `internal/constitution/merge` package, existing `spaolacci/murmur3`. No new external dependencies in Piece A.

**Reference spec:** [docs/superpowers/specs/2026-05-21-multi-layer-constitution-completion-design.md](../specs/2026-05-21-multi-layer-constitution-completion-design.md) — Sections 1, 2, 3, 11 (Piece D's CI guard is set up but Piece D itself ships later); Section 14 invariants 1, 2, 7, 9.

---

## File Structure

### Files created

| Path | Responsibility |
|---|---|
| `internal/storage/postgres/get_merged_constitution_test.go` | Postgres-level tests for new method (empty, single-layer, multi-layer) |

### Files modified

| Path | Change |
|---|---|
| `internal/storage/constitution.go` | Add `GetMergedConstitution` to `ConstitutionBackend`; mark `GetConstitution` `// Deprecated` |
| `internal/storage/postgres/constitution.go` | Add `GetMergedConstitution` implementation |
| `internal/storage/execution_domain.go` | Add `ProvenanceEntry` type; add `ConstitutionProvenance []ProvenanceEntry` to `PrimeData` |
| `internal/storage/postgres/execution.go` | `GetPrimeData` switches from `GetConstitution` to `GetMergedConstitution`; populate provenance |
| `internal/storage/postgres/execution_test.go` | Update existing prime tests; add multi-layer provenance test |
| `internal/server/test_scoper_test.go` | Add `GetMergedConstitution` stub to `stubBackend` |
| `internal/server/constitution_handler_test.go` | Add stub to `mockConstitutionBackend` |
| `internal/server/sync_handler_test.go` | Add stub to `syncTestBackend` |
| `internal/server/error_sanitize_test.go` | Add stub to `errorBackend` (returns sentinel error) |
| `internal/server/execution_handler_test.go` | Update `mockExecutionBackend.seedPrime` to accept provenance |
| `internal/export/schema.go` | Bump `CurrentSchemaVersion` to 2; add `Constitutions` field to `Data` |
| `internal/export/engine.go` | `collect` uses `GetAllLayers`; `writeEntities` schema-version-aware with cross-field validation |
| `internal/export/engine_test.go` | New tests for v1→v2 migration, cross-field validation, multi-layer export |

---

## Prerequisites

- Working tree on `main` with no uncommitted changes (the design doc commit `ulvupoow` is the parent).
- Postgres available for integration tests (`task test:integration`).
- `task tools` has been run; `golangci-lint` and friends installed.

---

## Task 0: Set up isolated workspace

**Files:** none (workspace setup)

- [ ] **Step 1: Create the workspace**

Run:

```bash
jj --no-pager workspace add ../specgraph-8ar-piece-a
```

Expected output: confirmation that the workspace is created and a new working copy is initialized.

- [ ] **Step 2: Switch to the workspace and base on the design commit**

Run:

```bash
cd ../specgraph-8ar-piece-a
jj --no-pager new ulvupoow -m "(working) piece A"
```

Expected: `@` is a new empty change with the design commit as parent.

- [ ] **Step 3: Verify clean state**

Run:

```bash
jj --no-pager status
```

Expected output: "Working copy: (empty)" with parent the design commit.

---

## Task 1: Add `GetMergedConstitution` to interface + Postgres impl + tests

**Files:**

- Modify: `internal/storage/constitution.go`
- Modify: `internal/storage/postgres/constitution.go`
- Create: `internal/storage/postgres/get_merged_constitution_test.go`

- [ ] **Step 1: Write failing tests for `GetMergedConstitution`**

Create `internal/storage/postgres/get_merged_constitution_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/storage"
)

func TestGetMergedConstitution_Empty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.GetMergedConstitution(ctx)

	require.Error(t, err)
	assert.True(t, errors.Is(err, storage.ErrConstitutionNotFound),
		"empty project must return ErrConstitutionNotFound, got %v", err)
}

func TestGetMergedConstitution_SingleLayer(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.UpdateConstitution(ctx, &storage.Constitution{
		Name:  "test",
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "Prefer explicit over implicit"},
		},
	})
	require.NoError(t, err)

	result, err := store.GetMergedConstitution(ctx)

	require.NoError(t, err)
	require.NotNil(t, result.Constitution)
	assert.Len(t, result.Constitution.Principles, 1)
	assert.Equal(t, "p1", result.Constitution.Principles[0].ID)
	assert.Equal(t, storage.ConstitutionLayerProject,
		result.Provenance["principles[p1]"],
		"provenance must attribute p1 to project layer")
}

func TestGetMergedConstitution_MultiLayer(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.UpdateConstitution(ctx, &storage.Constitution{
		Name:  "org",
		Layer: storage.ConstitutionLayerOrg,
		Principles: []storage.Principle{
			{ID: "p-org", Statement: "Org rule"},
			{ID: "p-shared", Statement: "Org's version"},
		},
	})
	require.NoError(t, err)

	_, err = store.UpdateConstitution(ctx, &storage.Constitution{
		Name:  "project",
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p-proj", Statement: "Project rule"},
			{ID: "p-shared", Statement: "Project's version"},
		},
	})
	require.NoError(t, err)

	result, err := store.GetMergedConstitution(ctx)

	require.NoError(t, err)
	require.NotNil(t, result.Constitution)
	assert.Len(t, result.Constitution.Principles, 3, "merge must yield org+proj unique + shared key")

	// Provenance attributes shared key to highest-precedence layer (project)
	assert.Equal(t, storage.ConstitutionLayerProject,
		result.Provenance["principles[p-shared]"],
		"shared key must be attributed to project (highest precedence)")
	assert.Equal(t, storage.ConstitutionLayerOrg,
		result.Provenance["principles[p-org]"])
	assert.Equal(t, storage.ConstitutionLayerProject,
		result.Provenance["principles[p-proj]"])
}
```

- [ ] **Step 2: Run tests to verify they fail with a compile error**

Run:

```bash
go test -tags integration ./internal/storage/postgres/ -run TestGetMergedConstitution -v
```

Expected: compile error — `s.GetMergedConstitution undefined`.

- [ ] **Step 3: Add `GetMergedConstitution` to the storage interface**

Edit `internal/storage/constitution.go` — replace the entire `ConstitutionBackend` interface block with:

```go
// ConstitutionBackend defines storage operations for the project constitution.
type ConstitutionBackend interface {
	// GetConstitution returns the active constitution.
	//
	// Deprecated: returns only the single highest-precedence layer with no
	// provenance. Use GetMergedConstitution for the effective constitution
	// across all layers. This method is removed in spgr-8ar Piece D once all
	// callers migrate.
	GetConstitution(ctx context.Context) (*Constitution, error)

	// GetConstitutionLayer returns a single layer's raw constitution data.
	// Returns ErrConstitutionNotFound if the layer does not exist.
	GetConstitutionLayer(ctx context.Context, layer ConstitutionLayer) (*Constitution, error)

	// GetAllLayers returns all constitution layers for the project,
	// ordered by precedence (user, org, project, domain).
	// Returns an empty slice (not error) if no layers exist.
	GetAllLayers(ctx context.Context) ([]*Constitution, error)

	// GetMergedConstitution returns all layers composed into a single
	// constitution plus per-field provenance. The single source of truth
	// for "the effective constitution."
	//
	// Returns ErrConstitutionNotFound if no layers exist.
	GetMergedConstitution(ctx context.Context) (*merge.Result, error)

	// UpdateConstitution stores or replaces a constitution layer,
	// bumping its version. The layer is determined by constitution.Layer.
	UpdateConstitution(ctx context.Context, constitution *Constitution) (*Constitution, error)
}
```

Add the merge import at the top:

```go
import (
	"context"

	"github.com/specgraph/specgraph/internal/constitution/merge"
)
```

- [ ] **Step 4: Implement `GetMergedConstitution` on the Postgres store**

Append to `internal/storage/postgres/constitution.go` (after the existing `UpdateConstitution`):

```go
// GetMergedConstitution returns all layers composed into a single
// constitution plus per-field provenance.
// Returns ErrConstitutionNotFound if no layers exist.
func (s *Store) GetMergedConstitution(ctx context.Context) (*merge.Result, error) {
	layers, err := s.GetAllLayers(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: get merged constitution: %w", err)
	}
	if len(layers) == 0 {
		return nil, fmt.Errorf("postgres: %w", storage.ErrConstitutionNotFound)
	}
	result, mergeErr := merge.Layers(layers)
	if mergeErr != nil {
		return nil, fmt.Errorf("postgres: merge layers: %w", mergeErr)
	}
	return result, nil
}
```

Add the merge import at the top of the file:

```go
import (
	// ... existing imports ...
	"github.com/specgraph/specgraph/internal/constitution/merge"
)
```

- [ ] **Step 5: Run tests to verify they pass**

Run:

```bash
go test -tags integration ./internal/storage/postgres/ -run TestGetMergedConstitution -v
```

Expected: all three tests PASS.

- [ ] **Step 6: Run full storage test suite to check for regressions**

Run:

```bash
go test -tags integration ./internal/storage/...
```

Expected: all tests PASS. If any mock backends in `internal/storage/...` fail to compile because they need to implement the new method, address them now — but the mocks under `internal/server/` are handled in Task 5.

- [ ] **Step 7: Commit**

```bash
git add internal/storage/constitution.go internal/storage/postgres/constitution.go internal/storage/postgres/get_merged_constitution_test.go
git commit -s -m "feat(storage): add GetMergedConstitution method (spgr-8ar piece A)

Adds a convenience method on ConstitutionBackend that returns the
merged constitution plus per-field provenance. Implementation calls
GetAllLayers followed by merge.Layers, matching the pattern already
used by ConstitutionService.GetConstitution RPC handler.

Marks single-layer GetConstitution as Deprecated. Removal happens in
Piece D once PrimeData and export.engine migrate (Piece A subsequent
tasks).

Part of spgr-8ar Piece A."
```

---

## Task 2: Add `ProvenanceEntry` domain type + extend `PrimeData`

**Files:**

- Modify: `internal/storage/execution_domain.go`

- [ ] **Step 1: Add `ProvenanceEntry` and extend `PrimeData`**

Replace the `PrimeData` struct in `internal/storage/execution_domain.go` (currently lines 87-92) with:

```go
// ProvenanceEntry maps a constitution field path to the layer that set its value.
type ProvenanceEntry struct {
	Path  string
	Layer ConstitutionLayer
}

// PrimeData holds the raw data needed to compose a prime response.
type PrimeData struct {
	Spec         *Spec
	Decisions    []*Decision
	Constitution *Constitution
	// ConstitutionProvenance maps constitution field paths to the layer
	// that set each value. Empty iff Constitution is nil.
	ConstitutionProvenance []ProvenanceEntry
}
```

- [ ] **Step 2: Verify the package compiles**

Run:

```bash
go build ./internal/storage/...
```

Expected: clean build (no callers yet read the new field).

- [ ] **Step 3: Commit**

```bash
git add internal/storage/execution_domain.go
git commit -s -m "feat(storage): add ProvenanceEntry + PrimeData.ConstitutionProvenance

Threads constitution provenance through the prime path so polecats and
Gastown see which layer set each constitution value. Domain-only change;
proto changes are deferred to Piece E (single source of truth in the
view oneof, not duplicated on PrimeResponse).

Part of spgr-8ar Piece A."
```

---

## Task 3: Route `GetPrimeData` through merged constitution + thread provenance

**Files:**

- Modify: `internal/storage/postgres/execution.go`
- Modify: `internal/storage/postgres/execution_test.go`

- [ ] **Step 1: Write a failing test for multi-layer prime provenance**

Append to `internal/storage/postgres/execution_test.go`:

```go
func TestGetPrimeData_MultiLayerProvenance(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Org layer with one principle.
	_, err := store.UpdateConstitution(ctx, &storage.Constitution{
		Name:  "org",
		Layer: storage.ConstitutionLayerOrg,
		Principles: []storage.Principle{
			{ID: "p-org", Statement: "Org rule"},
		},
	})
	require.NoError(t, err)

	// Project layer with another principle.
	_, err = store.UpdateConstitution(ctx, &storage.Constitution{
		Name:  "project",
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p-proj", Statement: "Project rule"},
		},
	})
	require.NoError(t, err)

	// Seed a spec so GetPrimeData has something to find.
	_, err = store.CreateSpec(ctx, "prime-spec", "intent", "P2", "M",
		storage.SpecProvenanceAuthored, "", nil, nil, nil, nil)
	require.NoError(t, err)

	pd, err := store.GetPrimeData(ctx, "prime-spec")

	require.NoError(t, err)
	require.NotNil(t, pd.Constitution)
	assert.Len(t, pd.Constitution.Principles, 2,
		"PrimeData must carry merged constitution from both layers")

	// Build a provenance lookup for assertion clarity.
	provByPath := map[string]storage.ConstitutionLayer{}
	for _, e := range pd.ConstitutionProvenance {
		provByPath[e.Path] = e.Layer
	}
	assert.Equal(t, storage.ConstitutionLayerOrg, provByPath["principles[p-org]"])
	assert.Equal(t, storage.ConstitutionLayerProject, provByPath["principles[p-proj]"])
}

func TestGetPrimeData_NoConstitution_EmptyProvenance(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "no-con-spec", "intent", "P2", "M",
		storage.SpecProvenanceAuthored, "", nil, nil, nil, nil)
	require.NoError(t, err)

	pd, err := store.GetPrimeData(ctx, "no-con-spec")

	require.NoError(t, err)
	assert.Nil(t, pd.Constitution,
		"invariant: Constitution nil when no layers exist")
	assert.Empty(t, pd.ConstitutionProvenance,
		"invariant: provenance empty iff Constitution nil")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test -tags integration ./internal/storage/postgres/ -run TestGetPrimeData_MultiLayerProvenance -v
go test -tags integration ./internal/storage/postgres/ -run TestGetPrimeData_NoConstitution_EmptyProvenance -v
```

Expected: `TestGetPrimeData_MultiLayerProvenance` fails (only one principle returned because current code calls single-layer `GetConstitution`); the empty-provenance test fails because the field doesn't get populated.

- [ ] **Step 3: Update `GetPrimeData` to use merged constitution**

Replace the `GetPrimeData` function in `internal/storage/postgres/execution.go` (lines 235-260) with:

```go
// GetPrimeData returns the data needed to compose a prime response.
func (s *Store) GetPrimeData(ctx context.Context, slug string) (*storage.PrimeData, error) {
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("postgres: get prime data: %w", err)
	}

	decisions, err := s.fetchLinkedDecisions(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("postgres: get prime data decisions: %w", err)
	}

	pd := &storage.PrimeData{
		Spec:      spec,
		Decisions: decisions,
	}

	merged, err := s.GetMergedConstitution(ctx)
	switch {
	case err == nil:
		pd.Constitution = merged.Constitution
		pd.ConstitutionProvenance = make([]storage.ProvenanceEntry, 0, len(merged.Provenance))
		for path, layer := range merged.Provenance {
			pd.ConstitutionProvenance = append(pd.ConstitutionProvenance, storage.ProvenanceEntry{
				Path:  path,
				Layer: layer,
			})
		}
		// Sort for deterministic output across runs.
		sort.Slice(pd.ConstitutionProvenance, func(i, j int) bool {
			return pd.ConstitutionProvenance[i].Path < pd.ConstitutionProvenance[j].Path
		})
	case errors.Is(err, storage.ErrConstitutionNotFound):
		// No layers exist — leave Constitution nil and Provenance empty.
		// Matches existing no-constitution behavior.
	default:
		return nil, fmt.Errorf("postgres: get prime data constitution: %w", err)
	}

	return pd, nil
}
```

Add `"sort"` to the imports if not already present.

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
go test -tags integration ./internal/storage/postgres/ -run TestGetPrimeData -v
```

Expected: all `TestGetPrimeData*` tests PASS, including the two new ones.

- [ ] **Step 5: Run the full server test suite (compile check for callers)**

Run:

```bash
go build ./...
go test ./internal/server/... -count=1
```

Expected: build succeeds; server tests pass (the existing `*BackendsStub` types in `internal/server/test_scoper_test.go` implement `ConstitutionBackend` via embedded stubs which now need `GetMergedConstitution`; addressed in Task 5).

If the server tests fail to compile because of the new interface method, **proceed to Task 5 now**; otherwise commit and continue.

- [ ] **Step 6: Commit**

```bash
git add internal/storage/postgres/execution.go internal/storage/postgres/execution_test.go
git commit -s -m "feat(storage): GetPrimeData uses merged constitution with provenance

Closes spgr-8ar item (6) — PrimeData carried the highest-precedence
single layer with no provenance. After this change, PrimeData carries
the merged constitution and a sorted ProvenanceEntry list. Polecats
and downstream consumers see the effective constitution.

Invariants enforced (Section 14 of design):
- Constitution nil iff ConstitutionProvenance empty
- ProvenanceEntry list is sorted by path for deterministic output

Part of spgr-8ar Piece A."
```

---

## Task 4: Update mock backends to implement `GetMergedConstitution`

**Files:**

- Modify: `internal/server/test_scoper_test.go`
- Modify: `internal/server/constitution_handler_test.go`
- Modify: `internal/server/sync_handler_test.go`
- Modify: `internal/server/error_sanitize_test.go`

- [ ] **Step 1: Add stub to `stubBackend` in `test_scoper_test.go`**

Find the existing `stubBackend` methods at `internal/server/test_scoper_test.go:146-154`. Add this method right after `GetAllLayers`:

```go
func (stubBackend) GetMergedConstitution(_ context.Context) (*merge.Result, error) {
	return nil, storage.ErrConstitutionNotFound
}
```

Add the import if not present:

```go
"github.com/specgraph/specgraph/internal/constitution/merge"
```

- [ ] **Step 2: Add stub to `mockConstitutionBackend` in `constitution_handler_test.go`**

After the existing `GetAllLayers` method at `internal/server/constitution_handler_test.go:65`, add:

```go
func (m *mockConstitutionBackend) GetMergedConstitution(ctx context.Context) (*merge.Result, error) {
	layers, err := m.GetAllLayers(ctx)
	if err != nil {
		return nil, err
	}
	if len(layers) == 0 {
		return nil, storage.ErrConstitutionNotFound
	}
	return merge.Layers(layers)
}
```

Add the import if not present.

- [ ] **Step 3: Add stub to `syncTestBackend` in `sync_handler_test.go`**

After the existing `GetAllLayers` delegation at `internal/server/sync_handler_test.go:197-199`, add:

```go
func (s *syncTestBackend) GetMergedConstitution(ctx context.Context) (*merge.Result, error) {
	if s.con != nil {
		return s.con.GetMergedConstitution(ctx)
	}
	return nil, storage.ErrConstitutionNotFound
}
```

Add the import if not present.

- [ ] **Step 4: Add stub to `errorBackend` in `error_sanitize_test.go`**

After the existing `GetConstitution` method at `internal/server/error_sanitize_test.go:95`, add:

```go
func (errorBackend) GetMergedConstitution(_ context.Context) (*merge.Result, error) {
	return nil, errSentinel
}
```

If `errSentinel` is not the right name, match whatever sentinel error this file uses for "internal error" testing (check the existing `GetConstitution` return value — that's the right one).

Add the import if not present.

- [ ] **Step 5: Run the server test suite**

Run:

```bash
go test ./internal/server/... -count=1 -v
```

Expected: all tests PASS. The new mock methods are unused by existing tests, so behavior is unchanged.

- [ ] **Step 6: Commit**

```bash
git add internal/server/test_scoper_test.go internal/server/constitution_handler_test.go internal/server/sync_handler_test.go internal/server/error_sanitize_test.go
git commit -s -m "test(server): mock backends implement GetMergedConstitution

Adds GetMergedConstitution stubs to stubBackend, mockConstitutionBackend,
syncTestBackend, and errorBackend so the server test suite compiles
against the new ConstitutionBackend interface.

Part of spgr-8ar Piece A."
```

---

## Task 5: Bump export schema to v2 + add `Constitutions` field

**Files:**

- Modify: `internal/export/schema.go`

- [ ] **Step 1: Bump version and add the new field**

Replace `internal/export/schema.go` content with:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package export implements project export, import, and verification.
package export

import (
	"time"

	"github.com/specgraph/specgraph/internal/storage"
)

// CurrentSchemaVersion is bumped to 2 with the multi-layer constitution
// migration. v2 documents use the Constitutions list field; v1 documents
// (still importable) use the singular Constitution field.
const CurrentSchemaVersion = 2

// Document is the top-level export structure.
type Document struct {
	SchemaVersion    int        `json:"schema_version"`
	ExportedAt       time.Time  `json:"exported_at"`
	SpecGraphVersion string     `json:"specgraph_version"`
	ProjectSlug      string     `json:"project_slug"`
	Data             Data       `json:"data"`
	Signature        *Signature `json:"signature,omitempty"`
}

// Signature holds HMAC verification data.
type Signature struct {
	Algorithm string `json:"algorithm"`
	Digest    string `json:"digest"`
}

// Data contains all exported entities in dependency order.
type Data struct {
	Project *storage.Project `json:"project"`

	// Constitution is the v1 single-layer field. Always nil in v2-emitted
	// documents (omitempty drops it). Populated when importing v1 documents
	// for the legacy single-layer case.
	Constitution *storage.Constitution `json:"constitution,omitempty"`

	// Constitutions is the v2 list of constitution layers in precedence
	// order (user, org, project, domain). Populated by v2 exports; consumed
	// by v2 imports.
	Constitutions []*storage.Constitution `json:"constitutions,omitempty"`

	Specs           []*storage.Spec                 `json:"specs"`
	Decisions       []*storage.Decision             `json:"decisions"`
	Slices          []*storage.Slice                `json:"slices"`
	Edges           []Edge                          `json:"edges"`
	Findings        []*storage.AnalyticalFinding    `json:"findings"`
	ChangeLogs      []*storage.ChangeLogEntry       `json:"changelogs"`
	Conversations   []*storage.ConversationLogEntry `json:"conversations"`
	SyncMappings    []*storage.SyncMapping          `json:"sync_mappings"`
	ExecutionEvents []*storage.ExecutionEvent       `json:"execution_events"`
}

// Edge is the export representation of a graph edge.
type Edge struct {
	FromSlug          string `json:"from_slug"`
	ToSlug            string `json:"to_slug"`
	Type              string `json:"type"`
	ContentHashAtLink string `json:"content_hash_at_link,omitempty"`
}
```

- [ ] **Step 2: Run existing export tests — expect failures, file the symptoms**

Run:

```bash
go test ./internal/export/... -count=1
```

Expected: `TestImport_RejectsUnsupportedSchemaVersion` and other tests using `CurrentSchemaVersion` should still pass (no v999 / v2 conflict). Existing tests using `doc.Data.Constitution` will compile but may exercise different behavior — note any unexpected failures; they will be addressed in Task 7.

- [ ] **Step 3: Commit**

```bash
git add internal/export/schema.go
git commit -s -m "feat(export): bump schema to v2 with multi-layer constitutions

Adds Data.Constitutions []*Constitution alongside the legacy singular
Constitution field. v2 exports populate Constitutions; v1 imports
continue to use Constitution. Cross-field validation is added in the
import path next.

Part of spgr-8ar Piece A."
```

---

## Task 6: Update `Engine.collect` to use `GetAllLayers`

**Files:**

- Modify: `internal/export/engine.go`
- Modify: `internal/export/engine_test.go`

- [ ] **Step 1: Write a failing test for multi-layer export**

Append to `internal/export/engine_test.go`:

```go
func TestExport_MultiLayerConstitution(t *testing.T) {
	backend := newTestBackend(t)
	ctx := context.Background()

	// Seed two layers.
	_, err := backend.UpdateConstitution(ctx, &storage.Constitution{
		Name:  "org",
		Layer: storage.ConstitutionLayerOrg,
		Principles: []storage.Principle{{ID: "p-org", Statement: "Org"}},
	})
	require.NoError(t, err)

	_, err = backend.UpdateConstitution(ctx, &storage.Constitution{
		Name:  "project",
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{{ID: "p-proj", Statement: "Proj"}},
	})
	require.NoError(t, err)

	engine := NewEngine(backend, "", "test-version")
	out, err := engine.Export(ctx, "test-project")
	require.NoError(t, err)

	var doc Document
	require.NoError(t, json.Unmarshal(out, &doc))

	assert.Equal(t, 2, doc.SchemaVersion)
	assert.Nil(t, doc.Data.Constitution, "v2 exports never populate the v1 field")
	require.Len(t, doc.Data.Constitutions, 2, "v2 export must contain both layers")
	assert.Equal(t, storage.ConstitutionLayerOrg, doc.Data.Constitutions[0].Layer,
		"layers in precedence order: org before project")
	assert.Equal(t, storage.ConstitutionLayerProject, doc.Data.Constitutions[1].Layer)
}
```

(If `newTestBackend` is not the right helper name, use the existing test backend constructor in the file.)

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test ./internal/export/ -run TestExport_MultiLayerConstitution -v
```

Expected: fail — current `collect` calls `GetConstitution` (single layer) and writes to `doc.Data.Constitution`; the test asserts both `Constitutions` populated and `Constitution` nil, which the current code can't do.

- [ ] **Step 3: Update `Engine.collect`**

In `internal/export/engine.go`, find the constitution block at lines 93-97:

```go
// Constitution (optional — may not exist)
constitution, err := e.backend.GetConstitution(ctx)
if err == nil {
	doc.Data.Constitution = constitution
}
```

Replace with:

```go
// Constitutions (optional — may not exist; v2 emits the list field).
layers, err := e.backend.GetAllLayers(ctx)
if err != nil {
	return nil, fmt.Errorf("get constitution layers: %w", err)
}
if len(layers) > 0 {
	doc.Data.Constitutions = layers
}
```

`GetAllLayers` returns an empty slice (not error) when no layers exist, so the `len > 0` guard prevents emitting an empty list.

- [ ] **Step 4: Run the test to verify it passes**

Run:

```bash
go test ./internal/export/ -run TestExport_MultiLayerConstitution -v
```

Expected: PASS.

- [ ] **Step 5: Run all export tests to verify no regressions**

Run:

```bash
go test ./internal/export/... -count=1 -v
```

Expected: all tests PASS. If any pre-existing test asserts on `doc.Data.Constitution` from an export (single-layer case), it should be updated to assert on `doc.Data.Constitutions[0]` instead. Update such tests as part of this commit.

- [ ] **Step 6: Commit**

```bash
git add internal/export/engine.go internal/export/engine_test.go
git commit -s -m "feat(export): collect uses GetAllLayers for v2 schema

Engine.collect now reads all constitution layers via GetAllLayers and
emits them as the v2 Constitutions list field. Previously it called
GetConstitution which returns only the highest-precedence layer,
silently flattening multi-layer projects on export. Bug fix.

Part of spgr-8ar Piece A."
```

---

## Task 7: Schema-version-aware import with cross-field validation

**Files:**

- Modify: `internal/export/engine.go`
- Modify: `internal/export/engine_test.go`

- [ ] **Step 1: Write failing tests for v1/v2 import paths and cross-field validation**

Append to `internal/export/engine_test.go`:

```go
func TestImport_V1Document_SingleLayer(t *testing.T) {
	backend := newTestBackend(t)
	ctx := context.Background()

	// Hand-crafted v1 document with the legacy singular field.
	v1Doc := Document{
		SchemaVersion:    1,
		ProjectSlug:      "test-project",
		SpecGraphVersion: "test-version",
		Data: Data{
			Project: &storage.Project{Slug: "test-project"},
			Constitution: &storage.Constitution{
				Name:  "v1-only",
				Layer: storage.ConstitutionLayerProject,
				Principles: []storage.Principle{{ID: "p1", Statement: "P1"}},
			},
		},
	}
	data, err := json.Marshal(v1Doc)
	require.NoError(t, err)

	engine := NewEngine(backend, "", "test-version")
	result, err := engine.Import(ctx, data, false, false)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Constitution, "exactly one layer imported")

	layers, err := backend.GetAllLayers(ctx)
	require.NoError(t, err)
	require.Len(t, layers, 1)
	assert.Equal(t, "v1-only", layers[0].Name)
	assert.Equal(t, storage.ConstitutionLayerProject, layers[0].Layer)
}

func TestImport_V2Document_MultipleLayers(t *testing.T) {
	backend := newTestBackend(t)
	ctx := context.Background()

	v2Doc := Document{
		SchemaVersion:    2,
		ProjectSlug:      "test-project",
		SpecGraphVersion: "test-version",
		Data: Data{
			Project: &storage.Project{Slug: "test-project"},
			Constitutions: []*storage.Constitution{
				{Name: "org", Layer: storage.ConstitutionLayerOrg, Principles: []storage.Principle{{ID: "po"}}},
				{Name: "proj", Layer: storage.ConstitutionLayerProject, Principles: []storage.Principle{{ID: "pp"}}},
			},
		},
	}
	data, err := json.Marshal(v2Doc)
	require.NoError(t, err)

	engine := NewEngine(backend, "", "test-version")
	result, err := engine.Import(ctx, data, false, false)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Constitution, "both layers imported")

	layers, err := backend.GetAllLayers(ctx)
	require.NoError(t, err)
	assert.Len(t, layers, 2)
}

func TestImport_V1Document_WithV2Field_Rejected(t *testing.T) {
	backend := newTestBackend(t)

	mismatched := Document{
		SchemaVersion: 1,
		ProjectSlug:   "test-project",
		Data: Data{
			Project: &storage.Project{Slug: "test-project"},
			Constitutions: []*storage.Constitution{
				{Layer: storage.ConstitutionLayerProject},
			},
		},
	}
	data, err := json.Marshal(mismatched)
	require.NoError(t, err)

	engine := NewEngine(backend, "", "test-version")
	_, err = engine.Import(context.Background(), data, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v1 documents must use 'constitution' field, not 'constitutions'")
}

func TestImport_V2Document_WithV1Field_Rejected(t *testing.T) {
	backend := newTestBackend(t)

	mismatched := Document{
		SchemaVersion: 2,
		ProjectSlug:   "test-project",
		Data: Data{
			Project: &storage.Project{Slug: "test-project"},
			Constitution: &storage.Constitution{
				Layer: storage.ConstitutionLayerProject,
			},
		},
	}
	data, err := json.Marshal(mismatched)
	require.NoError(t, err)

	engine := NewEngine(backend, "", "test-version")
	_, err = engine.Import(context.Background(), data, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "v2 documents must use 'constitutions' field, not 'constitution'")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:

```bash
go test ./internal/export/ -run "TestImport_V1Document|TestImport_V2Document" -v
```

Expected: all four fail. The current `writeEntities` calls `UpdateConstitution` with `doc.Data.Constitution` regardless of schema version, so the v2 path doesn't import layers, the v1 path works incidentally, and the cross-field validation is absent.

- [ ] **Step 3: Update `Engine.writeEntities` constitution block**

In `internal/export/engine.go`, find the constitution block in `writeEntities` (currently lines 379-385):

```go
// 2. Constitution
if doc.Data.Constitution != nil {
	if _, err := e.backend.UpdateConstitution(ctx, doc.Data.Constitution); err != nil {
		return nil, fmt.Errorf("update constitution: %w", err)
	}
	res.Constitution = 1
}
```

Replace with:

```go
// 2. Constitutions — handle schema v1 (single) and v2 (list) with strict
// cross-field validation to prevent silent data loss from mismatched docs.
var layers []*storage.Constitution
switch doc.SchemaVersion {
case 1:
	if doc.Data.Constitutions != nil {
		return nil, fmt.Errorf("v1 documents must use 'constitution' field, not 'constitutions'")
	}
	if doc.Data.Constitution != nil {
		layers = []*storage.Constitution{doc.Data.Constitution}
	}
case 2:
	if doc.Data.Constitution != nil {
		return nil, fmt.Errorf("v2 documents must use 'constitutions' field, not 'constitution'")
	}
	layers = doc.Data.Constitutions
}
for _, layer := range layers {
	if _, err := e.backend.UpdateConstitution(ctx, layer); err != nil {
		return nil, fmt.Errorf("update constitution layer %s: %w", layer.Layer, err)
	}
	res.Constitution++
}
```

- [ ] **Step 4: Run the new tests**

Run:

```bash
go test ./internal/export/ -run "TestImport_V1Document|TestImport_V2Document" -v
```

Expected: all four PASS.

- [ ] **Step 5: Run the full export test suite**

Run:

```bash
go test ./internal/export/... -count=1 -v
```

Expected: all tests PASS, including pre-existing import tests that use `CurrentSchemaVersion` (now 2 with `Constitutions` field). If any pre-existing test populated `doc.Data.Constitution` on a `SchemaVersion: CurrentSchemaVersion` document, it now hits the v2 cross-field error and must be updated to use `Constitutions: []*storage.Constitution{...}` instead. Update such tests as part of this commit.

- [ ] **Step 6: Commit**

```bash
git add internal/export/engine.go internal/export/engine_test.go
git commit -s -m "feat(export): schema-aware import with cross-field validation

Engine.writeEntities now handles both schema v1 (singular Constitution
field) and v2 (Constitutions list). Cross-field validation rejects
documents with schema_version + field mismatch (e.g., v1 doc with
'constitutions' populated) to prevent silent data loss.

Closes export round-trip bug surfaced in adversarial review: v1 docs
imported under v2 code preserve their single layer; v2 docs import all
layers; mismatched docs error explicitly.

Part of spgr-8ar Piece A."
```

---

## Task 8: v1↔v2 round-trip integration test

**Files:**

- Modify: `internal/export/engine_test.go`

- [ ] **Step 1: Write the round-trip test**

Append to `internal/export/engine_test.go`:

```go
func TestExportImport_V1ToV2_RoundTrip(t *testing.T) {
	// Hand-craft a v1 document, import it via the new code, re-export,
	// verify the resulting v2 document preserves the single layer.

	v1Source := Document{
		SchemaVersion:    1,
		ProjectSlug:      "rt-project",
		SpecGraphVersion: "test-version",
		Data: Data{
			Project: &storage.Project{Slug: "rt-project"},
			Constitution: &storage.Constitution{
				Name:  "legacy",
				Layer: storage.ConstitutionLayerProject,
				Principles: []storage.Principle{
					{ID: "legacy-p1", Statement: "Legacy principle"},
				},
				Constraints: []string{"legacy-constraint"},
			},
		},
	}
	v1Bytes, err := json.Marshal(v1Source)
	require.NoError(t, err)

	backend := newTestBackend(t)
	ctx := context.Background()
	engine := NewEngine(backend, "", "test-version")

	// Import v1.
	_, err = engine.Import(ctx, v1Bytes, false, false)
	require.NoError(t, err)

	// Re-export as v2.
	v2Bytes, err := engine.Export(ctx, "rt-project")
	require.NoError(t, err)

	var v2 Document
	require.NoError(t, json.Unmarshal(v2Bytes, &v2))

	assert.Equal(t, 2, v2.SchemaVersion, "re-export uses CurrentSchemaVersion=2")
	assert.Nil(t, v2.Data.Constitution, "v2 export never populates v1 field")
	require.Len(t, v2.Data.Constitutions, 1, "exactly one layer preserved")

	got := v2.Data.Constitutions[0]
	assert.Equal(t, "legacy", got.Name)
	assert.Equal(t, storage.ConstitutionLayerProject, got.Layer)
	require.Len(t, got.Principles, 1)
	assert.Equal(t, "legacy-p1", got.Principles[0].ID)
	require.Len(t, got.Constraints, 1)
	assert.Equal(t, "legacy-constraint", got.Constraints[0])
}
```

- [ ] **Step 2: Run the test**

Run:

```bash
go test ./internal/export/ -run TestExportImport_V1ToV2_RoundTrip -v
```

Expected: PASS. Implementation from Tasks 5-7 already supports this; the test is the explicit invariant verification.

- [ ] **Step 3: Commit**

```bash
git add internal/export/engine_test.go
git commit -s -m "test(export): v1 to v2 round-trip migration

Asserts the invariant: a v1 export document imports under v2 code and
re-exports as a v2 document with the single layer preserved (under the
Constitutions list field). Covers the 'forward-compatible for the data
v1 could hold' contract from the design.

Part of spgr-8ar Piece A."
```

---

## Task 9: Quality gate run

**Files:** none

- [ ] **Step 1: Run `task check`**

Run:

```bash
task check
```

Expected: all checks PASS — fmt, license, lint, build, unit tests.

If `revive` complains about a missing package comment, fix it inline. If `wrapcheck` complains, wrap the offending error. If `govet -shadow` complains, rename the shadowed variable. These are usually 1-line fixes.

- [ ] **Step 2: Run `task pr-prep`**

Run:

```bash
task pr-prep
```

Expected: all checks PASS — `task check` + `test:integration` + `test:e2e`. Docker must be available (per project rule: always is).

If integration or e2e tests fail in areas unrelated to Piece A (flaky tests, environmental issues), document the symptom and investigate before pushing.

- [ ] **Step 3: Update bd issue with progress note**

Run:

```bash
bd update spgr-8ar --notes "Piece A complete: GetMergedConstitution + PrimeData provenance + export schema v2 with v1->v2 migration. Pieces B/C/D/E remaining."
```

- [ ] **Step 4: Sync beads**

Run:

```bash
bd dolt push
```

Expected: "Push complete."

---

## Task 10: Push and open PR

**Files:** none

- [ ] **Step 1: Verify the commit graph**

Run:

```bash
jj --no-pager log -r 'main..@-' --limit 20
```

Expected: 8 commits (one per Tasks 1-8), each with a `Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>` trailer, conventional commit type prefixes, and `spgr-8ar piece A` mentioned in the message.

- [ ] **Step 2: Verify auth account for push**

Run:

```bash
gh auth switch -u seanb4t -h github.com
gh auth status
```

Expected: `Active account: true` on the `seanb4t` row (per the `gh-auth-gotcha-specgraph-seanb4t-is-the-personal` memory).

- [ ] **Step 3: Set the bookmark for the PR branch**

Run:

```bash
jj --no-pager bookmark set spgr-8ar-piece-a -r @-
```

Expected: bookmark created/moved to point at the top commit of the Piece A stack.

- [ ] **Step 4: Push the bookmark**

Run:

```bash
jj --no-pager git push --bookmark spgr-8ar-piece-a
```

Expected: branch pushed successfully to `origin`.

- [ ] **Step 5: Open the PR**

Run:

```bash
gh pr create --title "spgr-8ar PR A: storage gap close + export round-trip fix" --body "$(cat <<'EOF'
## Summary

- Adds `GetMergedConstitution` to `ConstitutionBackend`; routes `PrimeData` and (future) `export.engine` through it
- Threads `ConstitutionProvenance` through `PrimeData` (domain only — no proto change in this PR; deferred to Piece E)
- Bumps export `CurrentSchemaVersion` from 1 to 2; replaces single `Constitution` field with `Constitutions []` list; schema-version-aware import with cross-field validation
- Marks single-layer `Store.GetConstitution` as `// Deprecated` (removal in Piece D)

Implements Piece A of [spgr-8ar design](docs/superpowers/specs/2026-05-21-multi-layer-constitution-completion-design.md). Closes the genuine bead acceptance criterion (item 6: PrimeData uses merged constitution) and fixes the unstated export-flattens-layers bug surfaced in adversarial review.

## Test plan

- [ ] `task check` passes (fmt, lint, build, unit)
- [ ] `task pr-prep` passes (check + integration + e2e)
- [ ] `TestGetMergedConstitution_{Empty,SingleLayer,MultiLayer}` exercise the new method
- [ ] `TestGetPrimeData_MultiLayerProvenance` confirms prime path carries merged constitution + sorted provenance
- [ ] `TestGetPrimeData_NoConstitution_EmptyProvenance` confirms invariant (Constitution nil iff Provenance empty)
- [ ] `TestExport_MultiLayerConstitution` confirms v2 exports populate `Constitutions` list in precedence order
- [ ] `TestImport_V1Document_SingleLayer` confirms v1 documents still import correctly
- [ ] `TestImport_V2Document_MultipleLayers` confirms v2 import populates all layers
- [ ] `TestImport_{V1,V2}Document_With{V2,V1}Field_Rejected` confirms cross-field validation
- [ ] `TestExportImport_V1ToV2_RoundTrip` confirms forward migration is lossless for v1-expressible data

Pieces B/C/D/E remain — separate PRs per the design's piece breakdown.

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

Expected: PR URL printed to stdout.

- [ ] **Step 6: Drop the local working-copy commit**

Back in the workspace, the working copy `@` is empty after the push. Leave it for the next session, or:

```bash
jj --no-pager abandon @
```

to clean up. The pushed commits remain on the bookmark.

---

## Self-review

### Spec coverage check

- **Section 1 (Storage interface)**: Task 1 adds `GetMergedConstitution`; Task 4 propagates to mocks ✓
- **Section 2 (PrimeData provenance)**: Task 2 adds domain types; Task 3 routes through it ✓
- **Section 3 (Export round-trip fix)**: Tasks 5-8 cover schema v2, GetAllLayers in collect, cross-field validation, v1→v2 round-trip ✓
- **Section 14 invariant: Merge consistency** — both PrimeData and (this PR's) export read paths converge on merged; the storage interface deprecation comment plus the eventual Piece D CI guard structurally enforces it ✓
- **Section 14 invariant: Provenance ↔ Constitution coupling** — Task 3 Step 1 explicitly tests both branches ✓
- **Section 14 invariant: v1 ↔ v2 export** — Tasks 7-8 cover both directions, plus cross-field rejection ✓

### Placeholder scan

- No `TBD`, `TODO`, "fill in details", or "similar to Task N" references.
- Every step that changes code shows the actual code.
- Every test step has runnable assertions.

### Type consistency

- `ProvenanceEntry` defined once in Task 2; referenced consistently in Tasks 3, 4 ✓
- `GetMergedConstitution` signature `func(ctx) (*merge.Result, error)` consistent across interface, impl, mocks ✓
- `merge.Result` from `internal/constitution/merge` — existing type, no redefinition needed ✓

### Out-of-scope deferrals

Pieces B (remote fetch), C (provenance display), D (delete `GetConstitution`), E (prime unification) are explicitly out of scope for this plan. Each gets its own plan document.

Note: the `// Deprecated` comment on `Store.GetConstitution` lands in Task 1 of this plan; the actual deletion is in Piece D's plan after this merges and we verify no remaining callers.

---

## Plan complete

**Plan complete and saved to `docs/superpowers/plans/2026-05-21-spgr-8ar-piece-a-implementation-plan.md`.** Two execution options:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks, fast iteration
2. **Inline Execution** — execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
