// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package prime_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/mcp/skills"
	"github.com/specgraph/specgraph/internal/prime"
	"github.com/specgraph/specgraph/internal/storage"
)

// stubSkills is a tiny skills.Source for tests. Only List is exercised.
type stubSkills struct {
	metas []skills.Meta
	err   error
}

func (s *stubSkills) List(_ context.Context) ([]skills.Meta, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.metas, nil
}

func (s *stubSkills) Get(_ context.Context, _ string) (skills.Skill, error) {
	return skills.Skill{}, errors.New("not implemented")
}

func (s *stubSkills) Search(_ context.Context, _ string, _ skills.SearchOptions) ([]skills.Meta, error) {
	return nil, errors.New("not implemented")
}

func newComposer(t *testing.T, b *prime.StubBackend, sk skills.Source) *prime.Composer {
	t.Helper()
	if sk == nil {
		sk = &stubSkills{}
	}
	return prime.New(b, sk)
}

func TestComposer_Project_EmptyConstitution(t *testing.T) {
	be := &prime.StubBackend{
		GetMergedConstitutionFn: func(context.Context) (*storage.MergedResult, error) {
			return nil, storage.ErrConstitutionNotFound
		},
	}
	view, err := newComposer(t, be, nil).Project(context.Background())
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Nil(t, view.Constitution)
	require.Empty(t, view.ConstitutionProvenance)
}

func TestComposer_Project_PopulatedConstitution(t *testing.T) {
	be := &prime.StubBackend{
		GetMergedConstitutionFn: func(context.Context) (*storage.MergedResult, error) {
			return &storage.MergedResult{
				Constitution: &storage.Constitution{Name: "Test", Version: 7},
				Provenance: map[string]storage.ConstitutionLayer{
					"process.spec_review":       storage.ConstitutionLayerProject,
					"tech.languages.primary":    storage.ConstitutionLayerOrg,
					"principles[p1].statement":  storage.ConstitutionLayerUser,
					"principles[p2].rationale":  storage.ConstitutionLayerProject,
				},
			}, nil
		},
	}
	view, err := newComposer(t, be, nil).Project(context.Background())
	require.NoError(t, err)
	require.NotNil(t, view.Constitution)
	require.Equal(t, "Test", view.Constitution.Name)
	require.Len(t, view.ConstitutionProvenance, 4)
	// Sorted lexicographically by Path for determinism.
	paths := make([]string, 0, len(view.ConstitutionProvenance))
	for _, p := range view.ConstitutionProvenance {
		paths = append(paths, p.Path)
	}
	require.Equal(t, []string{
		"principles[p1].statement",
		"principles[p2].rationale",
		"process.spec_review",
		"tech.languages.primary",
	}, paths)
}

func TestComposer_Project_GraphOverviewBuckets(t *testing.T) {
	be := &prime.StubBackend{
		ListSpecsFn: func(_ context.Context, _, _ string, _ int) ([]*storage.Spec, error) {
			return []*storage.Spec{
				{Slug: "a", Stage: storage.SpecStageSpark},
				{Slug: "b", Stage: storage.SpecStageSpark},
				{Slug: "c", Stage: storage.SpecStageShape},
			}, nil
		},
	}
	view, err := newComposer(t, be, nil).Project(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, view.GraphOverview.CountsByStage["spark"])
	require.Equal(t, 1, view.GraphOverview.CountsByStage["shape"])
}

func TestComposer_Project_ReadyCapsAtTen(t *testing.T) {
	refs := make([]storage.NodeRef, 0, 15)
	for i := 0; i < 15; i++ {
		refs = append(refs, storage.NodeRef{Slug: string(rune('a' + i)), Label: storage.NodeLabelSpec})
	}
	be := &prime.StubBackend{
		GetReadyFn: func(context.Context) ([]storage.NodeRef, error) { return refs, nil },
		GetSpecFn: func(_ context.Context, slug string) (*storage.Spec, error) {
			return &storage.Spec{Slug: slug, Stage: storage.SpecStageApproved}, nil
		},
	}
	view, err := newComposer(t, be, nil).Project(context.Background())
	require.NoError(t, err)
	require.Len(t, view.Ready, 10)
	require.Equal(t, "a", view.Ready[0].Slug)
	require.Equal(t, "j", view.Ready[9].Slug)
}

func TestComposer_Project_FindingsBucketed(t *testing.T) {
	be := &prime.StubBackend{
		ListAllFindingsFn: func(context.Context) ([]*storage.AnalyticalFinding, error) {
			return []*storage.AnalyticalFinding{
				{Severity: storage.SeverityCritical},
				{Severity: storage.SeverityCritical},
				{Severity: storage.SeverityNote},
			}, nil
		},
	}
	view, err := newComposer(t, be, nil).Project(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, view.FindingsBySeverity[storage.SeverityCritical])
	require.Equal(t, 1, view.FindingsBySeverity[storage.SeverityNote])
	require.Zero(t, view.FindingsBySeverity[storage.SeverityWarning])
}

func TestComposer_Project_SkillsCount(t *testing.T) {
	be := &prime.StubBackend{}
	sk := &stubSkills{metas: []skills.Meta{
		{Name: "one"}, {Name: "two"}, {Name: "three"},
	}}
	view, err := newComposer(t, be, sk).Project(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, view.SkillsCount)
}

func TestComposer_Project_BackendError_BubblesUp(t *testing.T) {
	boom := errors.New("backend exploded")
	be := &prime.StubBackend{
		ListSpecsFn: func(context.Context, string, string, int) ([]*storage.Spec, error) {
			return nil, boom
		},
	}
	_, err := newComposer(t, be, nil).Project(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, boom)
}

func TestComposer_Spec_NotFound(t *testing.T) {
	be := &prime.StubBackend{
		GetSpecFn: func(context.Context, string) (*storage.Spec, error) {
			return nil, storage.ErrSpecNotFound
		},
	}
	_, err := newComposer(t, be, nil).Spec(context.Background(), "missing")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestComposer_Spec_FullView(t *testing.T) {
	spec := &storage.Spec{Slug: "demo", Stage: storage.SpecStageInProgress, Intent: "do a thing"}

	be := &prime.StubBackend{
		GetSpecFn: func(_ context.Context, slug string) (*storage.Spec, error) {
			require.Equal(t, "demo", slug)
			return spec, nil
		},
		GetMergedConstitutionFn: func(context.Context) (*storage.MergedResult, error) {
			return &storage.MergedResult{
				Constitution: &storage.Constitution{Name: "C"},
				Provenance:   map[string]storage.ConstitutionLayer{"p": storage.ConstitutionLayerProject},
			}, nil
		},
		ListEdgesFn: func(_ context.Context, slug string, et storage.EdgeType) ([]*storage.Edge, error) {
			require.Equal(t, "demo", slug)
			require.Equal(t, storage.EdgeTypeDecidedIn, et)
			return []*storage.Edge{
				{FromID: "demo", ToID: "dec-1", EdgeType: storage.EdgeTypeDecidedIn},
				{FromID: "demo", ToID: "dec-2", EdgeType: storage.EdgeTypeDecidedIn},
				{FromID: "demo", ToID: "dec-gone", EdgeType: storage.EdgeTypeDecidedIn},
			}, nil
		},
		GetDecisionFn: func(_ context.Context, slug string) (*storage.Decision, error) {
			if slug == "dec-gone" {
				return nil, storage.ErrDecisionNotFound
			}
			return &storage.Decision{Slug: slug, Title: "Decision " + slug}, nil
		},
		ListSlicesFn: func(_ context.Context, parent string) ([]*storage.Slice, error) {
			require.Equal(t, "demo", parent)
			return []*storage.Slice{{Slug: "demo/s1"}, {Slug: "demo/s2"}}, nil
		},
		GetExecutionEventsFn: func(_ context.Context, slug string, _ int) ([]*storage.ExecutionEvent, error) {
			require.Equal(t, "demo", slug)
			return []*storage.ExecutionEvent{
				{ID: "e1", Type: storage.ExecutionEventTypeBlocker, Message: "stuck"},
				{ID: "e2", Type: storage.ExecutionEventTypeProgress, Message: "started"},
				{ID: "e3", Type: storage.ExecutionEventTypeBlocker, Message: "still stuck"},
				{ID: "e4", Type: storage.ExecutionEventTypeCompletion, Message: "done"},
			}, nil
		},
	}

	view, err := newComposer(t, be, nil).Spec(context.Background(), "demo")
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Equal(t, spec, view.Spec)
	require.NotNil(t, view.Constitution)
	require.Equal(t, "C", view.Constitution.Name)
	require.Len(t, view.ConstitutionProvenance, 1)

	// Decisions: dec-gone is filtered out (ErrDecisionNotFound), keep dec-1 and dec-2.
	require.Len(t, view.Decisions, 2)
	require.Equal(t, "dec-1", view.Decisions[0].Slug)
	require.Equal(t, "dec-2", view.Decisions[1].Slug)

	require.Len(t, view.Slices, 2)

	// Blockers: only the two blocker events; progress + completion filtered.
	require.Len(t, view.Blockers, 2)
	require.Equal(t, "e1", view.Blockers[0].ID)
	require.Equal(t, "e3", view.Blockers[1].ID)
	for _, b := range view.Blockers {
		require.Equal(t, storage.ExecutionEventTypeBlocker, b.Type)
	}
}

func TestComposer_Spec_EmptyConstitution(t *testing.T) {
	be := &prime.StubBackend{
		GetSpecFn: func(_ context.Context, slug string) (*storage.Spec, error) {
			return &storage.Spec{Slug: slug}, nil
		},
		GetMergedConstitutionFn: func(context.Context) (*storage.MergedResult, error) {
			return nil, storage.ErrConstitutionNotFound
		},
	}
	view, err := newComposer(t, be, nil).Spec(context.Background(), "demo")
	require.NoError(t, err)
	require.Nil(t, view.Constitution)
	require.Empty(t, view.ConstitutionProvenance)
}
