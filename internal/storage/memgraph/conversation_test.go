// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordConversation_CreatesNode(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create a spec first (this also creates a ChangeLog entry).
	_, err = store.CreateSpec(ctx, "test-conv", "test intent", "p2", "medium")
	require.NoError(t, err)

	entry := storage.ConversationLogEntry{
		Stage:   storage.SpecStageSpark,
		IsAmend: false,
		Exchanges: []storage.ConversationExchange{
			{Role: "probe", Content: "What is the seed idea?", Stage: "spark", Sequence: 1},
			{Role: "response", Content: "Build a widget factory", Stage: "spark", Sequence: 1, DecisionPoint: true},
		},
		ExchangeCount: 2,
	}

	result, err := store.RecordConversation(ctx, "test-conv", entry)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, storage.SpecStageSpark, result.Stage)
	assert.Equal(t, int32(1), result.Version)
	assert.Equal(t, int32(2), result.ExchangeCount)
	assert.False(t, result.IsAmend)
	assert.NotZero(t, result.Date)
}

func TestListConversations_ReturnsChainOrder(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-conv-list", "test intent", "p2", "medium")
	require.NoError(t, err)

	// Record spark conversation.
	_, err = store.RecordConversation(ctx, "test-conv-list", storage.ConversationLogEntry{
		Stage:         storage.SpecStageSpark,
		Exchanges:     []storage.ConversationExchange{{Role: "probe", Content: "seed?", Stage: "spark", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	// Transition to shape and record shape conversation.
	err = store.TransitionStage(ctx, "test-conv-list", "spark", "shape")
	require.NoError(t, err)

	_, err = store.RecordConversation(ctx, "test-conv-list", storage.ConversationLogEntry{
		Stage:         storage.SpecStageShape,
		Exchanges:     []storage.ConversationExchange{{Role: "probe", Content: "scope?", Stage: "shape", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	entries, err := store.ListConversations(ctx, "test-conv-list", "")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, storage.SpecStageSpark, entries[0].Stage)
	assert.Equal(t, storage.SpecStageShape, entries[1].Stage)
}

func TestRecordConversation_NonexistentSpec(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.RecordConversation(ctx, "nonexistent", storage.ConversationLogEntry{
		Stage:         storage.SpecStageSpark,
		Exchanges:     []storage.ConversationExchange{{Role: "probe", Content: "test", Stage: "spark", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestListConversations_NonexistentSpec(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.ListConversations(ctx, "nonexistent", "")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestListConversations_StageFilter(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-conv-filter", "test", "p2", "medium")
	require.NoError(t, err)

	_, err = store.RecordConversation(ctx, "test-conv-filter", storage.ConversationLogEntry{
		Stage:         storage.SpecStageSpark,
		Exchanges:     []storage.ConversationExchange{{Role: "probe", Content: "seed?", Stage: "spark", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	err = store.TransitionStage(ctx, "test-conv-filter", "spark", "shape")
	require.NoError(t, err)

	_, err = store.RecordConversation(ctx, "test-conv-filter", storage.ConversationLogEntry{
		Stage:         storage.SpecStageShape,
		Exchanges:     []storage.ConversationExchange{{Role: "probe", Content: "scope?", Stage: "shape", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	entries, err := store.ListConversations(ctx, "test-conv-filter", "shape")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, storage.SpecStageShape, entries[0].Stage)
}

func TestListConversations_EmptyReturnsNil(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-conv-empty", "test", "p2", "medium")
	require.NoError(t, err)

	entries, err := store.ListConversations(ctx, "test-conv-empty", "")
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestRecordConversation_CreatesExplainsEdge(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-conv-explains", "test", "p2", "medium")
	require.NoError(t, err)

	convResult, err := store.RecordConversation(ctx, "test-conv-explains", storage.ConversationLogEntry{
		Stage:         storage.SpecStageSpark,
		Exchanges:     []storage.ConversationExchange{{Role: "probe", Content: "seed?", Stage: "spark", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	// Verify EXPLAINS edge exists via ListChanges + conversation version correlation.
	// The EXPLAINS edge should point to the spark checkpoint ChangeLog.
	changes, err := store.ListChanges(ctx, "test-conv-explains", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.NotEmpty(t, changes)

	// The ConversationLog version should match a ChangeLog's version.
	assert.Equal(t, changes[0].Version, convResult.Version,
		"ConversationLog version should match the ChangeLog it explains")
}

func TestGetSpec_IncludesConversationLogs(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-getspec-conv", "test intent", "p2", "medium")
	require.NoError(t, err)

	_, err = store.RecordConversation(ctx, "test-getspec-conv", storage.ConversationLogEntry{
		Stage:         storage.SpecStageSpark,
		Exchanges:     []storage.ConversationExchange{{Role: "probe", Content: "seed?", Stage: "spark", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	spec, err := store.GetSpec(ctx, "test-getspec-conv")
	require.NoError(t, err)
	require.Len(t, spec.ConversationLogs, 1)
	assert.Equal(t, storage.SpecStageSpark, spec.ConversationLogs[0].Stage)
	assert.Len(t, spec.ConversationLogs[0].Exchanges, 1)
}
