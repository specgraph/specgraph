// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupMemgraph(t *testing.T) (string, func()) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "memgraph/memgraph-platform:2.4.0",
		ExposedPorts: []string{"7687/tcp"},
		Env:          map[string]string{"MEMGRAPH": ""},
		Cmd:          []string{"/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("7687/tcp"),
			wait.ForLog("memgraph entered RUNNING state"),
		).WithDeadline(120 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "7687")
	require.NoError(t, err)

	boltURI := fmt.Sprintf("bolt://%s:%s", host, port.Port())

	cleanup := func() {
		_ = container.Terminate(ctx)
	}

	return boltURI, cleanup
}

// newStore creates a Store with retry logic to handle the brief window between
// the container wait strategy completing and the bolt protocol being fully ready.
func newStore(ctx context.Context, boltURI string) (*memgraph.Store, error) {
	var (
		store *memgraph.Store
		err   error
	)
	for range 10 {
		store, err = memgraph.New(ctx, boltURI)
		if err == nil {
			return store, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, err
}

func TestCreateAndGetSpec(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	spec, err := store.CreateSpec(ctx, "login-api", "Implement login API", "p1", "medium")
	require.NoError(t, err)
	require.NotNil(t, spec)
	require.Contains(t, spec.ID, "spec-")
	require.Len(t, spec.ID, 31) // "spec-" + 26 ULID chars
	require.Equal(t, "login-api", spec.Slug)
	require.Equal(t, "Implement login API", spec.Intent)
	require.Equal(t, storage.SpecStageSpark, spec.Stage)
	require.Equal(t, storage.SpecPriorityP1, spec.Priority)
	require.Equal(t, "medium", spec.Complexity)
	require.Equal(t, int32(1), spec.Version)
	require.NotNil(t, spec.CreatedAt)
	require.NotNil(t, spec.UpdatedAt)

	got, err := store.GetSpec(ctx, "login-api")
	require.NoError(t, err)
	require.Equal(t, spec.ID, got.ID)
	require.Equal(t, spec.Slug, got.Slug)
	require.Equal(t, spec.Intent, got.Intent)
	require.Equal(t, spec.Stage, got.Stage)
	require.Equal(t, spec.Priority, got.Priority)
	require.Equal(t, spec.Complexity, got.Complexity)
	require.Equal(t, spec.Version, got.Version)
	require.Equal(t, spec.CreatedAt.Unix(), got.CreatedAt.Unix())
	require.Equal(t, spec.UpdatedAt.Unix(), got.UpdatedAt.Unix())
}

func TestListSpecs(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "spec-a", "First spec", "p1", "low")
	require.NoError(t, err)

	_, err = store.CreateSpec(ctx, "spec-b", "Second spec", "p2", "high")
	require.NoError(t, err)

	// List all specs
	all, err := store.ListSpecs(ctx, "", "", 0)
	require.NoError(t, err)
	require.Len(t, all, 2)

	// List by priority p1
	filtered, err := store.ListSpecs(ctx, "", "p1", 0)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	require.Equal(t, "spec-a", filtered[0].Slug)
}

func TestUpdateSpec(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create a spec first.
	orig, err := store.CreateSpec(ctx, "update-me", "Original intent", "p2", "medium")
	require.NoError(t, err)
	require.Equal(t, int32(1), orig.Version)

	// Update intent only.
	newIntent := "Updated intent"
	updated, err := store.UpdateSpec(ctx, "update-me", &newIntent, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "Updated intent", updated.Intent)
	require.Equal(t, int32(2), updated.Version)
	require.Equal(t, storage.SpecPriorityP2, updated.Priority) // unchanged
	require.Equal(t, storage.SpecStageSpark, updated.Stage)    // unchanged

	// Update multiple fields.
	newStage := "shape"
	newPriority := "p0"
	updated2, err := store.UpdateSpec(ctx, "update-me", nil, &newStage, &newPriority, nil)
	require.NoError(t, err)
	require.Equal(t, storage.SpecStageShape, updated2.Stage)
	require.Equal(t, storage.SpecPriorityP0, updated2.Priority)
	require.Equal(t, int32(3), updated2.Version)
	require.Equal(t, "Updated intent", updated2.Intent) // still from previous update

	// Update non-existent spec.
	_, err = store.UpdateSpec(ctx, "no-such-spec", &newIntent, nil, nil, nil)
	require.Error(t, err)
}

func TestGetSpec_NotFound(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.GetSpec(ctx, "nonexistent")
	require.Error(t, err)
}
