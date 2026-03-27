// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClaimAndUnclaim(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create a spec to claim
	_, err = store.CreateSpec(ctx, "claimable", "A claimable spec", "p1", "low")
	require.NoError(t, err)

	// Claim it
	claim, err := store.ClaimSpec(ctx, "claimable", "agent-1", 10*time.Minute)
	require.NoError(t, err)
	require.Equal(t, "claimable", claim.Slug)
	require.Equal(t, "agent-1", claim.Agent)
	require.True(t, claim.LeaseExpires.After(time.Now()))

	// Claiming again should fail (still claimed)
	_, err = store.ClaimSpec(ctx, "claimable", "agent-2", 10*time.Minute)
	require.Error(t, err)

	// Unclaim
	err = store.UnclaimSpec(ctx, "claimable", "agent-1")
	require.NoError(t, err)

	// Now agent-2 can claim
	claim2, err := store.ClaimSpec(ctx, "claimable", "agent-2", 10*time.Minute)
	require.NoError(t, err)
	require.Equal(t, "agent-2", claim2.Agent)
}

func TestHeartbeat(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "hb-spec", "Heartbeat spec", "p1", "low")
	require.NoError(t, err)

	_, err = store.ClaimSpec(ctx, "hb-spec", "agent-1", 5*time.Minute)
	require.NoError(t, err)

	// Heartbeat extends the lease
	claim, err := store.Heartbeat(ctx, "hb-spec", "agent-1", 30*time.Minute)
	require.NoError(t, err)
	require.True(t, claim.LeaseExpires.After(time.Now().Add(29*time.Minute)))

	// Heartbeat on non-existent claim
	_, err = store.Heartbeat(ctx, "no-such-spec", "agent-1", 10*time.Minute)
	require.Error(t, err)
}

func TestClaimSpec_InTransaction_RollsBack(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "tx-claim", "Transactional claim spec", "p1", "low")
	require.NoError(t, err)

	// ClaimSpec inside a failing transaction should roll back.
	deliberateErr := errors.New("deliberate rollback")
	err = store.RunInTransaction(ctx, func(txCtx context.Context) error {
		if _, claimErr := store.ClaimSpec(txCtx, "tx-claim", "agent-tx", 10*time.Minute); claimErr != nil {
			return claimErr
		}
		return deliberateErr
	})
	require.ErrorIs(t, err, deliberateErr)

	// Spec should not be claimed after rollback — another agent can claim it.
	claim, err := store.ClaimSpec(ctx, "tx-claim", "agent-after", 10*time.Minute)
	require.NoError(t, err, "claim should succeed after transaction rollback")
	require.Equal(t, "agent-after", claim.Agent)
}
