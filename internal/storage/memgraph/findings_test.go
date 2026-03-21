// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestStoreFindings_CreatesNodesAndEdges(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "findings-create", "Test findings creation", "p1", "medium")
	require.NoError(t, err)

	findings := []storage.AnalyticalFinding{
		{
			PassType:   storage.PassTypeConstitutionCheck,
			Severity:   storage.SeverityWarning,
			Summary:    "Missing constraint coverage",
			Detail:     "The spec does not address constraint X",
			Constraint: "constitution.layer.constraint-x",
			Resolution: "Add a section covering constraint X",
		},
		{
			PassType:   storage.PassTypeConstitutionCheck,
			Severity:   storage.SeverityCritical,
			Summary:    "Violates naming convention",
			Detail:     "Slug uses underscores instead of hyphens",
			Constraint: "constitution.layer.naming",
			Resolution: "Rename to use hyphens",
		},
	}

	_, err = store.StoreFindings(ctx, "findings-create", storage.PassTypeConstitutionCheck, findings)
	require.NoError(t, err)

	got, err := store.ListFindings(ctx, "findings-create", storage.PassTypeConstitutionCheck)
	require.NoError(t, err)
	require.Len(t, got, 2)

	for _, f := range got {
		require.NotEmpty(t, f.ID, "ID should be generated")
		require.Equal(t, storage.PassTypeConstitutionCheck, f.PassType)
		require.False(t, f.CreatedAt.IsZero(), "CreatedAt should be set")
	}

	// Verify fields of the findings (order may vary, so find by summary).
	bySummary := make(map[string]storage.AnalyticalFinding, len(got))
	for _, f := range got {
		bySummary[f.Summary] = f
	}

	f1 := bySummary["Missing constraint coverage"]
	require.Equal(t, storage.SeverityWarning, f1.Severity)
	require.Equal(t, "The spec does not address constraint X", f1.Detail)
	require.Equal(t, "constitution.layer.constraint-x", f1.Constraint)
	require.Equal(t, "Add a section covering constraint X", f1.Resolution)

	f2 := bySummary["Violates naming convention"]
	require.Equal(t, storage.SeverityCritical, f2.Severity)
	require.Equal(t, "Slug uses underscores instead of hyphens", f2.Detail)
	require.Equal(t, "constitution.layer.naming", f2.Constraint)
	require.Equal(t, "Rename to use hyphens", f2.Resolution)
}

func TestStoreFindings_ReplacesExistingFindings(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "findings-replace", "Test findings replacement", "p1", "medium")
	require.NoError(t, err)

	initial := []storage.AnalyticalFinding{
		{
			PassType: storage.PassTypeRedTeam,
			Severity: storage.SeverityWarning,
			Summary:  "Old finding A",
		},
		{
			PassType: storage.PassTypeRedTeam,
			Severity: storage.SeverityNote,
			Summary:  "Old finding B",
		},
	}

	_, err = store.StoreFindings(ctx, "findings-replace", storage.PassTypeRedTeam, initial)
	require.NoError(t, err)

	// Replace with a single new finding.
	replacement := []storage.AnalyticalFinding{
		{
			PassType: storage.PassTypeRedTeam,
			Severity: storage.SeverityCritical,
			Summary:  "New finding C",
		},
	}

	_, err = store.StoreFindings(ctx, "findings-replace", storage.PassTypeRedTeam, replacement)
	require.NoError(t, err)

	got, err := store.ListFindings(ctx, "findings-replace", storage.PassTypeRedTeam)
	require.NoError(t, err)
	require.Len(t, got, 1, "should have replaced old findings")
	require.Equal(t, "New finding C", got[0].Summary)
	require.Equal(t, storage.SeverityCritical, got[0].Severity)
}

func TestStoreFindings_DifferentPassTypesAreIndependent(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "findings-types", "Test pass type independence", "p1", "medium")
	require.NoError(t, err)

	constFindings := []storage.AnalyticalFinding{
		{
			PassType: storage.PassTypeConstitutionCheck,
			Severity: storage.SeverityWarning,
			Summary:  "Constitution finding",
		},
	}
	_, err = store.StoreFindings(ctx, "findings-types", storage.PassTypeConstitutionCheck, constFindings)
	require.NoError(t, err)

	redTeamFindings := []storage.AnalyticalFinding{
		{
			PassType: storage.PassTypeRedTeam,
			Severity: storage.SeverityCritical,
			Summary:  "Red team finding 1",
		},
		{
			PassType: storage.PassTypeRedTeam,
			Severity: storage.SeverityNote,
			Summary:  "Red team finding 2",
		},
	}
	_, err = store.StoreFindings(ctx, "findings-types", storage.PassTypeRedTeam, redTeamFindings)
	require.NoError(t, err)

	// List by constitution_check — should get only 1.
	constGot, err := store.ListFindings(ctx, "findings-types", storage.PassTypeConstitutionCheck)
	require.NoError(t, err)
	require.Len(t, constGot, 1)
	require.Equal(t, "Constitution finding", constGot[0].Summary)

	// List by red_team — should get only 2.
	redGot, err := store.ListFindings(ctx, "findings-types", storage.PassTypeRedTeam)
	require.NoError(t, err)
	require.Len(t, redGot, 2)

	// List with empty pass type — should get all 3.
	allGot, err := store.ListFindings(ctx, "findings-types", "")
	require.NoError(t, err)
	require.Len(t, allGot, 3)
}

func TestStoreFindings_RecordsSpecVersion(t *testing.T) {
	store, ctx := newTestStore(t)

	spec, err := store.CreateSpec(ctx, "findings-version", "Test version recording", "p1", "medium")
	require.NoError(t, err)
	require.Equal(t, int32(1), spec.Version)

	findings := []storage.AnalyticalFinding{
		{
			PassType: storage.PassTypeConstitutionCheck,
			Severity: storage.SeverityNote,
			Summary:  "Version check finding",
		},
	}

	_, err = store.StoreFindings(ctx, "findings-version", storage.PassTypeConstitutionCheck, findings)
	require.NoError(t, err)

	got, err := store.ListFindings(ctx, "findings-version", storage.PassTypeConstitutionCheck)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, spec.Version, got[0].Version, "finding Version should match spec version")
}

func TestStoreFindings_SpecNotFound(t *testing.T) {
	store, ctx := newTestStore(t)

	findings := []storage.AnalyticalFinding{
		{
			PassType: storage.PassTypeRedTeam,
			Severity: storage.SeverityWarning,
			Summary:  "Should fail",
		},
	}

	_, err := store.StoreFindings(ctx, "nonexistent-spec", storage.PassTypeRedTeam, findings)
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestListFindings_EmptyResult(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "findings-empty", "Spec with no findings", "p1", "medium")
	require.NoError(t, err)

	got, err := store.ListFindings(ctx, "findings-empty", storage.PassTypeConstitutionCheck)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestStoreFindings_EmptySliceDeletesExisting(t *testing.T) {
	store, ctx := newTestStore(t)

	_, err := store.CreateSpec(ctx, "findings-delete", "Test empty slice deletion", "p1", "medium")
	require.NoError(t, err)

	// Store initial findings.
	initial := []storage.AnalyticalFinding{
		{
			PassType: storage.PassTypeConstitutionCheck,
			Severity: storage.SeverityWarning,
			Summary:  "Will be deleted",
		},
		{
			PassType: storage.PassTypeConstitutionCheck,
			Severity: storage.SeverityCritical,
			Summary:  "Also deleted",
		},
	}

	_, err = store.StoreFindings(ctx, "findings-delete", storage.PassTypeConstitutionCheck, initial)
	require.NoError(t, err)

	// Verify they exist.
	got, err := store.ListFindings(ctx, "findings-delete", storage.PassTypeConstitutionCheck)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Store empty slice — should delete existing findings for this pass type.
	_, err = store.StoreFindings(ctx, "findings-delete", storage.PassTypeConstitutionCheck, nil)
	require.NoError(t, err)

	// Verify findings are gone.
	got, err = store.ListFindings(ctx, "findings-delete", storage.PassTypeConstitutionCheck)
	require.NoError(t, err)
	require.Empty(t, got, "empty slice should delete existing findings")
}
