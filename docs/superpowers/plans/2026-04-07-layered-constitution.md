# Layered Constitution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement multi-layer constitution support with strategic merge, `$delete` directives, provenance tracking, and full test coverage.

**Architecture:** New DB migration changes uniqueness from `(project_slug)` to `(project_slug, layer)` allowing up to 4 rows per project. New `internal/constitution/merge` package implements strategic merge with `$delete` directives and provenance tracking. `GetConstitution` with no layer filter runs the merge engine; with a layer filter returns raw single-layer data. `UpdateConstitution` upserts by `(project_slug, layer)`.

**Tech Stack:** Go, PostgreSQL (goose migrations, pgx v5), ConnectRPC/protobuf, SvelteKit (Svelte 5), Ginkgo/Gomega for E2E, testify for unit/integration.

**Design spec:** `docs/superpowers/specs/2026-04-07-layered-constitution-design.md`

---

## File Map

### New Files

| File | Purpose |
|------|---------|
| `internal/constitution/merge/merge.go` | Strategic merge engine with `$delete` support |
| `internal/constitution/merge/merge_test.go` | Unit tests for merge engine |
| `internal/constitution/merge/provenance.go` | Provenance tracking during merge |
| `internal/storage/postgres/migrations/004_constitution_layers.sql` | DB migration: uniqueness change + new columns |

### Modified Files

| File | Changes |
|------|---------|
| `proto/specgraph/v1/constitution.proto` | Add `source_url`, `source_hash` to Constitution; add `layer` to GetConstitutionRequest; add `ProvenanceEntry` and `provenance` to GetConstitutionResponse |
| `internal/storage/constitution_domain.go` | Add `Delete` field to Principle/Antipattern/Reference; add `SourceURL`/`SourceHash` to Constitution |
| `internal/storage/constitution.go` | Expand backend interface: `GetConstitutionLayer`, `GetAllLayers`, multi-layer GetConstitution |
| `internal/storage/postgres/constitution.go` | Implement multi-layer storage: per-layer upsert, all-layer fetch, single-layer fetch |
| `internal/server/constitution_handler.go` | Wire layer filter on GetConstitution, call merge engine, return provenance |
| `internal/server/convert_constitution.go` | Add provenance conversion, `$delete` handling in domain↔proto |
| `cmd/specgraph/constitution.go` | Add `--layer` flag to `import` (default: project) and `show` |
| `web/src/routes/constitution/+page.svelte` | Add provenance badges to fields/items |
| `internal/storage/postgres/constitution_test.go` | Multi-layer integration tests |
| `e2e/api/constitution_test.go` | Multi-layer E2E tests |
| `site/docs/concepts/constitution.md` | Remove "Planned" admonition, document multi-layer behavior |
| `site/docs/guides/cli-cookbook.md` | Add multi-layer constitution recipe |

---

## Phase 1: Merge Engine (Pure, No Dependencies)

### Task 1: Domain Type Changes — `$delete` Support

**Files:**
- Modify: `internal/storage/constitution_domain.go`

- [ ] **Step 1: Add Delete field to keyed types and source fields to Constitution**

In `internal/storage/constitution_domain.go`, add `Delete bool` to `Principle`, `Antipattern`, and `Reference`. Add `SourceURL` and `SourceHash` to `Constitution`:

```go
type Constitution struct {
	ID           string            `json:"id,omitempty"`
	Layer        ConstitutionLayer `json:"layer,omitempty"`
	Name         string            `json:"name,omitempty"`
	Version      int32             `json:"version,omitempty"`
	SourceURL    string            `json:"source_url,omitempty"`
	SourceHash   string            `json:"source_hash,omitempty"`
	Tech         *TechStack        `json:"tech,omitempty"`
	Principles   []Principle       `json:"principles,omitempty"`
	Process      *ProcessConfig    `json:"process,omitempty"`
	Constraints  []string          `json:"constraints,omitempty"`
	Antipatterns []Antipattern     `json:"antipatterns,omitempty"`
	References   []Reference       `json:"references,omitempty"`
	CreatedAt    time.Time         `json:"created_at,omitempty"`
	UpdatedAt    time.Time         `json:"updated_at,omitempty"`
}

type Principle struct {
	ID         string `json:"id,omitempty"`
	Statement  string `json:"statement,omitempty"`
	Rationale  string `json:"rationale,omitempty"`
	Exceptions string `json:"exceptions,omitempty"`
	Delete     bool   `json:"$delete,omitempty"`
}

type Antipattern struct {
	Pattern string `json:"pattern,omitempty"`
	Why     string `json:"why,omitempty"`
	Instead string `json:"instead,omitempty"`
	Delete  bool   `json:"$delete,omitempty"`
}

type Reference struct {
	Type   string `json:"type,omitempty"`
	Path   string `json:"path,omitempty"`
	Delete bool   `json:"$delete,omitempty"`
}
```

Note: The `$delete` JSON tag uses the `$delete` name so YAML/JSON files authored by users map directly to the Go struct via `encoding/json`.

- [ ] **Step 2: Verify build**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/storage/constitution_domain.go
git commit -m "feat(storage): add Delete field and source tracking to constitution domain types"
```

---

### Task 2: Merge Engine — Core Merge Logic

**Files:**
- Create: `internal/constitution/merge/merge.go`
- Create: `internal/constitution/merge/merge_test.go`

- [ ] **Step 1: Write failing tests for basic merge scenarios**

Create `internal/constitution/merge/merge_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package merge

import (
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMerge_ScalarOverride(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Process: &storage.ProcessConfig{
			SpecReview: "required for all specs",
			Deployment: &storage.DeploymentConfig{
				Strategy: "blue-green",
				Rollback: "automatic",
			},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Process: &storage.ProcessConfig{
			Deployment: &storage.DeploymentConfig{
				Strategy: "canary",
			},
		},
	}

	result, err := Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	// Project overrides org's deployment strategy
	assert.Equal(t, "canary", result.Constitution.Process.Deployment.Strategy)
	// Org's rollback preserved (project didn't set it)
	assert.Equal(t, "automatic", result.Constitution.Process.Deployment.Rollback)
	// Org's spec_review preserved (project didn't set process.spec_review)
	assert.Equal(t, "required for all specs", result.Constitution.Process.SpecReview)
}

func TestMerge_StringListUnion(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Tech: &storage.TechStack{
			Languages: &storage.Languages{
				Allowed: []string{"go", "python"},
			},
		},
		Constraints: []string{"no vendor lock-in"},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Tech: &storage.TechStack{
			Languages: &storage.Languages{
				Allowed: []string{"go", "typescript"},
			},
		},
		Constraints: []string{"all APIs must be idempotent"},
	}

	result, err := Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"go", "python", "typescript"}, result.Constitution.Tech.Languages.Allowed)
	assert.ElementsMatch(t, []string{"no vendor lock-in", "all APIs must be idempotent"}, result.Constitution.Constraints)
}

func TestMerge_KeyedObjectMerge(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "org principle 1", Rationale: "org reason"},
			{ID: "p2", Statement: "org principle 2"},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p2", Statement: "project overrides p2", Rationale: "project reason"},
			{ID: "p3", Statement: "project adds p3"},
		},
	}

	result, err := Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	require.Len(t, result.Constitution.Principles, 3)
	// p1 unchanged from org
	assert.Equal(t, "org principle 1", findPrinciple(result.Constitution.Principles, "p1").Statement)
	// p2 overridden by project
	assert.Equal(t, "project overrides p2", findPrinciple(result.Constitution.Principles, "p2").Statement)
	assert.Equal(t, "project reason", findPrinciple(result.Constitution.Principles, "p2").Rationale)
	// p3 added by project
	assert.Equal(t, "project adds p3", findPrinciple(result.Constitution.Principles, "p3").Statement)
}

func TestMerge_DeleteDirective(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "keep this"},
			{ID: "p2", Statement: "remove this"},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p2", Delete: true},
		},
	}

	result, err := Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	require.Len(t, result.Constitution.Principles, 1)
	assert.Equal(t, "p1", result.Constitution.Principles[0].ID)
}

func TestMerge_DeleteNonexistent_NoOp(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "keep this"},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p99", Delete: true},
		},
	}

	result, err := Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	require.Len(t, result.Constitution.Principles, 1)
	assert.Equal(t, "p1", result.Constitution.Principles[0].ID)
}

func TestMerge_SingleLayer(t *testing.T) {
	single := &storage.Constitution{
		Layer:       storage.ConstitutionLayerProject,
		Constraints: []string{"constraint 1"},
	}

	result, err := Layers([]*storage.Constitution{single})
	require.NoError(t, err)

	assert.Equal(t, []string{"constraint 1"}, result.Constitution.Constraints)
}

func TestMerge_EmptyInput(t *testing.T) {
	result, err := Layers(nil)
	require.NoError(t, err)

	assert.NotNil(t, result.Constitution)
	assert.Empty(t, result.Constitution.Principles)
	assert.Empty(t, result.Provenance)
}

func TestMerge_FourLayers_DomainWins(t *testing.T) {
	user := &storage.Constitution{
		Layer: storage.ConstitutionLayerUser,
		Process: &storage.ProcessConfig{
			SpecReview: "user says optional",
		},
	}
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Process: &storage.ProcessConfig{
			SpecReview: "org says required",
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Process: &storage.ProcessConfig{
			SpecReview: "project says lightweight",
		},
	}
	domain := &storage.Constitution{
		Layer: storage.ConstitutionLayerDomain,
		Process: &storage.ProcessConfig{
			SpecReview: "domain says strict",
		},
	}

	result, err := Layers([]*storage.Constitution{user, org, project, domain})
	require.NoError(t, err)

	assert.Equal(t, "domain says strict", result.Constitution.Process.SpecReview)
}

func TestMerge_DeleteOverrideThenDelete(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "org p1"},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Principles: []storage.Principle{
			{ID: "p1", Statement: "project overrides p1"},
		},
	}
	domain := &storage.Constitution{
		Layer: storage.ConstitutionLayerDomain,
		Principles: []storage.Principle{
			{ID: "p1", Delete: true},
		},
	}

	result, err := Layers([]*storage.Constitution{org, project, domain})
	require.NoError(t, err)

	assert.Empty(t, result.Constitution.Principles)
}

func TestMerge_DeleteOnlyItem(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Antipatterns: []storage.Antipattern{
			{Pattern: "god object", Why: "too complex"},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Antipatterns: []storage.Antipattern{
			{Pattern: "god object", Delete: true},
		},
	}

	result, err := Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	assert.Empty(t, result.Constitution.Antipatterns)
	assert.NotNil(t, result.Constitution.Antipatterns) // empty, not nil
}

func TestMerge_MapMerge(t *testing.T) {
	org := &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Tech: &storage.TechStack{
			Frameworks: map[string]string{
				"rpc":  "connectrpc",
				"http": "stdlib",
			},
		},
	}
	project := &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Tech: &storage.TechStack{
			Frameworks: map[string]string{
				"http":    "chi",
				"testing": "testify",
			},
		},
	}

	result, err := Layers([]*storage.Constitution{org, project})
	require.NoError(t, err)

	assert.Equal(t, "connectrpc", result.Constitution.Tech.Frameworks["rpc"])
	assert.Equal(t, "chi", result.Constitution.Tech.Frameworks["http"])       // project overrides
	assert.Equal(t, "testify", result.Constitution.Tech.Frameworks["testing"]) // project adds
}

func findPrinciple(ps []storage.Principle, id string) *storage.Principle {
	for i := range ps {
		if ps[i].ID == id {
			return &ps[i]
		}
	}
	return nil
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/constitution/merge/ -v
```

Expected: compilation error (package doesn't exist).

- [ ] **Step 3: Implement the merge engine**

Create `internal/constitution/merge/merge.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package merge implements strategic merge for layered constitutions.
// Layers are merged in precedence order (user < org < project < domain).
// Higher layers override scalars, union string lists, merge keyed objects
// by their natural key, and support $delete directives to remove items.
package merge

import (
	"github.com/specgraph/specgraph/internal/storage"
)

// Result holds the merged constitution and provenance metadata.
type Result struct {
	Constitution *storage.Constitution
	Provenance   map[string]storage.ConstitutionLayer
}

// Layers merges multiple constitution layers in order (lowest precedence first).
// Returns the merged result with provenance tracking which layer each value came from.
func Layers(layers []*storage.Constitution) (*Result, error) {
	result := &Result{
		Constitution: &storage.Constitution{},
		Provenance:   make(map[string]storage.ConstitutionLayer),
	}

	if len(layers) == 0 {
		return result, nil
	}

	for _, layer := range layers {
		mergeTech(result, layer)
		mergePrinciples(result, layer)
		mergeAntipatterns(result, layer)
		mergeReferences(result, layer)
		mergeConstraints(result, layer)
		mergeProcess(result, layer)
	}

	// Strip $delete markers from the final result.
	stripDeletes(result.Constitution)

	return result, nil
}

func mergeTech(r *Result, layer *storage.Constitution) {
	if layer.Tech == nil {
		return
	}
	if r.Constitution.Tech == nil {
		r.Constitution.Tech = &storage.TechStack{}
	}
	t := r.Constitution.Tech
	lt := layer.Tech

	if lt.Languages != nil {
		if t.Languages == nil {
			t.Languages = &storage.Languages{}
		}
		if lt.Languages.Primary != "" {
			t.Languages.Primary = lt.Languages.Primary
			r.Provenance["tech_config.languages.primary"] = layer.Layer
		}
		t.Languages.Allowed = unionStrings(t.Languages.Allowed, lt.Languages.Allowed)
		for _, s := range lt.Languages.Allowed {
			r.Provenance["tech_config.languages.allowed["+s+"]"] = layer.Layer
		}
		t.Languages.Forbidden = unionStrings(t.Languages.Forbidden, lt.Languages.Forbidden)
		for _, s := range lt.Languages.Forbidden {
			r.Provenance["tech_config.languages.forbidden["+s+"]"] = layer.Layer
		}
		if lt.Languages.ForbiddenReasons != nil {
			if t.Languages.ForbiddenReasons == nil {
				t.Languages.ForbiddenReasons = make(map[string]string)
			}
			for k, v := range lt.Languages.ForbiddenReasons {
				t.Languages.ForbiddenReasons[k] = v
			}
		}
	}

	t.Frameworks = mergeMaps(t.Frameworks, lt.Frameworks, "tech_config.frameworks", layer.Layer, r.Provenance)
	t.Infrastructure = mergeMaps(t.Infrastructure, lt.Infrastructure, "tech_config.infrastructure", layer.Layer, r.Provenance)
	t.APIStandards = mergeMaps(t.APIStandards, lt.APIStandards, "tech_config.api_standards", layer.Layer, r.Provenance)
	t.Data = mergeMaps(t.Data, lt.Data, "tech_config.data", layer.Layer, r.Provenance)
}

func mergePrinciples(r *Result, layer *storage.Constitution) {
	for _, p := range layer.Principles {
		if p.ID == "" {
			continue
		}
		idx := findByKey(r.Constitution.Principles, func(e storage.Principle) string { return e.ID })
		if i, ok := idx[p.ID]; ok {
			r.Constitution.Principles[i] = p
		} else {
			r.Constitution.Principles = append(r.Constitution.Principles, p)
		}
		r.Provenance["principles["+p.ID+"]"] = layer.Layer
	}
}

func mergeAntipatterns(r *Result, layer *storage.Constitution) {
	for _, a := range layer.Antipatterns {
		if a.Pattern == "" {
			continue
		}
		idx := findByKey(r.Constitution.Antipatterns, func(e storage.Antipattern) string { return e.Pattern })
		if i, ok := idx[a.Pattern]; ok {
			r.Constitution.Antipatterns[i] = a
		} else {
			r.Constitution.Antipatterns = append(r.Constitution.Antipatterns, a)
		}
		r.Provenance["antipatterns["+a.Pattern+"]"] = layer.Layer
	}
}

func mergeReferences(r *Result, layer *storage.Constitution) {
	for _, ref := range layer.References {
		if ref.Path == "" {
			continue
		}
		idx := findByKey(r.Constitution.References, func(e storage.Reference) string { return e.Path })
		if i, ok := idx[ref.Path]; ok {
			r.Constitution.References[i] = ref
		} else {
			r.Constitution.References = append(r.Constitution.References, ref)
		}
		r.Provenance["references["+ref.Path+"]"] = layer.Layer
	}
}

func mergeConstraints(r *Result, layer *storage.Constitution) {
	for _, c := range layer.Constraints {
		r.Provenance["constraints["+c+"]"] = layer.Layer
	}
	r.Constitution.Constraints = unionStrings(r.Constitution.Constraints, layer.Constraints)
}

func mergeProcess(r *Result, layer *storage.Constitution) {
	if layer.Process == nil {
		return
	}
	if r.Constitution.Process == nil {
		r.Constitution.Process = &storage.ProcessConfig{}
	}
	p := r.Constitution.Process
	lp := layer.Process

	if lp.SpecReview != "" {
		p.SpecReview = lp.SpecReview
		r.Provenance["process.spec_review"] = layer.Layer
	}
	if lp.SecurityReview != nil {
		if p.SecurityReview == nil {
			p.SecurityReview = &storage.SecurityReviewConfig{}
		}
		if lp.SecurityReview.When != "" {
			p.SecurityReview.When = lp.SecurityReview.When
			r.Provenance["process.security_review.when"] = layer.Layer
		}
	}
	if lp.Deployment != nil {
		if p.Deployment == nil {
			p.Deployment = &storage.DeploymentConfig{}
		}
		if lp.Deployment.Strategy != "" {
			p.Deployment.Strategy = lp.Deployment.Strategy
			r.Provenance["process.deployment.strategy"] = layer.Layer
		}
		if lp.Deployment.Rollback != "" {
			p.Deployment.Rollback = lp.Deployment.Rollback
			r.Provenance["process.deployment.rollback"] = layer.Layer
		}
	}
	if lp.Documentation != nil {
		if p.Documentation == nil {
			p.Documentation = &storage.DocumentationConfig{}
		}
		if lp.Documentation.APIDocs != "" {
			p.Documentation.APIDocs = lp.Documentation.APIDocs
			r.Provenance["process.documentation.api_docs"] = layer.Layer
		}
		if lp.Documentation.Runbook != "" {
			p.Documentation.Runbook = lp.Documentation.Runbook
			r.Provenance["process.documentation.runbook"] = layer.Layer
		}
	}
}

// stripDeletes removes all entries marked with $delete from the final result.
func stripDeletes(c *storage.Constitution) {
	c.Principles = filterSlice(c.Principles, func(p storage.Principle) bool { return !p.Delete })
	c.Antipatterns = filterSlice(c.Antipatterns, func(a storage.Antipattern) bool { return !a.Delete })
	c.References = filterSlice(c.References, func(r storage.Reference) bool { return !r.Delete })
	// Ensure non-nil slices for empty results.
	if c.Principles == nil {
		c.Principles = []storage.Principle{}
	}
	if c.Antipatterns == nil {
		c.Antipatterns = []storage.Antipattern{}
	}
	if c.References == nil {
		c.References = []storage.Reference{}
	}
}

// --- helpers ---

func unionStrings(base, add []string) []string {
	seen := make(map[string]bool, len(base))
	for _, s := range base {
		seen[s] = true
	}
	result := append([]string(nil), base...)
	for _, s := range add {
		if !seen[s] {
			result = append(result, s)
			seen[s] = true
		}
	}
	return result
}

func mergeMaps(base, overlay map[string]string, pathPrefix string, layer storage.ConstitutionLayer, prov map[string]storage.ConstitutionLayer) map[string]string {
	if overlay == nil {
		return base
	}
	if base == nil {
		base = make(map[string]string)
	}
	for k, v := range overlay {
		base[k] = v
		prov[pathPrefix+"["+k+"]"] = layer
	}
	return base
}

func findByKey[T any](slice []T, keyFn func(T) string) map[string]int {
	idx := make(map[string]int, len(slice))
	for i, item := range slice {
		idx[keyFn(item)] = i
	}
	return idx
}

func filterSlice[T any](slice []T, keep func(T) bool) []T {
	result := make([]T, 0, len(slice))
	for _, item := range slice {
		if keep(item) {
			result = append(result, item)
		}
	}
	if len(result) == 0 {
		return result
	}
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/constitution/merge/ -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/constitution/merge/ internal/storage/constitution_domain.go
git commit -m "feat(constitution): add strategic merge engine with delete directives and provenance"
```

---

## Phase 2: Storage Layer

### Task 3: Database Migration

**Files:**
- Create: `internal/storage/postgres/migrations/004_constitution_layers.sql`

- [ ] **Step 1: Write migration**

Create `internal/storage/postgres/migrations/004_constitution_layers.sql`:

```sql
-- SPDX-License-Identifier: MIT
-- Copyright 2026 Sean Brandt

-- +goose Up

-- Drop single-project uniqueness to allow multiple layers per project.
DROP INDEX IF EXISTS idx_constitutions_project;

-- Add source tracking columns for external layer sources.
ALTER TABLE constitutions ADD COLUMN IF NOT EXISTS source_url TEXT NOT NULL DEFAULT '';
ALTER TABLE constitutions ADD COLUMN IF NOT EXISTS source_hash TEXT NOT NULL DEFAULT '';

-- Enforce at most one constitution row per (project, layer).
CREATE UNIQUE INDEX idx_constitutions_project_layer ON constitutions (project_slug, layer);

-- +goose Down

DROP INDEX IF EXISTS idx_constitutions_project_layer;
ALTER TABLE constitutions DROP COLUMN IF EXISTS source_hash;
ALTER TABLE constitutions DROP COLUMN IF EXISTS source_url;
CREATE UNIQUE INDEX idx_constitutions_project ON constitutions (project_slug);
```

- [ ] **Step 2: Verify migration applies**

```bash
task build
```

Expected: build succeeds (migrations are embedded).

- [ ] **Step 3: Commit**

```bash
git add internal/storage/postgres/migrations/004_constitution_layers.sql
git commit -m "feat(storage): add migration for multi-layer constitution support"
```

---

### Task 4: Storage Interface & Postgres Implementation

**Files:**
- Modify: `internal/storage/constitution.go`
- Modify: `internal/storage/postgres/constitution.go`

- [ ] **Step 1: Expand the storage interface**

Replace `internal/storage/constitution.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
)

// ConstitutionBackend defines storage operations for the project constitution.
type ConstitutionBackend interface {
	// GetConstitution returns the active constitution.
	// If no layer filter is specified, returns all layers for the project
	// (caller is responsible for merging).
	// Returns ErrConstitutionNotFound if no layers exist.
	GetConstitution(ctx context.Context) (*Constitution, error)

	// GetConstitutionLayer returns a single layer's raw constitution data.
	// Returns ErrConstitutionNotFound if the layer does not exist.
	GetConstitutionLayer(ctx context.Context, layer ConstitutionLayer) (*Constitution, error)

	// GetAllLayers returns all constitution layers for the project,
	// ordered by precedence (user, org, project, domain).
	// Returns an empty slice (not error) if no layers exist.
	GetAllLayers(ctx context.Context) ([]*Constitution, error)

	// UpdateConstitution stores or replaces a constitution layer,
	// bumping its version. The layer is determined by constitution.Layer.
	UpdateConstitution(ctx context.Context, constitution *Constitution) (*Constitution, error)
}
```

- [ ] **Step 2: Implement Postgres multi-layer storage**

Replace the `GetConstitution`, `UpdateConstitution` methods and add `GetConstitutionLayer`, `GetAllLayers` in `internal/storage/postgres/constitution.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/specgraph/specgraph/internal/storage"
)

// constitutionData is the intermediate struct marshaled into the JSONB data column.
// It excludes identity/version fields that are stored as explicit columns.
type constitutionData struct {
	Tech         *storage.TechStack     `json:"tech,omitempty"`
	Principles   []storage.Principle    `json:"principles,omitempty"`
	Process      *storage.ProcessConfig `json:"process,omitempty"`
	Constraints  []string               `json:"constraints,omitempty"`
	Antipatterns []storage.Antipattern  `json:"antipatterns,omitempty"`
	References   []storage.Reference    `json:"references,omitempty"`
}

// layerPrecedence defines the merge order. User is lowest, domain is highest.
var layerPrecedence = []storage.ConstitutionLayer{
	storage.ConstitutionLayerUser,
	storage.ConstitutionLayerOrg,
	storage.ConstitutionLayerProject,
	storage.ConstitutionLayerDomain,
}

// GetConstitution returns the active constitution for the current project.
// For backward compatibility, returns the single highest-precedence layer.
// Callers needing merged results should use GetAllLayers + merge engine.
func (s *Store) GetConstitution(ctx context.Context) (*storage.Constitution, error) {
	var (
		id        string
		layer     string
		name      string
		version   int32
		dataJSON  []byte
		sourceURL string
		sourceHash string
		createdAt time.Time
		updatedAt time.Time
	)

	err := s.queryRow(ctx,
		`SELECT id, layer, name, version, data, source_url, source_hash, created_at, updated_at
		 FROM constitutions WHERE project_slug = $1
		 ORDER BY version DESC LIMIT 1`,
		s.project,
	).Scan(&id, &layer, &name, &version, &dataJSON, &sourceURL, &sourceHash, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("postgres: %w", storage.ErrConstitutionNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get constitution: %w", err)
	}

	return constitutionFromRow(id, layer, name, version, dataJSON, sourceURL, sourceHash, createdAt, updatedAt)
}

// GetConstitutionLayer returns a single layer's raw constitution data.
func (s *Store) GetConstitutionLayer(ctx context.Context, layer storage.ConstitutionLayer) (*storage.Constitution, error) {
	var (
		id         string
		layerStr   string
		name       string
		version    int32
		dataJSON   []byte
		sourceURL  string
		sourceHash string
		createdAt  time.Time
		updatedAt  time.Time
	)

	err := s.queryRow(ctx,
		`SELECT id, layer, name, version, data, source_url, source_hash, created_at, updated_at
		 FROM constitutions WHERE project_slug = $1 AND layer = $2`,
		s.project, string(layer),
	).Scan(&id, &layerStr, &name, &version, &dataJSON, &sourceURL, &sourceHash, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("postgres: layer %q: %w", layer, storage.ErrConstitutionNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get constitution layer: %w", err)
	}

	return constitutionFromRow(id, layerStr, name, version, dataJSON, sourceURL, sourceHash, createdAt, updatedAt)
}

// GetAllLayers returns all constitution layers for the project,
// ordered by precedence (user, org, project, domain).
func (s *Store) GetAllLayers(ctx context.Context) ([]*storage.Constitution, error) {
	rows, err := s.query(ctx,
		`SELECT id, layer, name, version, data, source_url, source_hash, created_at, updated_at
		 FROM constitutions WHERE project_slug = $1
		 ORDER BY CASE layer
		   WHEN 'user' THEN 1
		   WHEN 'org' THEN 2
		   WHEN 'project' THEN 3
		   WHEN 'domain' THEN 4
		   ELSE 5
		 END`,
		s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: get all layers: %w", err)
	}
	defer rows.Close()

	var layers []*storage.Constitution
	for rows.Next() {
		var (
			id         string
			layer      string
			name       string
			version    int32
			dataJSON   []byte
			sourceURL  string
			sourceHash string
			createdAt  time.Time
			updatedAt  time.Time
		)
		if scanErr := rows.Scan(&id, &layer, &name, &version, &dataJSON, &sourceURL, &sourceHash, &createdAt, &updatedAt); scanErr != nil {
			return nil, fmt.Errorf("postgres: get all layers: scan: %w", scanErr)
		}
		c, err := constitutionFromRow(id, layer, name, version, dataJSON, sourceURL, sourceHash, createdAt, updatedAt)
		if err != nil {
			return nil, err
		}
		layers = append(layers, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: get all layers: iterate: %w", err)
	}
	return layers, nil
}

// UpdateConstitution stores or replaces a constitution layer for the current project.
func (s *Store) UpdateConstitution(ctx context.Context, constitution *storage.Constitution) (*storage.Constitution, error) {
	now := s.now()

	layer := string(constitution.Layer)
	if layer == "" {
		layer = string(storage.ConstitutionLayerProject)
	}

	payload := constitutionData{
		Tech:         constitution.Tech,
		Principles:   constitution.Principles,
		Process:      constitution.Process,
		Constraints:  constitution.Constraints,
		Antipatterns: constitution.Antipatterns,
		References:   constitution.References,
	}

	dataJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("postgres: update constitution marshal: %w", err)
	}

	id := constitution.ID
	if id == "" {
		// Check for existing row at this (project, layer) to reuse ID.
		var existingID string
		existErr := s.queryRow(ctx,
			`SELECT id FROM constitutions WHERE project_slug = $1 AND layer = $2`,
			s.project, layer,
		).Scan(&existingID)
		if existErr != nil && !errors.Is(existErr, pgx.ErrNoRows) {
			return nil, fmt.Errorf("postgres: update constitution: lookup existing: %w", existErr)
		}
		if existingID != "" {
			id = existingID
		} else {
			id = newID("con")
		}
	}

	var (
		retID         string
		retLayer      string
		retName       string
		retVersion    int32
		retDataJSON   []byte
		retSourceURL  string
		retSourceHash string
		retCreatedAt  time.Time
		retUpdatedAt  time.Time
	)

	err = s.queryRow(ctx,
		`INSERT INTO constitutions (id, project_slug, layer, name, version, data, source_url, source_hash, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 1, $5, $6, $7, $8, $8)
		 ON CONFLICT (project_slug, layer) DO UPDATE
		   SET id          = EXCLUDED.id,
		       name        = EXCLUDED.name,
		       data        = EXCLUDED.data,
		       source_url  = EXCLUDED.source_url,
		       source_hash = EXCLUDED.source_hash,
		       version     = constitutions.version + 1,
		       updated_at  = EXCLUDED.updated_at
		 RETURNING id, layer, name, version, data, source_url, source_hash, created_at, updated_at`,
		id, s.project, layer, constitution.Name, dataJSON,
		constitution.SourceURL, constitution.SourceHash, now,
	).Scan(&retID, &retLayer, &retName, &retVersion, &retDataJSON, &retSourceURL, &retSourceHash, &retCreatedAt, &retUpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("postgres: update constitution: %w", err)
	}

	return constitutionFromRow(retID, retLayer, retName, retVersion, retDataJSON, retSourceURL, retSourceHash, retCreatedAt, retUpdatedAt)
}

// constitutionFromRow assembles a *storage.Constitution from scanned column values.
func constitutionFromRow(id, layer, name string, version int32, dataJSON []byte, sourceURL, sourceHash string, createdAt, updatedAt time.Time) (*storage.Constitution, error) {
	var payload constitutionData
	if len(dataJSON) > 0 && string(dataJSON) != "{}" && string(dataJSON) != "null" {
		if err := json.Unmarshal(dataJSON, &payload); err != nil {
			return nil, fmt.Errorf("postgres: constitution unmarshal data: %w", err)
		}
	}

	c := &storage.Constitution{
		ID:           id,
		Layer:        storage.ConstitutionLayer(layer),
		Name:         name,
		Version:      version,
		SourceURL:    sourceURL,
		SourceHash:   sourceHash,
		Tech:         payload.Tech,
		Principles:   payload.Principles,
		Process:      payload.Process,
		Constraints:  payload.Constraints,
		Antipatterns: payload.Antipatterns,
		References:   payload.References,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}

	return c, nil
}
```

- [ ] **Step 3: Update mock backends**

Add `GetConstitutionLayer` and `GetAllLayers` stubs to `stubBackend` in `internal/server/test_scoper_test.go`:

```go
func (stubBackend) GetConstitutionLayer(_ context.Context, _ storage.ConstitutionLayer) (*storage.Constitution, error) {
	return nil, errNotImplemented
}

func (stubBackend) GetAllLayers(_ context.Context) ([]*storage.Constitution, error) {
	return nil, errNotImplemented
}
```

- [ ] **Step 4: Verify build**

```bash
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/storage/constitution.go internal/storage/postgres/constitution.go internal/server/test_scoper_test.go
git commit -m "feat(storage): implement multi-layer constitution storage with per-layer CRUD"
```

---

### Task 5: Storage Integration Tests

**Files:**
- Modify: `internal/storage/postgres/constitution_test.go`

- [ ] **Step 1: Add multi-layer integration tests**

Add to `internal/storage/postgres/constitution_test.go`:

```go
func TestGetAllLayers_ReturnsOrdered(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Store org and project layers.
	_, err := store.UpdateConstitution(ctx, &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Name:  "Org Standards",
		Principles: []storage.Principle{
			{ID: "p1", Statement: "org principle"},
		},
	})
	require.NoError(t, err)

	_, err = store.UpdateConstitution(ctx, &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Name:  "Project Overrides",
		Principles: []storage.Principle{
			{ID: "p2", Statement: "project principle"},
		},
	})
	require.NoError(t, err)

	layers, err := store.GetAllLayers(ctx)
	require.NoError(t, err)
	require.Len(t, layers, 2)
	assert.Equal(t, storage.ConstitutionLayerOrg, layers[0].Layer)
	assert.Equal(t, storage.ConstitutionLayerProject, layers[1].Layer)
}

func TestGetConstitutionLayer_ReturnsSpecific(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.UpdateConstitution(ctx, &storage.Constitution{
		Layer:      storage.ConstitutionLayerOrg,
		Name:       "Org Standards",
		Constraints: []string{"no vendor lock-in"},
	})
	require.NoError(t, err)

	_, err = store.UpdateConstitution(ctx, &storage.Constitution{
		Layer:      storage.ConstitutionLayerProject,
		Name:       "Project",
		Constraints: []string{"all APIs idempotent"},
	})
	require.NoError(t, err)

	// Fetch org layer only.
	org, err := store.GetConstitutionLayer(ctx, storage.ConstitutionLayerOrg)
	require.NoError(t, err)
	assert.Equal(t, []string{"no vendor lock-in"}, org.Constraints)

	// Fetch project layer only.
	proj, err := store.GetConstitutionLayer(ctx, storage.ConstitutionLayerProject)
	require.NoError(t, err)
	assert.Equal(t, []string{"all APIs idempotent"}, proj.Constraints)
}

func TestGetConstitutionLayer_NotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.GetConstitutionLayer(ctx, storage.ConstitutionLayerDomain)
	assert.ErrorIs(t, err, storage.ErrConstitutionNotFound)
}

func TestUpdateConstitution_UpsertSameLayer(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	c1, err := store.UpdateConstitution(ctx, &storage.Constitution{
		Layer:      storage.ConstitutionLayerOrg,
		Name:       "Org v1",
		Constraints: []string{"original"},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), c1.Version)

	c2, err := store.UpdateConstitution(ctx, &storage.Constitution{
		Layer:      storage.ConstitutionLayerOrg,
		Name:       "Org v2",
		Constraints: []string{"updated"},
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), c2.Version)

	// Only one row for org layer.
	layers, err := store.GetAllLayers(ctx)
	require.NoError(t, err)
	require.Len(t, layers, 1)
	assert.Equal(t, []string{"updated"}, layers[0].Constraints)
}

func TestUpdateConstitution_IndependentLayers(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.UpdateConstitution(ctx, &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Name:  "Org",
	})
	require.NoError(t, err)

	_, err = store.UpdateConstitution(ctx, &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Name:  "Project",
	})
	require.NoError(t, err)

	// Update org — project should be unaffected.
	_, err = store.UpdateConstitution(ctx, &storage.Constitution{
		Layer: storage.ConstitutionLayerOrg,
		Name:  "Org Updated",
	})
	require.NoError(t, err)

	proj, err := store.GetConstitutionLayer(ctx, storage.ConstitutionLayerProject)
	require.NoError(t, err)
	assert.Equal(t, int32(1), proj.Version) // unchanged
	assert.Equal(t, "Project", proj.Name)
}

func TestGetAllLayers_Empty(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	layers, err := store.GetAllLayers(ctx)
	require.NoError(t, err)
	assert.Empty(t, layers)
}
```

- [ ] **Step 2: Run integration tests**

```bash
go test -tags integration ./internal/storage/postgres/ -run "TestGetAllLayers\|TestGetConstitutionLayer\|TestUpdateConstitution_Upsert\|TestUpdateConstitution_Independent" -v -timeout 120s
```

- [ ] **Step 3: Commit**

```bash
git add internal/storage/postgres/constitution_test.go
git commit -m "test(storage): add multi-layer constitution integration tests"
```

---

## Phase 3: Proto, Handler, CLI

### Task 6: Proto Changes

**Files:**
- Modify: `proto/specgraph/v1/constitution.proto`

- [ ] **Step 1: Add layer filter, provenance, and source fields to proto**

In `proto/specgraph/v1/constitution.proto`:

Add `source_url` and `source_hash` fields to the `Constitution` message (after the last existing field):

```protobuf
  string source_url = 14;   // where this layer was imported from (empty = local)
  string source_hash = 15;  // content hash of source at import time
```

Add `layer` to `GetConstitutionRequest`:

```protobuf
message GetConstitutionRequest {
  ConstitutionLayer layer = 1; // 0 (UNSPECIFIED) = return merged result
}
```

Add `ProvenanceEntry` message and `provenance` field to `GetConstitutionResponse`:

```protobuf
message ProvenanceEntry {
  string path = 1;
  ConstitutionLayer layer = 2;
}

message GetConstitutionResponse {
  Constitution constitution = 1;
  repeated ProvenanceEntry provenance = 2;
}
```

Note: Check the current field numbers in the proto file before adding. The numbers above (14, 15 for Constitution) assume fields 1-13 are already used. Verify and adjust.

- [ ] **Step 2: Generate Go code**

```bash
task proto
```

- [ ] **Step 3: Verify build (will fail until handler is updated)**

```bash
go build ./gen/...
```

- [ ] **Step 4: Commit**

```bash
git add proto/ gen/
git commit -m "feat(proto): add layer filter, provenance, and source tracking to constitution"
```

---

### Task 7: Handler — Merge on Read

**Files:**
- Modify: `internal/server/constitution_handler.go`
- Modify: `internal/server/convert_constitution.go`

- [ ] **Step 1: Update GetConstitution handler to support layer filter and merge**

In `internal/server/constitution_handler.go`, replace the `GetConstitution` method:

```go
func (h *ConstitutionHandler) GetConstitution(ctx context.Context, req *connect.Request[specv1.GetConstitutionRequest]) (*connect.Response[specv1.GetConstitutionResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}

	msg := req.Msg

	// Single layer query — return raw data, no merge.
	if msg.Layer != specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED {
		domainLayer, ok := constitutionLayerFromProtoMap[msg.Layer]
		if !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown layer: %s", msg.Layer))
		}
		c, err := store.GetConstitutionLayer(ctx, domainLayer)
		if err != nil {
			return nil, constitutionError(err)
		}
		return connect.NewResponse(&specv1.GetConstitutionResponse{
			Constitution: constitutionToProto(c),
		}), nil
	}

	// Merged query — fetch all layers and merge.
	layers, err := store.GetAllLayers(ctx)
	if err != nil {
		return nil, constitutionError(err)
	}
	if len(layers) == 0 {
		return nil, constitutionError(fmt.Errorf("postgres: %w", storage.ErrConstitutionNotFound))
	}

	result, mergeErr := merge.Layers(layers)
	if mergeErr != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("merge layers: %w", mergeErr))
	}

	return connect.NewResponse(&specv1.GetConstitutionResponse{
		Constitution: constitutionToProto(result.Constitution),
		Provenance:   provenanceToProto(result.Provenance),
	}), nil
}
```

Add the import for the merge package:

```go
"github.com/specgraph/specgraph/internal/constitution/merge"
```

- [ ] **Step 2: Add provenance conversion helper**

In `internal/server/convert_constitution.go`:

```go
func provenanceToProto(prov map[string]storage.ConstitutionLayer) []*specv1.ProvenanceEntry {
	if len(prov) == 0 {
		return nil
	}
	entries := make([]*specv1.ProvenanceEntry, 0, len(prov))
	for path, layer := range prov {
		protoLayer, ok := constitutionLayerToProtoMap[layer]
		if !ok {
			continue
		}
		entries = append(entries, &specv1.ProvenanceEntry{
			Path:  path,
			Layer: protoLayer,
		})
	}
	return entries
}
```

- [ ] **Step 3: Update EmitToolFiles to use merged constitution**

In the `EmitToolFiles` method, replace `store.GetConstitution(ctx)` with fetch-all + merge:

```go
layers, err := store.GetAllLayers(ctx)
if err != nil {
	return nil, constitutionError(err)
}
if len(layers) == 0 {
	return nil, constitutionError(fmt.Errorf("postgres: %w", storage.ErrConstitutionNotFound))
}
result, mergeErr := merge.Layers(layers)
if mergeErr != nil {
	return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("merge layers: %w", mergeErr))
}
c := result.Constitution
```

- [ ] **Step 4: Update conversion functions for source fields**

In `constitutionToProto`, add:

```go
pb.SourceUrl = c.SourceURL
pb.SourceHash = c.SourceHash
```

In `constitutionFromProto`, add:

```go
c.SourceURL = pb.SourceUrl
c.SourceHash = pb.SourceHash
```

- [ ] **Step 5: Verify build**

```bash
go build ./...
```

- [ ] **Step 6: Commit**

```bash
git add internal/server/constitution_handler.go internal/server/convert_constitution.go
git commit -m "feat(server): wire merge engine into GetConstitution with layer filter and provenance"
```

---

### Task 8: CLI — `--layer` Flag

**Files:**
- Modify: `cmd/specgraph/constitution.go`

- [ ] **Step 1: Add `--layer` flag to import command**

Add a `constitutionImportLayer` variable and register the flag:

```go
var constitutionImportLayer string

func init() {
	constitutionImportCmd.Flags().StringVar(&constitutionImportLayer, "layer", "", "constitution layer (user|org|project|domain; default: project)")
	// ... existing flag registrations
}
```

In the import handler, resolve the layer with precedence: `--layer` flag > YAML `layer:` field > default `"project"`:

```go
// After parsing YAML and converting to proto:
if constitutionImportLayer != "" {
	protoLayer, ok := constitutionLayerStringToProto(constitutionImportLayer)
	if !ok {
		return fmt.Errorf("invalid layer: %s (must be user, org, project, or domain)", constitutionImportLayer)
	}
	pb.Layer = protoLayer
}
if pb.Layer == specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED {
	pb.Layer = specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT
}
```

- [ ] **Step 2: Add `--layer` flag to show command**

Add a `constitutionShowLayer` variable:

```go
var constitutionShowLayer string

func init() {
	constitutionShowCmd.Flags().StringVar(&constitutionShowLayer, "layer", "", "show specific layer (user|org|project|domain; default: merged)")
	// ... existing flags
}
```

In the show handler, pass the layer to the request:

```go
req := &specv1.GetConstitutionRequest{}
if constitutionShowLayer != "" {
	protoLayer, ok := constitutionLayerStringToProto(constitutionShowLayer)
	if !ok {
		return fmt.Errorf("invalid layer: %s", constitutionShowLayer)
	}
	req.Layer = protoLayer
}
```

- [ ] **Step 3: Verify build**

```bash
go build ./cmd/specgraph/
```

- [ ] **Step 4: Commit**

```bash
git add cmd/specgraph/constitution.go
git commit -m "feat(cli): add --layer flag to constitution import and show"
```

---

## Phase 4: Web Dashboard

### Task 9: Constitution Page — Provenance Badges

**Files:**
- Modify: `web/src/routes/constitution/+page.svelte`

- [ ] **Step 1: Update constitution page to display provenance badges**

In the `<script>` section, add state for provenance and update the load function:

```ts
import type { ProvenanceEntry } from '$lib/api/gen/specgraph/v1/constitution_pb';

let provenance = $state<ProvenanceEntry[]>([]);

// In the load function, after fetching:
const resp = await constitutionClient.getConstitution({});
constitution = resp.constitution ?? null;
provenance = resp.provenance ?? [];
```

Add a helper function to look up provenance:

```ts
function layerOf(path: string): string {
	const entry = provenance.find((p) => p.path === path);
	if (!entry) return '';
	const labels: Record<number, string> = { 1: 'user', 2: 'org', 3: 'project', 4: 'domain' };
	return labels[entry.layer] ?? '';
}
```

Add a provenance badge component inline:

```svelte
{#snippet layerBadge(path: string)}
	{@const l = layerOf(path)}
	{#if l}
		<span class="layer-badge layer-{l}">{l}</span>
	{/if}
{/snippet}
```

Use `{@render layerBadge("principles[p1]")}` next to principles, antipatterns, constraints, tech items, and process fields.

Add badge styles:

```css
.layer-badge {
	font-size: 0.65rem;
	padding: 0.1rem 0.3rem;
	border-radius: 3px;
	font-weight: 600;
	text-transform: uppercase;
	letter-spacing: 0.03em;
	margin-left: 0.375rem;
}
.layer-user { background: #dbeafe; color: #1e40af; }
.layer-org { background: #fef3c7; color: #92400e; }
.layer-project { background: #dcfce7; color: #166534; }
.layer-domain { background: #ede9fe; color: #5b21b6; }
```

- [ ] **Step 2: Verify web build**

```bash
cd web && pnpm build
```

- [ ] **Step 3: Commit**

```bash
git add web/src/routes/constitution/+page.svelte
git commit -m "feat(web): add provenance layer badges to constitution page"
```

---

## Phase 5: E2E Tests

### Task 10: E2E API Tests

**Files:**
- Modify: `e2e/api/constitution_test.go`

- [ ] **Step 1: Add multi-layer E2E tests**

Add to the existing constitution test suite:

```go
Context("Multi-layer", Ordered, func() {
	It("stores org and project layers independently", func() {
		_, err := client.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
			Constitution: &specv1.Constitution{
				Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
				Name:  "Org Standards",
				Tech: &specv1.TechConfig{
					Languages: &specv1.LanguageConfig{
						Primary: "go",
						Allowed: []string{"go", "python"},
					},
				},
				Principles: []*specv1.Principle{
					{Id: "p1", Statement: "org principle 1"},
					{Id: "p2", Statement: "org principle 2"},
				},
			},
		}))
		Expect(err).NotTo(HaveOccurred())

		_, err = client.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
			Constitution: &specv1.Constitution{
				Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
				Name:  "Project Overrides",
				Tech: &specv1.TechConfig{
					Languages: &specv1.LanguageConfig{
						Allowed: []string{"go", "typescript"},
					},
				},
				Principles: []*specv1.Principle{
					{Id: "p2", Statement: "project overrides p2"},
					{Id: "p3", Statement: "project adds p3"},
				},
			},
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns merged constitution with provenance", func() {
		resp, err := client.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
		Expect(err).NotTo(HaveOccurred())

		c := resp.Msg.Constitution
		// Languages merged: go, python (org), typescript (project)
		Expect(c.Tech.Languages.Allowed).To(ContainElements("go", "python", "typescript"))
		// Primary from org
		Expect(c.Tech.Languages.Primary).To(Equal("go"))
		// p1 from org, p2 overridden by project, p3 from project
		Expect(c.Principles).To(HaveLen(3))

		// Provenance populated
		prov := resp.Msg.Provenance
		Expect(len(prov)).To(BeNumerically(">", 0))
	})

	It("returns raw layer with layer filter", func() {
		resp, err := client.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{
			Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
		}))
		Expect(err).NotTo(HaveOccurred())

		c := resp.Msg.Constitution
		Expect(c.Name).To(Equal("Org Standards"))
		// Raw layer — no merge, no provenance
		Expect(resp.Msg.Provenance).To(BeEmpty())
	})
})
```

- [ ] **Step 2: Verify compilation**

```bash
go build -tags e2e ./e2e/api/...
```

- [ ] **Step 3: Commit**

```bash
git add e2e/api/constitution_test.go
git commit -m "test(e2e): add multi-layer constitution API tests"
```

---

## Phase 6: Documentation

### Task 11: Update Site Documentation

**Files:**
- Modify: `site/docs/concepts/constitution.md`
- Modify: `site/docs/guides/cli-cookbook.md`

- [ ] **Step 1: Update constitution concept page**

Remove the "Planned" admonition. Replace with documentation of multi-layer behavior:

- How layers work (User → Org → Project → Domain, higher overrides lower)
- How to import per-layer constitutions
- Merge semantics (scalars: override, lists: union, keyed objects: merge by key)
- `$delete` directives with example YAML
- Viewing merged vs individual layers
- Provenance in dashboard

- [ ] **Step 2: Add CLI cookbook recipe**

Add a recipe to `guides/cli-cookbook.md` for multi-layer constitution setup, showing import of org + project layers, viewing merged output, and checking individual layers.

- [ ] **Step 3: Regenerate CLI reference**

```bash
./specgraph docs cli
```

- [ ] **Step 4: Verify docs build**

```bash
cd site && uv run zensical build
```

- [ ] **Step 5: Commit**

```bash
git add site/docs/ site/zensical.toml
git commit -m "docs(site): document multi-layer constitution with merge semantics and provenance"
```

---

## Phase 7: Quality Gates

### Task 12: Quality Gates & Final Verification

- [ ] **Step 1: Run task check**

```bash
task check
```

- [ ] **Step 2: Run integration tests**

```bash
go test -tags integration ./internal/storage/postgres/ -run "TestGetAllLayers\|TestGetConstitutionLayer\|TestUpdateConstitution" -v -timeout 120s
```

- [ ] **Step 3: Run web build**

```bash
cd web && pnpm build
```

- [ ] **Step 4: Run docs build**

```bash
cd site && uv run zensical build
```

- [ ] **Step 5: Commit any fixups**

```bash
git status
```
