// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestStoreFindings_CreatesAndReplacesExisting(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "findings-spec", "Intent", "", "")
	require.NoError(t, err)

	// Store initial findings.
	ids1, err := store.StoreFindings(ctx, "findings-spec", storage.PassTypeConstitutionCheck, []storage.AnalyticalFindingInput{
		{Severity: "error", Summary: "Finding A", Detail: "detail A", Constraint: "rule-1", Resolution: "fix it"},
		{Severity: "warning", Summary: "Finding B"},
	})
	require.NoError(t, err)
	require.Len(t, ids1, 2)

	list1, err := store.ListFindings(ctx, "findings-spec", storage.PassTypeConstitutionCheck)
	require.NoError(t, err)
	require.Len(t, list1, 2)

	// Replace with a single new finding.
	ids2, err := store.StoreFindings(ctx, "findings-spec", storage.PassTypeConstitutionCheck, []storage.AnalyticalFindingInput{
		{Severity: "info", Summary: "Finding C"},
	})
	require.NoError(t, err)
	require.Len(t, ids2, 1)

	list2, err := store.ListFindings(ctx, "findings-spec", storage.PassTypeConstitutionCheck)
	require.NoError(t, err)
	require.Len(t, list2, 1)
	require.Equal(t, "Finding C", list2[0].Summary)

	// Old IDs are gone.
	for _, oldID := range ids1 {
		var found bool
		for _, f := range list2 {
			if f.ID == oldID {
				found = true
			}
		}
		require.False(t, found, "old finding ID %s should be replaced", oldID)
	}
}

func TestListFindings_FilterByPassType(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "filter-spec", "Intent", "", "")
	require.NoError(t, err)

	_, err = store.StoreFindings(ctx, "filter-spec", storage.PassTypeConstitutionCheck, []storage.AnalyticalFindingInput{
		{Severity: "error", Summary: "Constitution issue"},
	})
	require.NoError(t, err)

	_, err = store.StoreFindings(ctx, "filter-spec", storage.PassTypeRedTeam, []storage.AnalyticalFindingInput{
		{Severity: "warning", Summary: "Red team issue"},
	})
	require.NoError(t, err)

	// Filter by constitution_check.
	list, err := store.ListFindings(ctx, "filter-spec", storage.PassTypeConstitutionCheck)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "Constitution issue", list[0].Summary)

	// No filter returns all.
	all, err := store.ListFindings(ctx, "filter-spec", "")
	require.NoError(t, err)
	require.Len(t, all, 2)
}

func TestListFindings_SpecNotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.ListFindings(ctx, "no-such-spec", "")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestListAllFindings(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "all-spec-a", "A", "", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "all-spec-b", "B", "", "")
	require.NoError(t, err)

	_, err = store.StoreFindings(ctx, "all-spec-a", storage.PassTypeConstitutionCheck, []storage.AnalyticalFindingInput{
		{Severity: "error", Summary: "A finding"},
	})
	require.NoError(t, err)

	_, err = store.StoreFindings(ctx, "all-spec-b", storage.PassTypeRedTeam, []storage.AnalyticalFindingInput{
		{Severity: "warning", Summary: "B finding"},
	})
	require.NoError(t, err)

	all, err := store.ListAllFindings(ctx)
	require.NoError(t, err)
	require.Len(t, all, 2)

	specSlugs := make(map[string]bool)
	for _, f := range all {
		specSlugs[f.SpecSlug] = true
		require.NotEmpty(t, f.ID)
		require.NotEmpty(t, f.Summary)
	}
	require.Contains(t, specSlugs, "all-spec-a")
	require.Contains(t, specSlugs, "all-spec-b")
}
