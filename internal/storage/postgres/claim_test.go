// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/postgres"
	"github.com/stretchr/testify/require"
)

func TestClaimSpec_Basic(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "claim-basic", "intent", "p1", "low", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	claim, err := store.ClaimSpec(ctx, "claim-basic", "agent-a", 5*time.Minute)
	require.NoError(t, err)
	require.Equal(t, "claim-basic", claim.Slug)
	require.Equal(t, "agent-a", claim.Agent)
	require.False(t, claim.ClaimedAt.IsZero())
	require.True(t, claim.LeaseExpires.After(time.Now()))
}

func TestClaimSpec_ExpiredReleasedFirst(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	// Use a fixed clock: first call sets claimed_at to "now", which is in the past.
	fixedPast := time.Now().Add(-30 * time.Minute)
	pastStore := newStore(t, postgres.WithClock(func() time.Time { return fixedPast }))

	_, err := pastStore.CreateSpec(ctx, "claim-expired", "intent", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	// Claim with 1-second lease from past clock — already expired by now.
	_, err = pastStore.ClaimSpec(ctx, "claim-expired", "agent-old", 1*time.Second)
	require.NoError(t, err)

	// Now claim from a current store — expired claim should be swept, new claim succeeds.
	_, err = store.ClaimSpec(ctx, "claim-expired", "agent-new", 5*time.Minute)
	require.NoError(t, err)
}

func TestClaimSpec_DuplicateReturnsError(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "claim-dup", "intent", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	_, err = store.ClaimSpec(ctx, "claim-dup", "agent-a", 5*time.Minute)
	require.NoError(t, err)

	_, err = store.ClaimSpec(ctx, "claim-dup", "agent-b", 5*time.Minute)
	require.ErrorIs(t, err, storage.ErrSpecAlreadyClaimed)
}

func TestClaimSpec_SameAgentRefreshesLease(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "claim-refresh", "intent", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	c1, err := store.ClaimSpec(ctx, "claim-refresh", "agent-a", 5*time.Minute)
	require.NoError(t, err)

	c2, err := store.ClaimSpec(ctx, "claim-refresh", "agent-a", 30*time.Minute)
	require.NoError(t, err)
	require.True(t, c2.LeaseExpires.After(c1.LeaseExpires))
}

func TestUnclaimSpec(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "unclaim-spec", "intent", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	_, err = store.ClaimSpec(ctx, "unclaim-spec", "agent-a", 5*time.Minute)
	require.NoError(t, err)

	err = store.UnclaimSpec(ctx, "unclaim-spec", "agent-a")
	require.NoError(t, err)

	// After unclaiming, a different agent can now claim.
	_, err = store.ClaimSpec(ctx, "unclaim-spec", "agent-b", 5*time.Minute)
	require.NoError(t, err)
}

func TestUnclaimSpec_NotClaimed(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "unclaim-none", "intent", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	err = store.UnclaimSpec(ctx, "unclaim-none", "agent-a")
	require.ErrorIs(t, err, storage.ErrSpecNotClaimed)
}

func TestUnclaimSpec_WrongAgent(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "unclaim-wrong", "intent", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	_, err = store.ClaimSpec(ctx, "unclaim-wrong", "agent-a", 5*time.Minute)
	require.NoError(t, err)

	err = store.UnclaimSpec(ctx, "unclaim-wrong", "agent-b")
	require.True(t, errors.Is(err, storage.ErrNotClaimOwner))
}

func TestHeartbeat(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "heartbeat-spec", "intent", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	c1, err := store.ClaimSpec(ctx, "heartbeat-spec", "agent-a", 5*time.Minute)
	require.NoError(t, err)

	c2, err := store.Heartbeat(ctx, "heartbeat-spec", "agent-a", 30*time.Minute)
	require.NoError(t, err)
	require.Equal(t, "agent-a", c2.Agent)
	require.True(t, c2.LeaseExpires.After(c1.LeaseExpires))
}

func TestHeartbeat_NoActiveClaim(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "heartbeat-none", "intent", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	_, err = store.Heartbeat(ctx, "heartbeat-none", "agent-a", 5*time.Minute)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no active claim")
}

func TestGetActiveClaim_Unclaimed(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "gac-unclaimed", "intent", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	claim, err := store.GetActiveClaim(ctx, "gac-unclaimed")
	require.NoError(t, err)
	require.Nil(t, claim, "unclaimed spec should return nil + no error")
}

func TestGetActiveClaim_Active(t *testing.T) {
	store := newStore(t)
	clearDatabase(t, store)
	ctx := context.Background()

	_, err := store.CreateSpec(ctx, "gac-active", "intent", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)

	_, err = store.ClaimSpec(ctx, "gac-active", "agent-a", 5*time.Minute)
	require.NoError(t, err)

	claim, err := store.GetActiveClaim(ctx, "gac-active")
	require.NoError(t, err)
	require.NotNil(t, claim)
	require.Equal(t, "gac-active", claim.Slug)
	require.Equal(t, "agent-a", claim.Agent)
	require.True(t, claim.LeaseExpires.After(time.Now()))
}

func TestGetActiveClaim_ExpiredFilteredOut(t *testing.T) {
	ctx := context.Background()
	// Use a fixed past clock so the lease is written with lease_expires in
	// the past relative to wall-clock "now".
	fixedPast := time.Now().Add(-30 * time.Minute)
	pastStore := newStore(t, postgres.WithClock(func() time.Time { return fixedPast }))
	clearDatabase(t, pastStore)

	_, err := pastStore.CreateSpec(ctx, "gac-expired", "intent", "", "", storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
	require.NoError(t, err)
	_, err = pastStore.ClaimSpec(ctx, "gac-expired", "agent-a", 5*time.Minute)
	require.NoError(t, err)

	// Real-time store sees lease_expires < now → no active claim.
	store := newStore(t)
	claim, err := store.GetActiveClaim(ctx, "gac-expired")
	require.NoError(t, err)
	require.Nil(t, claim, "expired lease should be filtered out at the storage layer")
}
