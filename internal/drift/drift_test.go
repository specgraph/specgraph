// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package drift_test

import (
	"context"
	"errors"
	"testing"

	"github.com/specgraph/specgraph/internal/drift"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

type mockDriftBackend struct {
	specs           map[string]*storage.Spec
	depsWithEdge    map[string][]storage.DependencyRef
	listErr         error            // if non-nil, ListSpecs returns this error for all stages
	listErrForStage map[string]error // per-stage errors; checked before listErr
	depsErr         error            // if non-nil, GetDependenciesWithEdgeData returns this error
	specErr         error            // if non-nil, GetSpec returns this error for any slug
	specErrForSlug  map[string]error // per-slug errors; checked before specErr
}

func (m *mockDriftBackend) GetSpec(_ context.Context, slug string) (*storage.Spec, error) {
	if m.specErrForSlug != nil {
		if err, ok := m.specErrForSlug[slug]; ok {
			return nil, err
		}
	}
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
	if err, ok := m.listErrForStage[stage]; ok {
		return nil, err
	}
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []*storage.Spec
	for _, s := range m.specs {
		if stage == "" || string(s.Stage) == stage {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockDriftBackend) GetDependenciesWithEdgeData(_ context.Context, slug string) ([]storage.DependencyRef, error) {
	if m.depsErr != nil {
		return nil, m.depsErr
	}
	return m.depsWithEdge[slug], nil
}

func TestCheckDependencyDrift(t *testing.T) {
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"downstream": {
				Slug:        "downstream",
				Stage:       storage.SpecStageDone,
				ContentHash: "aaa",
			},
			"upstream": {
				Slug:        "upstream",
				Stage:       storage.SpecStageDone,
				ContentHash: "bbb", // changed from "aaa"
			},
		},
		depsWithEdge: map[string][]storage.DependencyRef{
			"downstream": {
				{
					NodeRef:           storage.NodeRef{Slug: "upstream", Label: storage.NodeLabelSpec},
					ContentHashAtLink: "aaa",
				},
			},
		},
	}

	engine := drift.NewEngine(backend, nil)
	result, err := engine.Check(context.Background(), "downstream", "")
	require.NoError(t, err)
	require.Len(t, result.Reports, 1)
	require.Len(t, result.Reports[0].Items, 1)
	require.Equal(t, storage.DriftTypeDependency, result.Reports[0].Items[0].Type)
	require.Equal(t, storage.DriftSeverityMedium, result.Reports[0].Items[0].Severity)
	require.Equal(t, "downstream", result.Reports[0].Items[0].SpecSlug)
	require.Equal(t, "upstream", result.Reports[0].Items[0].UpstreamSlug)
	require.Equal(t, "aaa", result.Reports[0].Items[0].ExpectedHash)
	require.Equal(t, "bbb", result.Reports[0].Items[0].ActualHash)
	require.Equal(t, int32(0), result.SkippedCount, "single-spec check should not have skipped count")
}

func TestCheckDependencyDrift_NoDrift(t *testing.T) {
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"downstream": {
				Slug:        "downstream",
				Stage:       storage.SpecStageDone,
				ContentHash: "aaa",
			},
			"upstream": {
				Slug:        "upstream",
				Stage:       storage.SpecStageDone,
				ContentHash: "aaa", // matches edge hash
			},
		},
		depsWithEdge: map[string][]storage.DependencyRef{
			"downstream": {
				{
					NodeRef:           storage.NodeRef{Slug: "upstream", Label: storage.NodeLabelSpec},
					ContentHashAtLink: "aaa",
				},
			},
		},
	}

	engine := drift.NewEngine(backend, nil)
	result, err := engine.Check(context.Background(), "downstream", "")
	require.NoError(t, err)
	require.Empty(t, result.Reports, "no-drift specs should be filtered out")
}

func TestCheckDependencyDrift_EmptyEdgeHash(t *testing.T) {
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"downstream": {
				Slug:        "downstream",
				Stage:       storage.SpecStageDone,
				ContentHash: "aaa",
			},
			"upstream": {
				Slug:        "upstream",
				Stage:       storage.SpecStageDone,
				ContentHash: "bbb",
			},
		},
		depsWithEdge: map[string][]storage.DependencyRef{
			"downstream": {
				{
					NodeRef:           storage.NodeRef{Slug: "upstream", Label: storage.NodeLabelSpec},
					ContentHashAtLink: "", // unmigrated edge — always drifted
				},
			},
		},
	}

	engine := drift.NewEngine(backend, nil)
	result, err := engine.Check(context.Background(), "downstream", "")
	require.NoError(t, err)
	require.Len(t, result.Reports, 1)
	require.Len(t, result.Reports[0].Items, 1, "empty edge hash should always produce drift")
	require.Equal(t, "", result.Reports[0].Items[0].ExpectedHash)
	require.Equal(t, "bbb", result.Reports[0].Items[0].ActualHash)
}

func TestCheckAllSpecs(t *testing.T) {
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"done-spec": {
				Slug:        "done-spec",
				Stage:       storage.SpecStageDone,
				ContentHash: "aaa",
			},
			"amended-spec": {
				Slug:        "amended-spec",
				Stage:       storage.SpecStageAmended,
				ContentHash: "aaa",
			},
			"upstream": {
				Slug:        "upstream",
				Stage:       storage.SpecStageApproved,
				ContentHash: "bbb", // changed from "aaa"
			},
		},
		depsWithEdge: map[string][]storage.DependencyRef{
			"done-spec": {
				{
					NodeRef:           storage.NodeRef{Slug: "upstream", Label: storage.NodeLabelSpec},
					ContentHashAtLink: "aaa",
				},
			},
			"amended-spec": {
				{
					NodeRef:           storage.NodeRef{Slug: "upstream", Label: storage.NodeLabelSpec},
					ContentHashAtLink: "aaa",
				},
			},
		},
	}

	engine := drift.NewEngine(backend, nil)
	result, err := engine.Check(context.Background(), "", "")
	require.NoError(t, err)
	require.Len(t, result.Reports, 2)
	// "upstream" is approved (not done/amended), so it was skipped.
	require.Equal(t, int32(1), result.SkippedCount)

	// Both specs should have drift items since upstream content hash changed.
	for _, r := range result.Reports {
		require.Len(t, r.Items, 1, "spec %s should have 1 drift item", r.SpecSlug)
	}
}

func TestCheckDrift_ScopeFilter(t *testing.T) {
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"downstream": {
				Slug:        "downstream",
				Stage:       storage.SpecStageDone,
				ContentHash: "aaa",
			},
			"upstream": {
				Slug:        "upstream",
				Stage:       storage.SpecStageDone,
				ContentHash: "bbb",
			},
		},
		depsWithEdge: map[string][]storage.DependencyRef{
			"downstream": {
				{
					NodeRef:           storage.NodeRef{Slug: "upstream", Label: storage.NodeLabelSpec},
					ContentHashAtLink: "aaa",
				},
			},
		},
	}

	engine := drift.NewEngine(backend, nil)

	// scope="interfaces" -> no items but ErrorMessage indicates not yet implemented.
	result, err := engine.Check(context.Background(), "downstream", "interfaces")
	require.NoError(t, err)
	require.Len(t, result.Reports, 1)
	require.Empty(t, result.Reports[0].Items)
	require.Equal(t, "interface drift checking not yet implemented", result.Reports[0].ErrorMessage)

	// scope="deps" -> drift found.
	result, err = engine.Check(context.Background(), "downstream", "deps")
	require.NoError(t, err)
	require.Len(t, result.Reports, 1)
	require.Len(t, result.Reports[0].Items, 1)
	require.Equal(t, storage.DriftTypeDependency, result.Reports[0].Items[0].Type)
}

func TestCheckDrift_ScopeVerify(t *testing.T) {
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"downstream": {
				Slug:        "downstream",
				Stage:       storage.SpecStageDone,
				ContentHash: "aaa",
			},
			"upstream": {
				Slug:        "upstream",
				Stage:       storage.SpecStageDone,
				ContentHash: "bbb",
			},
		},
		depsWithEdge: map[string][]storage.DependencyRef{
			"downstream": {
				{
					NodeRef:           storage.NodeRef{Slug: "upstream", Label: storage.NodeLabelSpec},
					ContentHashAtLink: "aaa",
				},
			},
		},
	}

	engine := drift.NewEngine(backend, nil)

	// scope="verify" -> no items but ErrorMessage indicates not yet implemented.
	result, err := engine.Check(context.Background(), "downstream", "verify")
	require.NoError(t, err)
	require.Len(t, result.Reports, 1)
	require.Equal(t, "verify drift checking not yet implemented", result.Reports[0].ErrorMessage)
	require.Empty(t, result.Reports[0].Items)
}

func TestCheck_InvalidScope(t *testing.T) {
	backend := &mockDriftBackend{specs: map[string]*storage.Spec{}}
	engine := drift.NewEngine(backend, nil)

	_, err := engine.Check(context.Background(), "", "bogus")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown scope")
}

func TestCheck_ListSpecsError(t *testing.T) {
	backend := &mockDriftBackend{
		specs:   map[string]*storage.Spec{},
		listErr: errors.New("db connection lost"),
	}
	engine := drift.NewEngine(backend, nil)

	_, err := engine.Check(context.Background(), "", "")
	require.Error(t, err)
	require.ErrorContains(t, err, "db connection lost")
}

func TestCheck_ListSpecsError_AmendedStageOnly(t *testing.T) {
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"done-spec": {
				Slug:        "done-spec",
				Stage:       storage.SpecStageDone,
				ContentHash: "aaa",
			},
		},
		listErrForStage: map[string]error{
			string(storage.SpecStageAmended): errors.New("amended stage query failed"),
		},
	}
	engine := drift.NewEngine(backend, nil)

	_, err := engine.Check(context.Background(), "", "")
	require.Error(t, err)
	require.ErrorContains(t, err, "amended stage query failed")
}

func TestCheck_ListAllSpecsError(t *testing.T) {
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"done-spec": {
				Slug:  "done-spec",
				Stage: storage.SpecStageDone,
			},
		},
		listErrForStage: map[string]error{
			"": errors.New("all-specs query failed"),
		},
	}
	engine := drift.NewEngine(backend, nil)

	_, err := engine.Check(context.Background(), "", "")
	require.Error(t, err)
	require.ErrorContains(t, err, "all-specs query failed")
}

func TestCheckSpec_GetDependenciesError(t *testing.T) {
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"my-spec": {
				Slug:        "my-spec",
				Stage:       storage.SpecStageDone,
				ContentHash: "aaa",
			},
		},
		depsWithEdge: map[string][]storage.DependencyRef{},
		depsErr:      errors.New("graph unavailable"),
	}
	engine := drift.NewEngine(backend, nil)

	result, err := engine.Check(context.Background(), "my-spec", "deps")
	require.NoError(t, err)
	require.Len(t, result.Reports, 1)
	require.Equal(t, "my-spec", result.Reports[0].SpecSlug)
	require.Equal(t, "drift check failed", result.Reports[0].ErrorMessage)
	require.Empty(t, result.Reports[0].Items)
}

func TestCheck_NonDoneStageBySlug(t *testing.T) {
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"in-progress": {
				Slug:        "in-progress",
				Stage:       storage.SpecStageShape,
				ContentHash: "aaa",
			},
			"upstream": {
				Slug:        "upstream",
				Stage:       storage.SpecStageDone,
				ContentHash: "bbb",
			},
		},
		depsWithEdge: map[string][]storage.DependencyRef{
			"in-progress": {
				{
					NodeRef:           storage.NodeRef{Slug: "upstream", Label: storage.NodeLabelSpec},
					ContentHashAtLink: "aaa",
				},
			},
		},
	}

	engine := drift.NewEngine(backend, nil)
	// Non-done/non-amended specs are not eligible for drift detection.
	_, err := engine.Check(context.Background(), "in-progress", "deps")
	require.Error(t, err)
	require.ErrorIs(t, err, storage.ErrSpecIneligibleForDrift)
}

func TestCheck_GetSpecError(t *testing.T) {
	backend := &mockDriftBackend{
		specs:   map[string]*storage.Spec{},
		specErr: errors.New("connection timeout"),
	}
	engine := drift.NewEngine(backend, nil)

	_, err := engine.Check(context.Background(), "my-spec", "")
	require.Error(t, err)
	require.ErrorContains(t, err, "connection timeout")
}

func TestCheckSpec_MissingDependencyCreatesInfoItem(t *testing.T) {
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"downstream": {
				Slug:        "downstream",
				Stage:       storage.SpecStageDone,
				ContentHash: "aaa",
			},
			// "gone-dep" intentionally absent
		},
		depsWithEdge: map[string][]storage.DependencyRef{
			"downstream": {
				{
					NodeRef:           storage.NodeRef{Slug: "gone-dep", Label: storage.NodeLabelSpec},
					ContentHashAtLink: "xxx",
				},
			},
		},
	}
	engine := drift.NewEngine(backend, nil)

	result, err := engine.Check(context.Background(), "downstream", "deps")
	require.NoError(t, err)
	require.Len(t, result.Reports, 1)
	require.Len(t, result.Reports[0].Items, 1)

	item := result.Reports[0].Items[0]
	require.Equal(t, storage.DriftTypeDependency, item.Type)
	require.Equal(t, storage.DriftSeverityMedium, item.Severity)
	require.Contains(t, item.Description, "gone-dep")
	require.Contains(t, item.Description, "not found")
	require.Equal(t, "downstream", item.SpecSlug)
	require.Equal(t, "gone-dep", item.UpstreamSlug)
}

func TestCheck_UpstreamGetSpecError(t *testing.T) {
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"downstream": {
				Slug:        "downstream",
				Stage:       storage.SpecStageDone,
				ContentHash: "aaa",
			},
			// upstream exists in the map so ListSpecs won't affect it,
			// but specErrForSlug overrides GetSpec for this slug.
			"upstream": {
				Slug:        "upstream",
				Stage:       storage.SpecStageDone,
				ContentHash: "bbb",
			},
		},
		depsWithEdge: map[string][]storage.DependencyRef{
			"downstream": {
				{
					NodeRef:           storage.NodeRef{Slug: "upstream", Label: storage.NodeLabelSpec},
					ContentHashAtLink: "aaa",
				},
			},
		},
		specErrForSlug: map[string]error{
			"upstream": errors.New("connection reset"),
		},
	}

	engine := drift.NewEngine(backend, nil)
	result, err := engine.Check(context.Background(), "downstream", "deps")

	// Top-level error should be nil (partial success).
	require.NoError(t, err)
	// The report for "downstream" should have a non-empty ErrorMessage
	// because checkSpec failed when fetching the upstream.
	require.Len(t, result.Reports, 1)
	require.Equal(t, "downstream", result.Reports[0].SpecSlug)
	require.NotEmpty(t, result.Reports[0].ErrorMessage, "expected ErrorMessage for mid-traversal failure")
	require.Equal(t, "drift check failed", result.Reports[0].ErrorMessage)
	require.Empty(t, result.Reports[0].Items)
}

func TestCheckDrift_AmendedSpecEligibleBySlug(t *testing.T) {
	backend := &mockDriftBackend{
		specs: map[string]*storage.Spec{
			"amended-spec": {
				Slug:        "amended-spec",
				Stage:       storage.SpecStageAmended,
				ContentHash: "aaa",
			},
			"upstream": {
				Slug:        "upstream",
				Stage:       storage.SpecStageDone,
				ContentHash: "bbb",
			},
		},
		depsWithEdge: map[string][]storage.DependencyRef{
			"amended-spec": {
				{
					NodeRef:           storage.NodeRef{Slug: "upstream", Label: storage.NodeLabelSpec},
					ContentHashAtLink: "aaa",
				},
			},
		},
	}

	engine := drift.NewEngine(backend, nil)
	result, err := engine.Check(context.Background(), "amended-spec", "")
	require.NoError(t, err)
	require.Len(t, result.Reports, 1)
	require.Equal(t, "amended-spec", result.Reports[0].SpecSlug)
	require.Len(t, result.Reports[0].Items, 1)
	require.Equal(t, storage.DriftTypeDependency, result.Reports[0].Items[0].Type)
}
