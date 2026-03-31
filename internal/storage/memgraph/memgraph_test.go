// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/memgraph"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// boltURI is set once by TestMain and shared across all tests.
var boltURI string

func TestMain(m *testing.M) {
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
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start memgraph container: %v\n", err)
		os.Exit(1)
	}

	host, err := container.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get container host: %v\n", err)
		_ = container.Terminate(ctx)
		os.Exit(1)
	}

	port, err := container.MappedPort(ctx, "7687")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get mapped port: %v\n", err)
		_ = container.Terminate(ctx)
		os.Exit(1)
	}

	boltURI = fmt.Sprintf("bolt://%s:%s", host, port.Port())

	code := m.Run()

	_ = container.Terminate(ctx)
	os.Exit(code)
}

// clearDatabase removes all nodes and edges from the shared Memgraph container.
// Call at the start of each test to ensure isolation. Uses a retry loop and
// neo4j.ExecuteQuery with EagerResultTransformer to avoid CI flakes from
// transient Bolt connection issues.
func clearDatabase(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	var lastErr error
	for range 10 {
		driver, err := neo4j.NewDriver(boltURI, neo4j.NoAuth())
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		_, err = neo4j.ExecuteQuery(ctx, driver,
			"MATCH (n) DETACH DELETE n", nil,
			neo4j.EagerResultTransformer)
		closeErr := driver.Close(ctx)
		if err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		require.NoError(t, closeErr)
		return
	}
	require.NoError(t, lastErr, "clearDatabase: retry exhausted")
}

// newStore creates a Store with retry logic. Uses the shared boltURI.
func newStore(ctx context.Context, boltURI string, opts ...memgraph.Option) (*memgraph.Store, error) {
	opts = append([]memgraph.Option{memgraph.WithProject("test")}, opts...)
	var (
		store *memgraph.Store
		err   error
	)
	for range 10 {
		store, err = memgraph.New(ctx, boltURI, opts...)
		if err == nil {
			return store, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, err
}

func TestCreateAndGetSpec(t *testing.T) {
	clearDatabase(t)

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
	require.Equal(t, storage.SpecComplexityMedium, spec.Complexity)
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

func TestCreateSpec_DuplicateSlugReturnsError(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "login-api", "Implement login API", "medium", "medium")
	require.NoError(t, err)

	_, err = store.CreateSpec(ctx, "login-api", "Duplicate login API", "medium", "medium")
	require.ErrorIs(t, err, storage.ErrSpecAlreadyExists)
}

func TestCreateSpec_SameSlugDifferentProjectsSucceeds(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()

	// Create spec in project "test" (default).
	store1, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store1.Close(ctx)

	_, err = store1.CreateSpec(ctx, "shared-slug", "Intent A", "p1", "medium")
	require.NoError(t, err)

	// Create spec with same slug in project "other".
	store2, err := newStore(ctx, boltURI, memgraph.WithProject("other"))
	require.NoError(t, err)
	defer store2.Close(ctx)

	_, err = store2.CreateSpec(ctx, "shared-slug", "Intent B", "p1", "medium")
	require.NoError(t, err, "same slug in different project should succeed")
}

func TestListSpecs(t *testing.T) {
	clearDatabase(t)

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
	clearDatabase(t)

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
	updated, err := store.UpdateSpec(ctx, "update-me", &newIntent, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "Updated intent", updated.Intent)
	require.Equal(t, int32(2), updated.Version)
	require.Equal(t, storage.SpecPriorityP2, updated.Priority) // unchanged
	require.Equal(t, storage.SpecStageSpark, updated.Stage)    // unchanged

	// Update multiple fields.
	newStage := "shape"
	newPriority := "p0"
	updated2, err := store.UpdateSpec(ctx, "update-me", nil, &newStage, &newPriority, nil, nil)
	require.NoError(t, err)
	require.Equal(t, storage.SpecStageShape, updated2.Stage)
	require.Equal(t, storage.SpecPriorityP0, updated2.Priority)
	require.Equal(t, int32(3), updated2.Version)
	require.Equal(t, "Updated intent", updated2.Intent) // still from previous update

	// Update non-existent spec.
	_, err = store.UpdateSpec(ctx, "no-such-spec", &newIntent, nil, nil, nil, nil)
	require.Error(t, err)
}

func TestCreateSpec_SetsContentHash(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	spec, err := store.CreateSpec(ctx, "hash-test", "Test content hashing", "p1", "medium")
	require.NoError(t, err)
	require.Len(t, spec.ContentHash, 32, "content_hash should be 32-char hex")

	// Update should change the hash.
	newIntent := "Updated intent"
	updated, err := store.UpdateSpec(ctx, "hash-test", &newIntent, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, updated.ContentHash, 32)
	require.NotEqual(t, spec.ContentHash, updated.ContentHash, "hash should change when intent changes")

	// GetSpec should return the same hash as UpdateSpec.
	fetched, err := store.GetSpec(ctx, "hash-test")
	require.NoError(t, err)
	require.Equal(t, updated.ContentHash, fetched.ContentHash)
}

func TestGetSpec_NotFound(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.GetSpec(ctx, "nonexistent")
	require.Error(t, err)
}
