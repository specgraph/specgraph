// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordConversation_Basic(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "conv-basic", "test intent", "p2", "medium")
	require.NoError(t, err)

	entry := storage.ConversationLogEntry{
		Stage:   storage.SpecStageSpark,
		IsAmend: false,
		Exchanges: []storage.ConversationExchange{
			{Role: storage.ConversationRoleProbe, Content: "What is the seed idea?", Stage: "spark", Sequence: 1},
			{Role: storage.ConversationRoleResponse, Content: "Build a widget factory", Stage: "spark", Sequence: 1, DecisionPoint: true},
		},
		ExchangeCount: 2,
	}

	result, err := store.RecordConversation(ctx, "conv-basic", entry)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, storage.SpecStageSpark, result.Stage)
	assert.Equal(t, int32(1), result.Version)
	assert.Equal(t, int32(2), result.ExchangeCount)
	assert.False(t, result.IsAmend)
	assert.NotZero(t, result.Date)
	assert.Equal(t, "conv-basic", result.SpecSlug)

	// Verify exchanges round-trip.
	require.Len(t, result.Exchanges, 2)
	assert.Equal(t, storage.ConversationRoleProbe, result.Exchanges[0].Role)
	assert.Equal(t, "What is the seed idea?", result.Exchanges[0].Content)
	assert.False(t, result.Exchanges[0].DecisionPoint)
	assert.Equal(t, storage.ConversationRoleResponse, result.Exchanges[1].Role)
	assert.True(t, result.Exchanges[1].DecisionPoint)
}

func TestRecordConversation_ChainEdges(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "conv-chain", "test intent", "p2", "medium")
	require.NoError(t, err)

	// First log — should create AUTHORED_VIA edge.
	first, err := store.RecordConversation(ctx, "conv-chain", storage.ConversationLogEntry{
		Stage:         storage.SpecStageSpark,
		Exchanges:     []storage.ConversationExchange{{Role: storage.ConversationRoleProbe, Content: "seed?", Stage: "spark", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)
	require.NotEmpty(t, first.ID)

	// Second log — should create CONTINUES edge from first.
	second, err := store.RecordConversation(ctx, "conv-chain", storage.ConversationLogEntry{
		Stage:         storage.SpecStageSpark,
		IsAmend:       true,
		Exchanges:     []storage.ConversationExchange{{Role: storage.ConversationRoleProbe, Content: "more?", Stage: "spark", Sequence: 2}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)
	require.NotEmpty(t, second.ID)
	assert.NotEqual(t, first.ID, second.ID)

	// Verify both are returned in order.
	entries, err := store.ListConversations(ctx, "conv-chain", "")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, first.ID, entries[0].ID)
	assert.Equal(t, second.ID, entries[1].ID)
}

func TestRecordConversation_SpecNotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.RecordConversation(ctx, "nonexistent", storage.ConversationLogEntry{
		Stage:         storage.SpecStageSpark,
		Exchanges:     []storage.ConversationExchange{{Role: storage.ConversationRoleProbe, Content: "test", Stage: "spark", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestListConversations_FilterByStage(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Use a fixed clock so timestamps are distinct.
	t0 := time.Now().UTC()
	tick := 0
	clockStore := newStore(t, postgres.WithClock(func() time.Time {
		tick++
		return t0.Add(time.Duration(tick) * time.Millisecond)
	}))
	clearDatabase(t, clockStore)

	_, err := clockStore.CreateSpec(ctx, "conv-filter", "test", "p2", "medium")
	require.NoError(t, err)

	_, err = clockStore.RecordConversation(ctx, "conv-filter", storage.ConversationLogEntry{
		Stage:         storage.SpecStageSpark,
		Exchanges:     []storage.ConversationExchange{{Role: storage.ConversationRoleProbe, Content: "seed?", Stage: "spark", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	_, err = clockStore.RecordConversation(ctx, "conv-filter", storage.ConversationLogEntry{
		Stage:         storage.SpecStageShape,
		Exchanges:     []storage.ConversationExchange{{Role: storage.ConversationRoleProbe, Content: "scope?", Stage: "shape", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	// Filter by shape — only one result.
	entries, err := clockStore.ListConversations(ctx, "conv-filter", "shape")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, storage.SpecStageShape, entries[0].Stage)
}

func TestListConversations_OrderByDate(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	t0 := time.Now().UTC()
	tick := 0
	clockStore := newStore(t, postgres.WithClock(func() time.Time {
		tick++
		return t0.Add(time.Duration(tick) * time.Millisecond)
	}))
	clearDatabase(t, clockStore)

	_, err := clockStore.CreateSpec(ctx, "conv-order", "test", "p2", "medium")
	require.NoError(t, err)

	for i := range 3 {
		_, err = clockStore.RecordConversation(ctx, "conv-order", storage.ConversationLogEntry{
			Stage:         storage.SpecStageSpark,
			Exchanges:     []storage.ConversationExchange{{Role: storage.ConversationRoleProbe, Content: "msg", Stage: "spark", Sequence: int32(i + 1)}},
			ExchangeCount: 1,
		})
		require.NoError(t, err)
	}

	entries, err := clockStore.ListConversations(ctx, "conv-order", "")
	require.NoError(t, err)
	require.Len(t, entries, 3)

	// Verify ascending date order.
	assert.True(t, entries[0].Date.Before(entries[1].Date) || entries[0].Date.Equal(entries[1].Date))
	assert.True(t, entries[1].Date.Before(entries[2].Date) || entries[1].Date.Equal(entries[2].Date))
}

func TestListConversations_SpecNotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.ListConversations(ctx, "nonexistent", "")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestListConversations_EmptySlice(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "conv-empty", "test", "p2", "medium")
	require.NoError(t, err)

	entries, err := store.ListConversations(ctx, "conv-empty", "")
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestListAllConversations(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "conv-all-a", "intent a", "p1", "low")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "conv-all-b", "intent b", "p2", "medium")
	require.NoError(t, err)

	_, err = store.RecordConversation(ctx, "conv-all-a", storage.ConversationLogEntry{
		Stage:         storage.SpecStageSpark,
		Exchanges:     []storage.ConversationExchange{{Role: storage.ConversationRoleProbe, Content: "a1", Stage: "spark", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	_, err = store.RecordConversation(ctx, "conv-all-b", storage.ConversationLogEntry{
		Stage:         storage.SpecStageSpark,
		Exchanges:     []storage.ConversationExchange{{Role: storage.ConversationRoleProbe, Content: "b1", Stage: "spark", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	entries, err := store.ListAllConversations(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	// All entries should have SpecSlug populated.
	for _, e := range entries {
		assert.NotEmpty(t, e.SpecSlug)
		assert.NotEmpty(t, e.ID)
	}

	// Ordered by spec_slug.
	assert.Equal(t, "conv-all-a", entries[0].SpecSlug)
	assert.Equal(t, "conv-all-b", entries[1].SpecSlug)
}

func TestGetSpec_IncludesConversationLogs(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "conv-getspec", "test intent", "p2", "medium")
	require.NoError(t, err)

	_, err = store.RecordConversation(ctx, "conv-getspec", storage.ConversationLogEntry{
		Stage: storage.SpecStageSpark,
		Exchanges: []storage.ConversationExchange{
			{Role: storage.ConversationRoleProbe, Content: "seed?", Stage: "spark", Sequence: 1},
		},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	spec, err := store.GetSpec(ctx, "conv-getspec")
	require.NoError(t, err)
	require.Len(t, spec.ConversationLogs, 1)
	assert.Equal(t, storage.SpecStageSpark, spec.ConversationLogs[0].Stage)
	assert.Len(t, spec.ConversationLogs[0].Exchanges, 1)
}
