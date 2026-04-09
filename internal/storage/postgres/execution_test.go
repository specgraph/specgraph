// SPDX-License-Identifier: Apache-2.0
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

// approveAndClaim is a test helper that creates, approves, and claims a spec.
func approveAndClaim(t *testing.T, store *postgres.Store, slug, agent string) {
	t.Helper()
	ctx := context.Background()
	ptr := func(s string) *string { return &s }

	_, err := store.CreateSpec(ctx, slug, "test intent for "+slug, "p1", "medium")
	require.NoError(t, err)

	_, err = store.UpdateSpec(ctx, slug, nil, ptr("approved"), nil, nil, nil)
	require.NoError(t, err)

	_, err = store.ClaimSpec(ctx, slug, agent, 15*time.Minute)
	require.NoError(t, err)
}

func TestRecordProgress_Basic(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	approveAndClaim(t, store, "prog-spec", "agent-1")

	err := store.RecordProgress(ctx, "prog-spec", "agent-1", "making progress")
	require.NoError(t, err)

	events, err := store.GetExecutionEvents(ctx, "prog-spec", 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, storage.ExecutionEventTypeProgress, events[0].Type)
	assert.Equal(t, "making progress", events[0].Message)
	assert.Equal(t, "agent-1", events[0].Agent)
	assert.Equal(t, "prog-spec", events[0].SpecSlug)
}

func TestRecordProgress_NoClaim(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()
	ptr := func(s string) *string { return &s }

	_, err := store.CreateSpec(ctx, "unclaimed-spec", "unclaimed", "p1", "low")
	require.NoError(t, err)
	_, err = store.UpdateSpec(ctx, "unclaimed-spec", nil, ptr("approved"), nil, nil, nil)
	require.NoError(t, err)

	err = store.RecordProgress(ctx, "unclaimed-spec", "agent-x", "doing work")
	require.ErrorIs(t, err, storage.ErrAgentNotClaimOwner)
}

func TestRecordBlocker(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	approveAndClaim(t, store, "blocker-spec", "agent-2")

	err := store.RecordBlocker(ctx, "blocker-spec", "agent-2", "waiting for upstream")
	require.NoError(t, err)

	events, err := store.GetExecutionEvents(ctx, "blocker-spec", 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, storage.ExecutionEventTypeBlocker, events[0].Type)
	assert.Equal(t, "waiting for upstream", events[0].Message)
}

func TestRecordBlocker_WrongAgent(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	approveAndClaim(t, store, "blocker-wrong", "agent-real")

	err := store.RecordBlocker(ctx, "blocker-wrong", "agent-impostor", "trying to block")
	require.ErrorIs(t, err, storage.ErrAgentNotClaimOwner)
}

func TestRecordCompletion_Atomic(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	approveAndClaim(t, store, "complete-spec", "agent-3")

	// Record progress first.
	require.NoError(t, store.RecordProgress(ctx, "complete-spec", "agent-3", "halfway"))

	// Complete the spec.
	err := store.RecordCompletion(ctx, "complete-spec", "agent-3")
	require.NoError(t, err)

	// Spec must be in "done" stage.
	spec, err := store.GetSpec(ctx, "complete-spec")
	require.NoError(t, err)
	assert.Equal(t, storage.SpecStage("done"), spec.Stage)

	// Claim must be gone — another agent can now claim.
	_, err = store.ClaimSpec(ctx, "complete-spec", "agent-new", 5*time.Minute)
	require.NoError(t, err)

	// Events must include both progress and completion.
	events, err := store.GetExecutionEvents(ctx, "complete-spec", 10)
	require.NoError(t, err)
	typeSet := map[storage.ExecutionEventType]bool{}
	for _, e := range events {
		typeSet[e.Type] = true
	}
	assert.True(t, typeSet[storage.ExecutionEventTypeProgress], "missing progress event")
	assert.True(t, typeSet[storage.ExecutionEventTypeCompletion], "missing completion event")
}

func TestRecordCompletion_WrongAgent(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	approveAndClaim(t, store, "complete-wrong", "agent-owner")

	err := store.RecordCompletion(ctx, "complete-wrong", "agent-interloper")
	require.ErrorIs(t, err, storage.ErrAgentNotClaimOwner)

	// Spec must still be in approved stage.
	spec, err := store.GetSpec(ctx, "complete-wrong")
	require.NoError(t, err)
	assert.Equal(t, storage.SpecStageApproved, spec.Stage)
}

func TestRecordCompletion_HashRefreshed(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()
	ptr := func(s string) *string { return &s }

	// Create a spec that depends on another, and complete the downstream.
	// RefreshDependencyHashes(slug) refreshes the completing spec's outgoing
	// DEPENDS_ON edges — i.e. the hash recorded on edges FROM the completing spec.

	_, err := store.CreateSpec(ctx, "dep-target", "dep target", "p1", "medium")
	require.NoError(t, err)

	_, err = store.CreateSpec(ctx, "completing-spec", "completing", "p1", "medium")
	require.NoError(t, err)

	// completing-spec depends on dep-target.
	_, err = store.AddEdge(ctx, "completing-spec", "dep-target", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	// Update dep-target so its content hash differs from the baseline at link time.
	newIntent := "updated dep target"
	_, err = store.UpdateSpec(ctx, "dep-target", &newIntent, nil, nil, nil, nil)
	require.NoError(t, err)

	// Approve and complete completing-spec.
	_, err = store.UpdateSpec(ctx, "completing-spec", nil, ptr("approved"), nil, nil, nil)
	require.NoError(t, err)
	_, err = store.ClaimSpec(ctx, "completing-spec", "agent-c", 15*time.Minute)
	require.NoError(t, err)
	require.NoError(t, store.RecordCompletion(ctx, "completing-spec", "agent-c"))

	// The edge from completing-spec → dep-target should now have content_hash_at_link
	// equal to dep-target's current content hash (refreshed on completion).
	refs, err := store.GetDependenciesWithEdgeData(ctx, "completing-spec")
	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.NotEmpty(t, refs[0].ContentHashAtLink)
	assert.Equal(t, refs[0].UpstreamContentHash, refs[0].ContentHashAtLink,
		"content_hash_at_link should be refreshed to match upstream's current hash after completion")
}

func TestGetExecutionEvents_Order(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	approveAndClaim(t, store, "order-spec", "agent-ord")

	require.NoError(t, store.RecordProgress(ctx, "order-spec", "agent-ord", "first"))
	require.NoError(t, store.RecordBlocker(ctx, "order-spec", "agent-ord", "second"))
	require.NoError(t, store.RecordCompletion(ctx, "order-spec", "agent-ord"))

	// Events ordered descending by created_at — completion last inserted is first returned.
	events, err := store.GetExecutionEvents(ctx, "order-spec", 10)
	require.NoError(t, err)
	require.Len(t, events, 3)

	// Verify all three types present.
	typeSet := map[storage.ExecutionEventType]bool{}
	for _, e := range events {
		typeSet[e.Type] = true
	}
	assert.True(t, typeSet[storage.ExecutionEventTypeProgress])
	assert.True(t, typeSet[storage.ExecutionEventTypeBlocker])
	assert.True(t, typeSet[storage.ExecutionEventTypeCompletion])
}

func TestGetExecutionEvents_Limit(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	approveAndClaim(t, store, "limit-spec", "agent-lim")
	require.NoError(t, store.RecordProgress(ctx, "limit-spec", "agent-lim", "one"))
	require.NoError(t, store.RecordBlocker(ctx, "limit-spec", "agent-lim", "two"))

	events, err := store.GetExecutionEvents(ctx, "limit-spec", 1)
	require.NoError(t, err)
	require.Len(t, events, 1)
}

func TestGenerateBundle(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Create spec, approve, and claim.
	approveAndClaim(t, store, "bundle-spec", "agent-bundle")

	// Create decision and link via DECIDED_IN.
	_, err := store.CreateDecision(ctx, "bundle-dec", "Use PostgreSQL", "We use PG", "Reliability",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "bundle-spec", "bundle-dec", storage.EdgeTypeDecidedIn)
	require.NoError(t, err)

	bundle, err := store.GenerateBundle(ctx, "bundle-spec")
	require.NoError(t, err)
	require.NotNil(t, bundle)
	assert.Equal(t, int32(2), bundle.Version)
	assert.Equal(t, "bundle-spec", bundle.Spec.Slug)
	assert.Equal(t, storage.SpecStageApproved, bundle.Spec.Stage)
	require.Len(t, bundle.Decisions, 1)
	assert.Equal(t, "bundle-dec", bundle.Decisions[0].Slug)
	require.NotNil(t, bundle.Claim)
	assert.Equal(t, "agent-bundle", bundle.Claim.Agent)
}

func TestGenerateBundle_NotApproved(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "spark-spec", "spark stage spec", "p1", "low")
	require.NoError(t, err)

	_, err = store.GenerateBundle(ctx, "spark-spec")
	require.ErrorIs(t, err, storage.ErrSpecNotApproved)
}

func TestGenerateBundle_NotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.GenerateBundle(ctx, "no-such-spec")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestGenerateBundle_IncludesDependencies(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()
	ptr := func(s string) *string { return &s }

	_, err := store.CreateSpec(ctx, "upstream-dep", "upstream", "p1", "medium")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "downstream-dep", "downstream", "p1", "medium")
	require.NoError(t, err)

	_, err = store.AddEdge(ctx, "downstream-dep", "upstream-dep", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	_, err = store.UpdateSpec(ctx, "downstream-dep", nil, ptr("approved"), nil, nil, nil)
	require.NoError(t, err)

	bundle, err := store.GenerateBundle(ctx, "downstream-dep")
	require.NoError(t, err)
	require.Len(t, bundle.Dependencies, 1)
	assert.Equal(t, "upstream-dep", bundle.Dependencies[0].Slug)
}

func TestGetPrimeData(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "prime-spec", "prime intent", "p1", "medium")
	require.NoError(t, err)

	_, err = store.CreateDecision(ctx, "prime-dec", "Use Redis", "Redis for caching", "Speed",
		"", nil, "", nil, "", "", "")
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "prime-spec", "prime-dec", storage.EdgeTypeDecidedIn)
	require.NoError(t, err)

	_, err = store.UpdateConstitution(ctx, &storage.Constitution{
		Layer: storage.ConstitutionLayerProject,
		Name:  "test-constitution",
	})
	require.NoError(t, err)

	pd, err := store.GetPrimeData(ctx, "prime-spec")
	require.NoError(t, err)
	require.NotNil(t, pd)
	assert.Equal(t, "prime-spec", pd.Spec.Slug)
	require.Len(t, pd.Decisions, 1)
	assert.Equal(t, "prime-dec", pd.Decisions[0].Slug)
	require.NotNil(t, pd.Constitution)
	assert.Equal(t, "test-constitution", pd.Constitution.Name)
}

func TestGetPrimeData_NoConstitution(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "no-con-spec", "spec without constitution", "p1", "low")
	require.NoError(t, err)

	pd, err := store.GetPrimeData(ctx, "no-con-spec")
	require.NoError(t, err)
	require.NotNil(t, pd)
	assert.Equal(t, "no-con-spec", pd.Spec.Slug)
	assert.Empty(t, pd.Decisions)
	assert.Nil(t, pd.Constitution)
}

func TestGetPrimeData_NotFound(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.GetPrimeData(ctx, "missing-spec")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestReleaseExpiredClaims(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Claim with a very short lease using a past clock so it's already expired.
	pastStore := newStore(t, postgres.WithClock(func() time.Time {
		return time.Now().Add(-5 * time.Minute)
	}))

	_, err := pastStore.CreateSpec(ctx, "expiring-spec", "expiring", "p1", "low")
	require.NoError(t, err)

	_, err = pastStore.ClaimSpec(ctx, "expiring-spec", "agent-slow", 1*time.Second)
	require.NoError(t, err)

	// Release expired claims from the present-time store.
	released, err := store.ReleaseExpiredClaims(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, released)

	// Another agent can now claim.
	_, err = store.ClaimSpec(ctx, "expiring-spec", "agent-fast", 5*time.Minute)
	require.NoError(t, err)
}

func TestReleaseExpiredClaims_NoneExpired(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "live-spec", "live claim spec", "p1", "low")
	require.NoError(t, err)
	_, err = store.ClaimSpec(ctx, "live-spec", "agent-live", 15*time.Minute)
	require.NoError(t, err)

	released, err := store.ReleaseExpiredClaims(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, released)
}

func TestFullExecutionLifecycle(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	approveAndClaim(t, store, "lifecycle-spec", "agent-lc")

	require.NoError(t, store.RecordProgress(ctx, "lifecycle-spec", "agent-lc", "started"))
	require.NoError(t, store.RecordBlocker(ctx, "lifecycle-spec", "agent-lc", "blocked"))
	require.NoError(t, store.RecordCompletion(ctx, "lifecycle-spec", "agent-lc"))

	spec, err := store.GetSpec(ctx, "lifecycle-spec")
	require.NoError(t, err)
	assert.Equal(t, storage.SpecStage("done"), spec.Stage)

	events, err := store.GetExecutionEvents(ctx, "lifecycle-spec", 10)
	require.NoError(t, err)
	require.Len(t, events, 3)

	typeSet := map[storage.ExecutionEventType]bool{}
	for _, e := range events {
		typeSet[e.Type] = true
		assert.Equal(t, "lifecycle-spec", e.SpecSlug)
		assert.Equal(t, "agent-lc", e.Agent)
	}
	assert.True(t, typeSet[storage.ExecutionEventTypeProgress])
	assert.True(t, typeSet[storage.ExecutionEventTypeBlocker])
	assert.True(t, typeSet[storage.ExecutionEventTypeCompletion])

	// Claim released — another agent can claim.
	_, err = store.ClaimSpec(ctx, "lifecycle-spec", "agent-next", 5*time.Minute)
	require.NoError(t, err)
}
