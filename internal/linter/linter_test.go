// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package linter_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/seanb4t/specgraph/internal/linter"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

// mockLintBackend implements linter.Backend for testing.
type mockLintBackend struct {
	specs              map[string]*storage.Spec
	deps               map[string][]storage.NodeRef
	getSpecErr         error            // non-nil overrides GetSpec with this error for all slugs
	listSpecErr        error            // non-nil overrides ListSpecs with this error
	getDepsErr         error            // non-nil overrides GetDependencies with this error for all slugs
	getDepsErrMap      map[string]error // per-slug errors checked before getDepsErr
	listSpecsLastLimit int              // captures the limit arg passed to ListSpecs
}

func (m *mockLintBackend) GetSpec(_ context.Context, slug string) (*storage.Spec, error) {
	if m.getSpecErr != nil {
		return nil, m.getSpecErr
	}
	spec, ok := m.specs[slug]
	if !ok {
		return nil, storage.ErrSpecNotFound
	}
	return spec, nil
}

func (m *mockLintBackend) ListSpecs(_ context.Context, _, _ string, limit int) ([]*storage.Spec, error) {
	m.listSpecsLastLimit = limit
	if m.listSpecErr != nil {
		return nil, m.listSpecErr
	}
	specs := make([]*storage.Spec, 0, len(m.specs))
	for _, s := range m.specs {
		specs = append(specs, s)
	}
	return specs, nil
}

func (m *mockLintBackend) GetDependencies(_ context.Context, slug string) ([]storage.NodeRef, error) {
	if err, ok := m.getDepsErrMap[slug]; ok {
		return nil, err
	}
	if m.getDepsErr != nil {
		return nil, m.getDepsErr
	}
	return m.deps[slug], nil
}

func TestLint_SchemaViolation(t *testing.T) {
	backend := &mockLintBackend{
		specs: map[string]*storage.Spec{
			"bad-spec": {
				Slug:    "bad-spec",
				Intent:  "", // missing intent → schema violation
				Stage:   storage.SpecStageSpark,
				Version: 1,
			},
		},
		deps: map[string][]storage.NodeRef{},
	}

	results, err := linter.NewEngine(backend).Lint(context.Background(), "bad-spec")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.False(t, results[0].Passed)
	require.Equal(t, "bad-spec", results[0].SpecSlug)

	hasSchemaViolation := false
	for _, v := range results[0].Violations {
		if v.Rule == "schema.required" && v.Location == "intent" {
			hasSchemaViolation = true
		}
	}
	require.True(t, hasSchemaViolation, "expected schema.required violation for intent")
}

func TestLint_DanglingDependency(t *testing.T) {
	backend := &mockLintBackend{
		specs: map[string]*storage.Spec{
			"spec-a": {
				Slug:    "spec-a",
				Intent:  "Do something",
				Stage:   storage.SpecStageSpark,
				Version: 1,
			},
		},
		deps: map[string][]storage.NodeRef{
			"spec-a": {
				{Slug: "nonexistent-spec", Label: storage.NodeLabelSpec},
			},
		},
	}

	results, err := linter.NewEngine(backend).Lint(context.Background(), "spec-a")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.False(t, results[0].Passed)

	hasDangling := false
	for _, v := range results[0].Violations {
		if v.Rule == "edge.dangling_ref" {
			hasDangling = true
			require.Equal(t, storage.LintSeverityError, v.Severity)
		}
	}
	require.True(t, hasDangling, "expected edge.dangling_ref violation")
}

func TestLint_CycleDetection(t *testing.T) {
	backend := &mockLintBackend{
		specs: map[string]*storage.Spec{
			"spec-a": {
				Slug:    "spec-a",
				Intent:  "First spec",
				Stage:   storage.SpecStageSpark,
				Version: 1,
			},
			"spec-b": {
				Slug:    "spec-b",
				Intent:  "Second spec",
				Stage:   storage.SpecStageSpark,
				Version: 1,
			},
		},
		deps: map[string][]storage.NodeRef{
			"spec-a": {{Slug: "spec-b", Label: storage.NodeLabelSpec}},
			"spec-b": {{Slug: "spec-a", Label: storage.NodeLabelSpec}},
		},
	}

	results, err := linter.NewEngine(backend).Lint(context.Background(), "spec-a")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.False(t, results[0].Passed)

	hasCycle := false
	for _, v := range results[0].Violations {
		if v.Rule == "graph.cycle" {
			hasCycle = true
			require.Equal(t, storage.LintSeverityError, v.Severity)
		}
	}
	require.True(t, hasCycle, "expected graph.cycle violation")
}

func TestLint_ValidSpec(t *testing.T) {
	backend := &mockLintBackend{
		specs: map[string]*storage.Spec{
			"spec-a": {
				Slug:    "spec-a",
				Intent:  "Do something",
				Stage:   storage.SpecStageSpark,
				Version: 1,
			},
			"spec-b": {
				Slug:    "spec-b",
				Intent:  "Do something else",
				Stage:   storage.SpecStageSpark,
				Version: 1,
			},
		},
		deps: map[string][]storage.NodeRef{
			"spec-a": {{Slug: "spec-b", Label: storage.NodeLabelSpec}},
		},
	}

	results, err := linter.NewEngine(backend).Lint(context.Background(), "spec-a")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.True(t, results[0].Passed)
	require.Empty(t, results[0].Violations)
}

func TestLint_AllSpecs(t *testing.T) {
	backend := &mockLintBackend{
		specs: map[string]*storage.Spec{
			"spec-a": {
				Slug:    "spec-a",
				Intent:  "Do something",
				Stage:   storage.SpecStageSpark,
				Version: 1,
			},
			"spec-b": {
				Slug:    "spec-b",
				Intent:  "Do something else",
				Stage:   storage.SpecStageSpark,
				Version: 1,
			},
		},
		deps: map[string][]storage.NodeRef{},
	}

	results, err := linter.NewEngine(backend).Lint(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, results, 2)

	for _, r := range results {
		require.True(t, r.Passed)
		require.Empty(t, r.Violations)
	}
}

func TestLint_ListSpecsStorageError(t *testing.T) {
	dbErr := errors.New("connection refused")
	backend := &mockLintBackend{
		specs:       map[string]*storage.Spec{},
		deps:        map[string][]storage.NodeRef{},
		listSpecErr: dbErr,
	}

	_, err := linter.NewEngine(backend).Lint(context.Background(), "")
	require.Error(t, err)
	require.ErrorIs(t, err, dbErr)
}

func TestLint_GetSpecStorageError(t *testing.T) {
	dbErr := errors.New("connection refused")
	backend := &mockLintBackend{
		specs:      map[string]*storage.Spec{},
		deps:       map[string][]storage.NodeRef{},
		getSpecErr: dbErr,
	}

	_, err := linter.NewEngine(backend).Lint(context.Background(), "some-spec")
	require.Error(t, err)
	require.ErrorIs(t, err, dbErr)
}

func TestLint_GetDependenciesStorageError(t *testing.T) {
	dbErr := errors.New("connection refused")
	backend := &mockLintBackend{
		specs: map[string]*storage.Spec{
			"spec-a": {
				Slug:    "spec-a",
				Intent:  "Do something",
				Stage:   storage.SpecStageSpark,
				Version: 1,
			},
		},
		deps:       map[string][]storage.NodeRef{},
		getDepsErr: dbErr,
	}

	results, err := linter.NewEngine(backend).Lint(context.Background(), "spec-a")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotEmpty(t, results[0].Error, "expected per-spec error in LintResult")
	require.Contains(t, results[0].Error, dbErr.Error())
}

func TestLint_GetDependenciesMidTraversalStorageError(t *testing.T) {
	dbErr := errors.New("connection reset")
	backend := &mockLintBackend{
		specs: map[string]*storage.Spec{
			"root": {
				Slug:    "root",
				Intent:  "Root spec",
				Stage:   storage.SpecStageSpark,
				Version: 1,
			},
			"child": {
				Slug:    "child",
				Intent:  "Child spec",
				Stage:   storage.SpecStageSpark,
				Version: 1,
			},
		},
		deps: map[string][]storage.NodeRef{
			"root": {{Slug: "child"}},
		},
		getDepsErrMap: map[string]error{
			"child": dbErr,
		},
	}

	results, err := linter.NewEngine(backend).Lint(context.Background(), "root")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotEmpty(t, results[0].Error, "expected per-spec error in LintResult")
	require.Contains(t, results[0].Error, dbErr.Error())
}

func TestLint_MaxCycleDepthExceeded(t *testing.T) {
	const chainLen = 1002
	specs := make(map[string]*storage.Spec, chainLen)
	deps := make(map[string][]storage.NodeRef, chainLen-1)
	for i := 0; i < chainLen; i++ {
		slug := fmt.Sprintf("spec-%d", i)
		specs[slug] = &storage.Spec{
			Slug:    slug,
			Intent:  "chain node",
			Stage:   storage.SpecStageSpark,
			Version: 1,
		}
		if i < chainLen-1 {
			deps[slug] = []storage.NodeRef{{Slug: fmt.Sprintf("spec-%d", i+1)}}
		}
	}
	backend := &mockLintBackend{specs: specs, deps: deps}

	results, err := linter.NewEngine(backend).Lint(context.Background(), "spec-0")
	require.NoError(t, err)
	require.Len(t, results, 1)
	var found bool
	for _, v := range results[0].Violations {
		if v.Rule == "graph.cycle" && v.Severity == storage.LintSeverityWarning {
			found = true
			require.Contains(t, v.Message, "exceeds maximum depth")
			break
		}
	}
	require.True(t, found, "expected graph.cycle warning for depth exceeded, got violations: %v", results[0].Violations)
}

func TestLint_MissingTransitiveDep(t *testing.T) {
	// spec-A → spec-B → spec-C, but spec-C is missing from storage.
	// spec-C is a transitive dep (not a direct dep of spec-A), so it must NOT
	// appear in danglingSet and must trigger the 'graph.missing_node' warning
	// (not 'edge.dangling_ref').
	backend := &mockLintBackend{
		specs: map[string]*storage.Spec{
			"spec-a": {
				Slug:    "spec-a",
				Intent:  "Top-level spec",
				Stage:   storage.SpecStageSpark,
				Version: 1,
			},
			"spec-b": {
				Slug:    "spec-b",
				Intent:  "Intermediate spec",
				Stage:   storage.SpecStageSpark,
				Version: 1,
			},
			// spec-c intentionally absent
		},
		deps: map[string][]storage.NodeRef{
			"spec-a": {{Slug: "spec-b", Label: storage.NodeLabelSpec}},
			"spec-b": {{Slug: "spec-c", Label: storage.NodeLabelSpec}},
		},
		// GetDependencies("spec-c") returns ErrSpecNotFound to simulate a node
		// that exists as a dep target but has no entry in the graph.
		getDepsErrMap: map[string]error{
			"spec-c": storage.ErrSpecNotFound,
		},
	}

	results, err := linter.NewEngine(backend).Lint(context.Background(), "spec-a")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.False(t, results[0].Passed)

	var hasMissingNode bool
	var hasDangling bool
	for _, v := range results[0].Violations {
		if v.Rule == "graph.missing_node" && v.Location == "spec-c" {
			hasMissingNode = true
			require.Equal(t, storage.LintSeverityWarning, v.Severity)
		}
		if v.Rule == "edge.dangling_ref" {
			hasDangling = true
		}
	}
	require.True(t, hasMissingNode, "expected graph.missing_node violation for spec-c")
	require.False(t, hasDangling, "must not emit edge.dangling_ref for a transitive dep")
}

func TestLint_CycleDetection_StorageErrorPropagates(t *testing.T) {
	dbErr := errors.New("database connection lost")
	backend := &mockLintBackend{
		specs: map[string]*storage.Spec{
			"root": {
				Slug:    "root",
				Intent:  "Root spec",
				Stage:   storage.SpecStageSpark,
				Version: 1,
			},
			"child": {
				Slug:    "child",
				Intent:  "Child spec",
				Stage:   storage.SpecStageSpark,
				Version: 1,
			},
		},
		deps: map[string][]storage.NodeRef{
			"root": {{Slug: "child", Label: storage.NodeLabelSpec}},
		},
		// GetDependencies for "child" returns an error during DFS traversal.
		getDepsErrMap: map[string]error{
			"child": dbErr,
		},
	}

	results, err := linter.NewEngine(backend).Lint(context.Background(), "root")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotEmpty(t, results[0].Error, "expected per-spec error in LintResult")
	require.Contains(t, results[0].Error, "database connection lost")
}

func TestLint_SelfReferentialCycle(t *testing.T) {
	backend := &mockLintBackend{
		specs: map[string]*storage.Spec{
			"spec-a": {
				Slug:    "spec-a",
				Intent:  "Self-referencing spec",
				Stage:   storage.SpecStageSpark,
				Version: 1,
			},
		},
		deps: map[string][]storage.NodeRef{
			"spec-a": {{Slug: "spec-a", Label: storage.NodeLabelSpec}},
		},
	}

	results, err := linter.NewEngine(backend).Lint(context.Background(), "spec-a")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.False(t, results[0].Passed)

	hasCycle := false
	for _, v := range results[0].Violations {
		if v.Rule == "graph.cycle" {
			hasCycle = true
			require.Contains(t, v.Message, "spec-a")
			break
		}
	}
	require.True(t, hasCycle, "expected graph.cycle violation for self-referential dep")
}

func TestLint_AllSpecs_PassesMaxSpecsPerLintAsLimit(t *testing.T) {
	backend := &mockLintBackend{specs: map[string]*storage.Spec{}}
	engine := linter.NewEngine(backend)
	_, _ = engine.Lint(context.Background(), "") // all-specs path
	// maxSpecsPerLint is 10000 (unexported constant in linter.go).
	require.Equal(t, 10000, backend.listSpecsLastLimit,
		"ListSpecs should be called with maxSpecsPerLint (10000) as limit")
}
