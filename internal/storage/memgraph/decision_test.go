// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestCreateAndGetDecision(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	d, err := store.CreateDecision(ctx, "use-memgraph", "Use Memgraph", "Use Memgraph as primary DB", "Native Cypher support",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)
	require.NotNil(t, d)
	require.Contains(t, d.ID, "dec-")
	require.Equal(t, "use-memgraph", d.Slug)
	require.Equal(t, "Use Memgraph", d.Title)
	require.Equal(t, storage.DecisionStatusProposed, d.Status)
	require.Equal(t, "Use Memgraph as primary DB", d.Body)
	require.Equal(t, "Native Cypher support", d.Rationale)
	require.NotNil(t, d.CreatedAt)

	got, err := store.GetDecision(ctx, "use-memgraph")
	require.NoError(t, err)
	require.Equal(t, d.ID, got.ID)
	require.Equal(t, d.Slug, got.Slug)
}

func TestListDecisions(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateDecision(ctx, "dec-a", "First", "Decision A", "Reason A",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)
	_, err = store.CreateDecision(ctx, "dec-b", "Second", "Decision B", "Reason B",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)

	all, err := store.ListDecisions(ctx, "", 0)
	require.NoError(t, err)
	require.Len(t, all, 2)

	filtered, err := store.ListDecisions(ctx, storage.DecisionStatusProposed, 0)
	require.NoError(t, err)
	require.Len(t, filtered, 2)
}

func TestUpdateDecision(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateDecision(ctx, "update-dec", "Original Title", "Original decision", "Original rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)

	newStatus := storage.DecisionStatusAccepted
	updated, err := store.UpdateDecision(ctx, "update-dec", 0, nil, &newStatus, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, storage.DecisionStatusAccepted, updated.Status)
	require.Equal(t, "Original Title", updated.Title)

	_, err = store.UpdateDecision(ctx, "nonexistent", 0, nil, &newStatus, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil)
	require.Error(t, err)
}

func TestCreateDecision_SetsContentHash(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	dec, err := store.CreateDecision(ctx, "hash-test-dec", "Test Decision", "We decided this", "Because reasons",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)
	require.Len(t, dec.ContentHash, 32, "content_hash should be 32-char hex")

	// Update and verify hash changes
	newTitle := "Updated Decision Title"
	updated, err := store.UpdateDecision(ctx, "hash-test-dec", 0, &newTitle, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, updated.ContentHash, 32)
	require.NotEqual(t, dec.ContentHash, updated.ContentHash, "hash should change when title changes")

	// Get and verify hash is persisted
	fetched, err := store.GetDecision(ctx, "hash-test-dec")
	require.NoError(t, err)
	require.Equal(t, updated.ContentHash, fetched.ContentHash)
}

func TestGetDecision_NotFound(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.GetDecision(ctx, "nonexistent")
	require.Error(t, err)
}

func TestCreateDecision_WithADR003Fields(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	dec, err := store.CreateDecision(ctx, "adr003-test", "Token Storage", "Use Postgres", "Because reasons",
		"Where to store tokens?",
		[]storage.RejectedAlternative{{Option: "Redis", Reason: "ops"}},
		storage.DecisionConfidenceHigh,
		[]string{"auth", "storage"},
		storage.DecisionScopeProject,
		"login-api", "specify")
	require.NoError(t, err)
	require.Equal(t, "Where to store tokens?", dec.Question)
	require.Len(t, dec.RejectedAlternatives, 1)
	require.Equal(t, "Redis", dec.RejectedAlternatives[0].Option)
	require.Equal(t, "ops", dec.RejectedAlternatives[0].Reason)
	require.Equal(t, storage.DecisionConfidenceHigh, dec.Confidence)
	require.Equal(t, []string{"auth", "storage"}, dec.Tags)
	require.Equal(t, storage.DecisionScopeProject, dec.Scope)
	require.Equal(t, "login-api", dec.OriginSpec)
	require.Equal(t, "specify", dec.OriginStage)
	require.Equal(t, 1, dec.Version)
}

func TestUpdateDecision_ADR003Fields(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	dec, err := store.CreateDecision(ctx, "update-adr003", "Original", "Body", "Rationale",
		"Original question?",
		nil, "", nil, "", "", "")
	require.NoError(t, err)
	origHash := dec.ContentHash

	newQuestion := "Updated question?"
	newConfidence := storage.DecisionConfidenceMedium
	newTags := []string{"updated", "tags"}
	updated, err := store.UpdateDecision(ctx, "update-adr003", 0,
		nil, nil, nil, nil, nil, &newQuestion,
		nil, &newConfidence, &newTags, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "Updated question?", updated.Question)
	require.Equal(t, storage.DecisionConfidenceMedium, updated.Confidence)
	require.Equal(t, []string{"updated", "tags"}, updated.Tags)
	require.Equal(t, 2, updated.Version)
	require.NotEqual(t, origHash, updated.ContentHash, "content hash should change")
}

func TestDecision_BackwardCompat(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	dec, err := store.CreateDecision(ctx, "compat-test", "Compat", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)
	require.Empty(t, dec.Question)
	require.Empty(t, dec.RejectedAlternatives)
	require.Empty(t, string(dec.Confidence))
	require.Empty(t, dec.Tags)
	require.Empty(t, string(dec.Scope))
	require.Empty(t, dec.OriginSpec)
	require.Empty(t, dec.OriginStage)

	got, err := store.GetDecision(ctx, "compat-test")
	require.NoError(t, err)
	require.Empty(t, got.Question)
	require.Empty(t, got.RejectedAlternatives)
	require.Empty(t, string(got.Confidence))
	require.Empty(t, got.Tags)
	require.Empty(t, string(got.Scope))
	require.Empty(t, got.OriginSpec)
	require.Empty(t, got.OriginStage)
}

func TestUpdateDecision_VersionGuard(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	dec, err := store.CreateDecision(ctx, "ver-guard", "Title", "Body", "Rationale",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)
	require.Equal(t, 1, dec.Version)

	// Update with correct version succeeds.
	newTitle := "Updated"
	updated, err := store.UpdateDecision(ctx, "ver-guard", 1, &newTitle, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "Updated", updated.Title)
	require.Equal(t, 2, updated.Version)

	// Update with stale version fails with ErrConcurrentModification.
	anotherTitle := "Stale"
	_, err = store.UpdateDecision(ctx, "ver-guard", 1, &anotherTitle, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil)
	require.ErrorIs(t, err, storage.ErrConcurrentModification)

	// Update with version=0 skips the check and succeeds.
	skipTitle := "NoCheck"
	noCheck, err := store.UpdateDecision(ctx, "ver-guard", 0, &skipTitle, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "NoCheck", noCheck.Title)
	require.Equal(t, 3, noCheck.Version)
}
