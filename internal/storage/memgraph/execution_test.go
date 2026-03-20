// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"context"
	"testing"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr(s string) *string { return &s }

func TestExecution(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	t.Run("GenerateBundle_SpecNotFound", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.GenerateBundle(ctx, "nonexistent")
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrSpecNotFound)
	})

	t.Run("GenerateBundle_NotApproved", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "spark-spec", "A spec in spark stage", "p1", "low")
		require.NoError(t, err)

		_, err = store.GenerateBundle(ctx, "spark-spec")
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrSpecNotApproved)
	})

	t.Run("GenerateBundle_Success", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create and approve spec.
		_, err = store.CreateSpec(ctx, "bundle-spec", "Build the thing", "p1", "medium")
		require.NoError(t, err)
		_, err = store.UpdateSpec(ctx, "bundle-spec", nil, ptr("approved"), nil, nil, nil)
		require.NoError(t, err)

		// Create decision and link it.
		_, err = store.CreateDecision(ctx, "dec-1", "Use Go", "We will use Go", "Performance")
		require.NoError(t, err)
		_, err = store.AddEdge(ctx, "bundle-spec", "dec-1", storage.EdgeTypeDecidedIn)
		require.NoError(t, err)

		// Generate bundle.
		bundle, err := store.GenerateBundle(ctx, "bundle-spec")
		require.NoError(t, err)
		require.NotNil(t, bundle)
		assert.Equal(t, int32(1), bundle.Version)
		assert.Equal(t, "bundle-spec", bundle.Spec.Slug)
		assert.Equal(t, storage.SpecStageApproved, bundle.Spec.Stage)
		require.Len(t, bundle.Decisions, 1)
		assert.Equal(t, "dec-1", bundle.Decisions[0].Slug)
		assert.Equal(t, "Use Go", bundle.Decisions[0].Title)
	})

	t.Run("RecordProgress_NoClaim", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create and approve spec but do not claim it.
		_, err = store.CreateSpec(ctx, "unclaimed", "Unclaimed spec", "p1", "low")
		require.NoError(t, err)
		_, err = store.UpdateSpec(ctx, "unclaimed", nil, ptr("approved"), nil, nil, nil)
		require.NoError(t, err)

		err = store.RecordProgress(ctx, "unclaimed", "agent-x", "doing work")
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrAgentNotClaimOwner)
	})

	t.Run("FullLifecycle", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create, approve, and claim spec.
		_, err = store.CreateSpec(ctx, "lifecycle", "Full lifecycle spec", "p0", "high")
		require.NoError(t, err)
		_, err = store.UpdateSpec(ctx, "lifecycle", nil, ptr("approved"), nil, nil, nil)
		require.NoError(t, err)
		_, err = store.ClaimSpec(ctx, "lifecycle", "agent-1", 15*time.Minute)
		require.NoError(t, err)

		// Record progress.
		err = store.RecordProgress(ctx, "lifecycle", "agent-1", "started implementation")
		require.NoError(t, err)

		// Record blocker.
		err = store.RecordBlocker(ctx, "lifecycle", "agent-1", "waiting for dependency")
		require.NoError(t, err)

		// Record completion.
		err = store.RecordCompletion(ctx, "lifecycle", "agent-1")
		require.NoError(t, err)

		// Verify spec stage is now "done".
		spec, err := store.GetSpec(ctx, "lifecycle")
		require.NoError(t, err)
		assert.Equal(t, storage.SpecStage("done"), spec.Stage)

		// Verify events — all three types present. ULID ordering within the same
		// millisecond is non-deterministic, so check by type set rather than index.
		events, err := store.GetExecutionEvents(ctx, "lifecycle", 10)
		require.NoError(t, err)
		require.Len(t, events, 3)
		typeSet := map[storage.ExecutionEventType]bool{}
		for _, e := range events {
			typeSet[e.Type] = true
			assert.Equal(t, "lifecycle", e.SpecSlug)
			assert.Equal(t, "agent-1", e.Agent)
		}
		assert.True(t, typeSet[storage.ExecutionEventTypeCompletion], "missing completion event")
		assert.True(t, typeSet[storage.ExecutionEventTypeBlocker], "missing blocker event")
		assert.True(t, typeSet[storage.ExecutionEventTypeProgress], "missing progress event")

		// Verify claim was released — reclaiming should succeed.
		_, err = store.ClaimSpec(ctx, "lifecycle", "agent-2", 10*time.Minute)
		require.NoError(t, err)
	})

	t.Run("GetExecutionEvents_DeduplicatesPaths", func(t *testing.T) {
		// Regression test for DISTINCT fix: GetExecutionEvents must return
		// each event exactly once even when the graph has multiple paths
		// from Project to Spec (e.g., duplicate BELONGS_TO edges).
		// Note: the original duplicates were observed in E2E tests with
		// multi-project scoping through ProjectMiddleware; injecting a
		// duplicate edge here is a best-effort reproduction. The DISTINCT
		// is defensive — if this test passes without it, the protection
		// is still warranted for the E2E scenario.
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		// Set up: create spec, approve, claim, record 2 events.
		_, err = store.CreateSpec(ctx, "dedup-test", "Dedup test spec", "p1", "medium")
		require.NoError(t, err)
		_, err = store.UpdateSpec(ctx, "dedup-test", nil, ptr("approved"), nil, nil, nil)
		require.NoError(t, err)
		_, err = store.ClaimSpec(ctx, "dedup-test", "agent-dedup", 15*time.Minute)
		require.NoError(t, err)
		require.NoError(t, store.RecordProgress(ctx, "dedup-test", "agent-dedup", "working"))
		require.NoError(t, store.RecordBlocker(ctx, "dedup-test", "agent-dedup", "stuck"))

		// Inject a duplicate BELONGS_TO edge to create a second path
		// from Project to Spec — this is the graph topology that caused
		// GetExecutionEvents to return duplicated rows before DISTINCT was added.
		driver, dErr := neo4j.NewDriverWithContext(boltURI, neo4j.NoAuth())
		require.NoError(t, dErr)
		defer driver.Close(ctx)
		_, dErr = neo4j.ExecuteQuery(ctx, driver,
			`MATCH (p:Project {slug: $project}), (s:Spec {slug: "dedup-test"})
			 CREATE (s)-[:BELONGS_TO]->(p)`,
			map[string]any{"project": "e2e-test"},
			neo4j.EagerResultTransformer)
		require.NoError(t, dErr)

		// Verify: should still return exactly 2 events, not 4.
		events, evtErr := store.GetExecutionEvents(ctx, "dedup-test", 10)
		require.NoError(t, evtErr)
		require.Len(t, events, 2, "DISTINCT should deduplicate events from multiple BELONGS_TO paths")

		idSet := map[string]bool{}
		for _, e := range events {
			require.False(t, idSet[e.ID], "duplicate event ID: %s", e.ID)
			idSet[e.ID] = true
		}
	})

	t.Run("GetPrimeData", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create spec.
		_, err = store.CreateSpec(ctx, "prime-spec", "Spec for prime data", "p1", "medium")
		require.NoError(t, err)

		// Create decision and link.
		_, err = store.CreateDecision(ctx, "prime-dec", "Use Redis", "Redis for caching", "Speed")
		require.NoError(t, err)
		_, err = store.AddEdge(ctx, "prime-spec", "prime-dec", storage.EdgeTypeDecidedIn)
		require.NoError(t, err)

		// Set up constitution.
		_, err = store.UpdateConstitution(ctx, &storage.Constitution{
			Layer: storage.ConstitutionLayerProject,
			Name:  "test-constitution",
		})
		require.NoError(t, err)

		// Get prime data.
		pd, err := store.GetPrimeData(ctx, "prime-spec")
		require.NoError(t, err)
		require.NotNil(t, pd)
		assert.Equal(t, "prime-spec", pd.Spec.Slug)
		require.Len(t, pd.Decisions, 1)
		assert.Equal(t, "prime-dec", pd.Decisions[0].Slug)
		require.NotNil(t, pd.Constitution)
		assert.Equal(t, "test-constitution", pd.Constitution.Name)
	})

	t.Run("GetPrimeData_NoConstitution", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		_, err = store.CreateSpec(ctx, "no-con-spec", "Spec without constitution", "p2", "low")
		require.NoError(t, err)

		pd, err := store.GetPrimeData(ctx, "no-con-spec")
		require.NoError(t, err)
		require.NotNil(t, pd)
		assert.Equal(t, "no-con-spec", pd.Spec.Slug)
		assert.Empty(t, pd.Decisions)
		assert.Nil(t, pd.Constitution)
	})

	t.Run("ReleaseExpiredClaims", func(t *testing.T) {
		clearGraph(t, boltURI)
		ctx := context.Background()
		store, err := newStore(ctx, boltURI)
		require.NoError(t, err)
		defer store.Close(ctx)

		// Create and claim spec with a very short lease (1 second).
		_, err = store.CreateSpec(ctx, "expiring", "Expiring claim spec", "p1", "low")
		require.NoError(t, err)
		_, err = store.ClaimSpec(ctx, "expiring", "agent-slow", 1*time.Second)
		require.NoError(t, err)

		// Wait for lease to expire.
		time.Sleep(2 * time.Second)

		// Release expired claims.
		released, err := store.ReleaseExpiredClaims(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, released)

		// Verify claim is gone — another agent can claim.
		_, err = store.ClaimSpec(ctx, "expiring", "agent-fast", 10*time.Minute)
		require.NoError(t, err)
	})
}
