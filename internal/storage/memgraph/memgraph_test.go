package memgraph_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/seanb4t/specgraph/internal/storage/memgraph"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupMemgraph(t *testing.T) (string, func()) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "memgraph/memgraph:latest",
		ExposedPorts: []string{"7687/tcp"},
		WaitingFor:   wait.ForListeningPort("7687/tcp").WithStartupTimeout(30 * time.Second),
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

func TestCreateAndGetSpec(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	spec, err := store.CreateSpec(ctx, "login-api", "Implement login API", "p1", "medium")
	require.NoError(t, err)
	require.NotNil(t, spec)
	require.Contains(t, spec.Id, "spec-")
	require.Len(t, spec.Id, 12) // "spec-" + 7 hex chars
	require.Equal(t, "login-api", spec.Slug)
	require.Equal(t, "Implement login API", spec.Intent)
	require.Equal(t, "spark", spec.Stage)
	require.Equal(t, "p1", spec.Priority)
	require.Equal(t, "medium", spec.Complexity)
	require.Equal(t, int32(1), spec.Version)
	require.NotNil(t, spec.CreatedAt)
	require.NotNil(t, spec.UpdatedAt)

	got, err := store.GetSpec(ctx, "login-api")
	require.NoError(t, err)
	require.Equal(t, spec.Id, got.Id)
	require.Equal(t, spec.Slug, got.Slug)
	require.Equal(t, spec.Intent, got.Intent)
	require.Equal(t, spec.Stage, got.Stage)
	require.Equal(t, spec.Priority, got.Priority)
	require.Equal(t, spec.Complexity, got.Complexity)
	require.Equal(t, spec.Version, got.Version)
	require.Equal(t, spec.CreatedAt.AsTime().Unix(), got.CreatedAt.AsTime().Unix())
	require.Equal(t, spec.UpdatedAt.AsTime().Unix(), got.UpdatedAt.AsTime().Unix())
}

func TestListSpecs(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
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

func TestGetSpec_NotFound(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.GetSpec(ctx, "nonexistent")
	require.Error(t, err)
}
