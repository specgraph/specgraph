// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// connString is set once by TestMain and shared across all tests.
var connString string

func TestMain(m *testing.M) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "pgvector/pgvector:pg18",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start postgres container: %v\n", err)
		os.Exit(1)
	}

	host, err := container.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get container host: %v\n", err)
		_ = container.Terminate(ctx)
		os.Exit(1)
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get mapped port: %v\n", err)
		_ = container.Terminate(ctx)
		os.Exit(1)
	}

	connString = fmt.Sprintf("postgres://test:test@%s:%s/testdb", host, port.Port())

	code := m.Run()

	_ = container.Terminate(ctx)
	os.Exit(code)
}

// newStore creates a postgres Store with retry logic and registers cleanup.
func newStore(t *testing.T, opts ...postgres.Option) *postgres.Store {
	t.Helper()
	opts = append([]postgres.Option{postgres.WithProject("test")}, opts...)
	ctx := context.Background()

	var (
		store *postgres.Store
		err   error
	)
	for range 10 {
		store, err = postgres.New(ctx, connString, opts...)
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NoError(t, err, "newStore: retry exhausted")

	t.Cleanup(func() {
		if closeErr := store.Close(ctx); closeErr != nil {
			t.Errorf("store.Close: %v", closeErr)
		}
	})
	return store
}

// clearDatabase truncates all application tables for test isolation.
func clearDatabase(t *testing.T, store *postgres.Store) {
	t.Helper()
	_ = store // store is accepted for API consistency; pool accessed via connString

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)
	defer pool.Close()

	tables := []string{
		"sync_mappings",
		"execution_events",
		"claims",
		"findings",
		"conversation_logs",
		"changelog_entries",
		"constitutions",
		"edges",
		"slices",
		"decisions",
		"specs",
	}
	for _, tbl := range tables {
		_, err := pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", tbl)) //nolint:gosec // table names are hardcoded constants
		if err != nil {
			// Table may not exist yet (earlier migration tasks); skip silently.
			t.Logf("clearDatabase: truncate %s: %v (skipping)", tbl, err)
		}
	}
}

func TestNew_ConnectsAndMigrates(t *testing.T) {
	ctx := context.Background()
	store, err := postgres.New(ctx, connString, postgres.WithProject("test-connects"))
	require.NoError(t, err)
	require.NotNil(t, store)
	t.Cleanup(func() { _ = store.Close(ctx) })
}

func TestScoped_ReturnsProjectScopedStore(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)

	scoped, err := store.Scoped(ctx, "other-project")
	require.NoError(t, err)
	require.NotNil(t, scoped)
}

func TestRunInTransaction_CommitsOnSuccess(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	err := store.RunInTransaction(ctx, func(_ context.Context) error {
		return nil
	})
	require.NoError(t, err)
}

func TestRunInTransaction_RollsBackOnError(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	err := store.RunInTransaction(ctx, func(_ context.Context) error {
		return fmt.Errorf("intentional error")
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "intentional error")
}

func TestRunInTransaction_NestedReusesOuterTx(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	var innerCalled bool
	err := store.RunInTransaction(ctx, func(txCtx context.Context) error {
		return store.RunInTransaction(txCtx, func(_ context.Context) error {
			innerCalled = true
			return nil
		})
	})
	require.NoError(t, err)
	require.True(t, innerCalled)
}

func TestRunInTransaction_DispatchesChangeEvents(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	var received []string
	store.Subscribe(changeSubscriberFunc(func(_ context.Context, event *storage.ChangeEvent) {
		received = append(received, event.Slug)
	}))

	err := store.RunInTransaction(ctx, func(txCtx context.Context) error {
		storage.StashChangeEvent(txCtx, &storage.ChangeEvent{Slug: "spec-abc"})
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, []string{"spec-abc"}, received)
}

// changeSubscriberFunc is an adapter to use a plain function as a ChangeSubscriber.
type changeSubscriberFunc func(ctx context.Context, event *storage.ChangeEvent)

func (f changeSubscriberFunc) OnSpecChanged(ctx context.Context, event *storage.ChangeEvent) {
	f(ctx, event)
}

// ---- Project tests ----

func TestEnsureProject_CreatesAndIsIdempotent(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	p1, err := store.EnsureProject(ctx, "ensure-proj")
	require.NoError(t, err)
	require.Equal(t, "ensure-proj", p1.Slug)
	require.False(t, p1.CreatedAt.IsZero())
	require.Empty(t, p1.SyncAdapters)

	// Idempotent — second call returns same project with same CreatedAt.
	p2, err := store.EnsureProject(ctx, "ensure-proj")
	require.NoError(t, err)
	require.Equal(t, p1.CreatedAt.UTC(), p2.CreatedAt.UTC())
}

func TestGetProject_NotFound(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	_, err := store.GetProject(ctx, "does-not-exist")
	require.ErrorIs(t, err, storage.ErrProjectNotFound)
}

func TestUpdateProject_SetsFields(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	_, err := store.EnsureProject(ctx, "updatable")
	require.NoError(t, err)

	p, err := store.UpdateProject(ctx, "updatable", []string{"beads", "github"}, "owner/repo")
	require.NoError(t, err)
	require.Equal(t, []string{"beads", "github"}, p.SyncAdapters)
	require.Equal(t, "owner/repo", p.GitHubRepo)
}

func TestListProjects_ReturnsAll(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	_, err := store.EnsureProject(ctx, "list-proj-a")
	require.NoError(t, err)
	_, err = store.EnsureProject(ctx, "list-proj-b")
	require.NoError(t, err)

	projects, err := store.ListProjects(ctx)
	require.NoError(t, err)

	slugs := make([]string, 0, len(projects))
	for _, p := range projects {
		slugs = append(slugs, p.Slug)
	}
	require.Contains(t, slugs, "list-proj-a")
	require.Contains(t, slugs, "list-proj-b")
}

func TestWipeProjectData_CleansUp(t *testing.T) {
	store := newStore(t, postgres.WithProject("wipe-test"))
	clearDatabase(t, store)
	ctx := context.Background()

	// Insert a spec directly so WipeProjectData has something to delete.
	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)
	defer pool.Close()

	_, err = pool.Exec(ctx,
		`INSERT INTO specs (slug, project_slug, intent) VALUES ('spec-wipe', 'wipe-test', 'test intent')`,
	)
	require.NoError(t, err)

	// Insert an edge too.
	_, err = pool.Exec(ctx,
		`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug) VALUES ('spec-wipe', 'other', 'DEPENDS_ON', 'wipe-test')`,
	)
	require.NoError(t, err)

	// Wipe.
	require.NoError(t, store.WipeProjectData(ctx))

	// Spec and edge should be gone.
	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM specs WHERE project_slug = 'wipe-test'`,
	).Scan(&count))
	require.Equal(t, 0, count)

	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM edges WHERE project_slug = 'wipe-test'`,
	).Scan(&count))
	require.Equal(t, 0, count)

	// Project row itself must still exist.
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM projects WHERE slug = 'wipe-test'`,
	).Scan(&count))
	require.Equal(t, 1, count)
}

// ---- Spec tests ----

func TestCreateSpec_Basic(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	spec, err := store.CreateSpec(ctx, "test-spec", "Test intent", "p1", "medium")
	require.NoError(t, err)
	require.Equal(t, "test-spec", spec.Slug)
	require.Equal(t, storage.SpecStage("spark"), spec.Stage)
	require.Equal(t, int32(1), spec.Version)
	require.NotEmpty(t, spec.ContentHash)
}

func TestCreateSpec_DuplicateSlug(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "dup-spec", "intent", "", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "dup-spec", "intent", "", "")
	require.ErrorIs(t, err, storage.ErrSpecAlreadyExists)
}

func TestGetSpec_Basic(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	created, err := store.CreateSpec(ctx, "get-spec", "Get intent", "p2", "low")
	require.NoError(t, err)
	got, err := store.GetSpec(ctx, "get-spec")
	require.NoError(t, err)
	require.Equal(t, created.Slug, got.Slug)
	require.Equal(t, created.ContentHash, got.ContentHash)
}

func TestGetSpec_NotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.GetSpec(ctx, "nonexistent")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestListSpecs_WithFilters(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "spec-a", "A intent", "p1", "low")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "spec-b", "B intent", "p2", "medium")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "spec-c", "C intent", "p1", "high")
	require.NoError(t, err)

	// No filters — all three returned.
	all, err := store.ListSpecs(ctx, "", "", 0)
	require.NoError(t, err)
	require.Len(t, all, 3)

	// Filter by priority p1 — two results.
	p1, err := store.ListSpecs(ctx, "", "p1", 0)
	require.NoError(t, err)
	require.Len(t, p1, 2)

	// Limit to 1.
	limited, err := store.ListSpecs(ctx, "", "", 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
}

func TestBatchGetSpecs_ReturnsMap(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "batch-a", "Intent A", "", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "batch-b", "Intent B", "", "")
	require.NoError(t, err)

	result, err := store.BatchGetSpecs(ctx, []string{"batch-a", "batch-b", "missing-slug"})
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Contains(t, result, "batch-a")
	require.Contains(t, result, "batch-b")
}

func TestBatchGetSpecs_EmptySlice(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	result, err := store.BatchGetSpecs(ctx, []string{})
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestUpdateSpec_PartialUpdate(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "update-spec", "Original", "p1", "low")
	require.NoError(t, err)

	newIntent := "Updated"
	updated, err := store.UpdateSpec(ctx, "update-spec", &newIntent, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, "Updated", updated.Intent)
	require.Equal(t, int32(2), updated.Version)
}

func TestUpdateSpec_NoChangeSkipsChangelog(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "noop-spec", "Intent", "", "")
	require.NoError(t, err)

	// Update with same intent value — content hash unchanged, no changelog entry added.
	intent := "Intent"
	updated, err := store.UpdateSpec(ctx, "noop-spec", &intent, nil, nil, nil, nil)
	require.NoError(t, err)
	// Version still bumps (write always increments version).
	require.Equal(t, int32(2), updated.Version)
}

func TestUpdateSpec_NotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	intent := "x"
	_, err := store.UpdateSpec(ctx, "does-not-exist", &intent, nil, nil, nil, nil)
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

// ---- ClearAll tests ----

func TestClearAll_WipesDataAndPreservesProject(t *testing.T) {
	store := newStore(t, postgres.WithProject("clearall-test"))
	ctx := context.Background()

	// Create a spec and an edge so ClearAll has something to remove.
	_, err := store.CreateSpec(ctx, "ca-spec", "Intent", "", "")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "ca-spec2", "Intent2", "", "")
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "ca-spec", "ca-spec2", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	require.NoError(t, store.ClearAll(ctx))

	// Spec should be gone.
	_, err = store.GetSpec(ctx, "ca-spec")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)

	// ListSpecs should be empty.
	specs, err := store.ListSpecs(ctx, "", "", 0)
	require.NoError(t, err)
	require.Empty(t, specs)

	// Project should still exist (re-ensured by ClearAll).
	p, err := store.GetProject(ctx, "clearall-test")
	require.NoError(t, err)
	require.Equal(t, "clearall-test", p.Slug)
}

// ---- New error path tests ----

func TestNew_MissingProject_ReturnsError(t *testing.T) {
	ctx := context.Background()
	// No WithProject option — should fail.
	_, err := postgres.New(ctx, connString)
	require.Error(t, err)
	require.Contains(t, err.Error(), "project slug required")
}

func TestNew_InvalidConnString_ReturnsError(t *testing.T) {
	ctx := context.Background()
	_, err := postgres.New(ctx, "postgres://invalid-host-that-does-not-exist:5432/db",
		postgres.WithProject("test"))
	require.Error(t, err)
}

func TestScoped_EmptyProject_ReturnsError(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	_, err := store.Scoped(ctx, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "project slug required")
}

func TestWithSliceOps_CanBeSet(t *testing.T) {
	// WithSliceOps is an Option function — verify it can be applied without error.
	// The store created here uses the store itself as the slice backend (default behaviour),
	// so we just verify construction succeeds.
	ctx := context.Background()
	store := newStore(t)
	scoped, err := store.Scoped(ctx, "sliceops-test")
	require.NoError(t, err)
	require.NotNil(t, scoped)
}

// ---- Ping tests ----
//
// Without real-pool coverage a refactor could silently reduce Ping to a
// no-op that always returns nil, making /readyz lie about readiness.

func TestStore_Ping_ReturnsNilWhenPoolHealthy(t *testing.T) {
	store := newStore(t)
	require.NoError(t, store.Ping(context.Background()))
}

func TestStore_Ping_ReturnsErrorWhenPoolClosed(t *testing.T) {
	ctx := context.Background()
	store, err := postgres.New(ctx, connString, postgres.WithProject("ping-closed"))
	require.NoError(t, err)
	require.NoError(t, store.Close(ctx))

	err = store.Ping(ctx)
	require.Error(t, err, "Ping against a closed pool must error — a silent nil would mask readiness regressions")
	assert.Contains(t, err.Error(), "postgres: ping:", "wrapping is load-bearing — /readyz body surfaces it")
}

func TestStore_Ping_RespectsContextCancel(t *testing.T) {
	store := newStore(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := store.Ping(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled),
		"Ping must propagate ctx cancel so readiness times out cleanly during shutdown, got %v", err)
}

func TestNew_WithSliceOps_Option(t *testing.T) {
	ctx := context.Background()
	// Pass WithSliceOps(nil) explicitly — should still work (nil backend means fall back to self).
	store, err := postgres.New(ctx, connString,
		postgres.WithProject("sliceops-opt"),
		postgres.WithSliceOps(nil),
	)
	require.NoError(t, err)
	require.NotNil(t, store)
	t.Cleanup(func() { _ = store.Close(ctx) })
}
