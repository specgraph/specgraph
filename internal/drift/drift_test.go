// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package drift_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/seanb4t/specgraph/internal/drift"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

type mockDriftBackend struct {
	specs   map[string]*storage.Spec
	deps    map[string][]storage.NodeRef
	listErr error // if non-nil, ListSpecs returns this error
	depsErr error // if non-nil, GetDependencies returns this error
	specErr error // if non-nil, GetSpec returns this error for any slug
}

func (m *mockDriftBackend) GetSpec(_ context.Context, slug string) (*storage.Spec, error) {
	if m.specErr != nil {
		return nil, m.specErr
	}
	spec, ok := m.specs[slug]
	if !ok {
		return nil, storage.ErrSpecNotFound
	}
	return spec, nil
}

func (m *mockDriftBackend) ListSpecs(_ context.Context, stage, _ string, _ int) ([]*storage.Spec, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []*storage.Spec
	for _, s := range m.specs {
		if string(s.Stage) == stage {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockDriftBackend) GetDependencies(_ context.Context, slug string) ([]storage.NodeRef, error) {
	if m.depsErr != nil {
		return nil, m.depsErr
	}
	return m.deps[slug], nil
}

func TestCheckDependencyDrift(t *testing.T) {
	now := time.Now()
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"downstream": {
				Slug:      "downstream",
				Stage:     storage.SpecStageDone,
				UpdatedAt: now.Add(-time.Hour),
			},
			"upstream": {
				Slug:      "upstream",
				Stage:     storage.SpecStageDone,
				UpdatedAt: now,
			},
		},
		deps: map[string][]storage.NodeRef{
			"downstream": {
				{Slug: "upstream", Label: storage.NodeLabelSpec},
			},
		},
	}

	engine := drift.NewEngine(backend)
	reports, err := engine.Check(context.Background(), "downstream", "")
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Len(t, reports[0].Items, 1)
	require.Equal(t, storage.DriftTypeDependency, reports[0].Items[0].Type)
	require.Equal(t, storage.DriftSeverityMedium, reports[0].Items[0].Severity)
	require.Equal(t, "downstream", reports[0].Items[0].SpecSlug)
	require.Equal(t, "upstream", reports[0].Items[0].UpstreamSlug)
}

func TestCheckDependencyDrift_NoDrift(t *testing.T) {
	now := time.Now()
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"downstream": {
				Slug:      "downstream",
				Stage:     storage.SpecStageDone,
				UpdatedAt: now,
			},
			"upstream": {
				Slug:      "upstream",
				Stage:     storage.SpecStageDone,
				UpdatedAt: now.Add(-time.Hour),
			},
		},
		deps: map[string][]storage.NodeRef{
			"downstream": {
				{Slug: "upstream", Label: storage.NodeLabelSpec},
			},
		},
	}

	engine := drift.NewEngine(backend)
	reports, err := engine.Check(context.Background(), "downstream", "")
	require.NoError(t, err)
	require.Empty(t, reports, "no-drift specs should be filtered out")
}

func TestCheckAllSpecs(t *testing.T) {
	now := time.Now()
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"done-spec": {
				Slug:      "done-spec",
				Stage:     storage.SpecStageDone,
				UpdatedAt: now.Add(-time.Hour),
			},
			"amended-spec": {
				Slug:      "amended-spec",
				Stage:     storage.SpecStageAmended,
				UpdatedAt: now.Add(-time.Hour),
			},
			"upstream": {
				Slug:      "upstream",
				Stage:     storage.SpecStageApproved,
				UpdatedAt: now,
			},
		},
		deps: map[string][]storage.NodeRef{
			"done-spec": {
				{Slug: "upstream", Label: storage.NodeLabelSpec},
			},
			"amended-spec": {
				{Slug: "upstream", Label: storage.NodeLabelSpec},
			},
		},
	}

	engine := drift.NewEngine(backend)
	reports, err := engine.Check(context.Background(), "", "")
	require.NoError(t, err)
	require.Len(t, reports, 2)

	// Both specs should have drift items since upstream is newer.
	for _, r := range reports {
		require.Len(t, r.Items, 1, "spec %s should have 1 drift item", r.SpecSlug)
	}
}

func TestCheckDrift_ScopeFilter(t *testing.T) {
	now := time.Now()
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"downstream": {
				Slug:      "downstream",
				Stage:     storage.SpecStageDone,
				UpdatedAt: now.Add(-time.Hour),
			},
			"upstream": {
				Slug:      "upstream",
				Stage:     storage.SpecStageDone,
				UpdatedAt: now,
			},
		},
		deps: map[string][]storage.NodeRef{
			"downstream": {
				{Slug: "upstream", Label: storage.NodeLabelSpec},
			},
		},
	}

	engine := drift.NewEngine(backend)

	// scope="interfaces" → no drift (placeholder), filtered out.
	reports, err := engine.Check(context.Background(), "downstream", "interfaces")
	require.NoError(t, err)
	require.Empty(t, reports)

	// scope="deps" → drift found.
	reports, err = engine.Check(context.Background(), "downstream", "deps")
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Len(t, reports[0].Items, 1)
	require.Equal(t, storage.DriftTypeDependency, reports[0].Items[0].Type)
}

func TestCheck_InvalidScope(t *testing.T) {
	backend := &mockDriftBackend{specs: map[string]*storage.Spec{}}
	engine := drift.NewEngine(backend)

	_, err := engine.Check(context.Background(), "", "bogus")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown scope")
}

func TestCheck_ListSpecsError(t *testing.T) {
	backend := &mockDriftBackend{
		specs:   map[string]*storage.Spec{},
		listErr: errors.New("db connection lost"),
	}
	engine := drift.NewEngine(backend)

	_, err := engine.Check(context.Background(), "", "")
	require.Error(t, err)
	require.ErrorContains(t, err, "db connection lost")
}

func TestCheckSpec_GetDependenciesError(t *testing.T) {
	now := time.Now()
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"my-spec": {
				Slug:      "my-spec",
				Stage:     storage.SpecStageDone,
				UpdatedAt: now,
			},
		},
		deps:    map[string][]storage.NodeRef{},
		depsErr: errors.New("graph unavailable"),
	}
	engine := drift.NewEngine(backend)

	_, err := engine.Check(context.Background(), "my-spec", "deps")
	require.Error(t, err)
	require.ErrorContains(t, err, "graph unavailable")
}

func TestCheckSpec_MissingDependencyCreatesInfoItem(t *testing.T) {
	now := time.Now()
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"downstream": {
				Slug:      "downstream",
				Stage:     storage.SpecStageDone,
				UpdatedAt: now,
			},
			// "gone-dep" intentionally absent
		},
		deps: map[string][]storage.NodeRef{
			"downstream": {
				{Slug: "gone-dep", Label: storage.NodeLabelSpec},
			},
		},
	}
	engine := drift.NewEngine(backend)

	reports, err := engine.Check(context.Background(), "downstream", "deps")
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Len(t, reports[0].Items, 1)

	item := reports[0].Items[0]
	require.Equal(t, storage.DriftTypeDependency, item.Type)
	require.Equal(t, storage.DriftSeverityMedium, item.Severity)
	require.Contains(t, item.Description, "gone-dep")
	require.Contains(t, item.Description, "not found")
	require.Equal(t, "downstream", item.SpecSlug)
	require.Equal(t, "gone-dep", item.UpstreamSlug)
}
