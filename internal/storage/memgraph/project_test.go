//go:build integration

// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph_test

import (
	"context"
	"testing"

	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureProject_CreatesNew(t *testing.T) {
	ctx := context.Background()
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	p, err := store.EnsureProject(ctx, "test-project")
	require.NoError(t, err)
	assert.Equal(t, "test-project", p.Slug)
	assert.False(t, p.CreatedAt.IsZero())
}

func TestEnsureProject_Idempotent(t *testing.T) {
	ctx := context.Background()
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	p1, err := store.EnsureProject(ctx, "idem-project")
	require.NoError(t, err)
	p2, err := store.EnsureProject(ctx, "idem-project")
	require.NoError(t, err)
	assert.Equal(t, p1.CreatedAt, p2.CreatedAt)
}

func TestGetProject_NotFound(t *testing.T) {
	ctx := context.Background()
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.GetProject(ctx, "nonexistent")
	assert.ErrorIs(t, err, storage.ErrProjectNotFound)
}

func TestUpdateProject_SyncAdapters(t *testing.T) {
	ctx := context.Background()
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.EnsureProject(ctx, "updatable")
	require.NoError(t, err)
	p, err := store.UpdateProject(ctx, "updatable", []string{"beads", "github"}, "owner/repo")
	require.NoError(t, err)
	assert.Equal(t, []string{"beads", "github"}, p.SyncAdapters)
	assert.Equal(t, "owner/repo", p.GitHubRepo)
}

func TestListProjects(t *testing.T) {
	ctx := context.Background()
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.EnsureProject(ctx, "proj-a")
	require.NoError(t, err)
	_, err = store.EnsureProject(ctx, "proj-b")
	require.NoError(t, err)

	projects, err := store.ListProjects(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(projects), 2)
}
