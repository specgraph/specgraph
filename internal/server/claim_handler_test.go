// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
)

type mockClaimBackend struct {
	stubBackend
	mu     sync.Mutex
	claims map[string]*storage.Claim
}

func newMockClaimBackend() *mockClaimBackend {
	return &mockClaimBackend{claims: make(map[string]*storage.Claim)}
}

// GetSpec returns a minimal AUTHORED spec so the claim handler's provenance
// gate passes without requiring a full storage backend.
func (m *mockClaimBackend) GetSpec(_ context.Context, slug string) (*storage.Spec, error) {
	return &storage.Spec{
		Slug:       slug,
		Provenance: storage.SpecProvenanceAuthored,
	}, nil
}

func (m *mockClaimBackend) ClaimSpec(_ context.Context, slug, agent string, leaseDuration time.Duration) (*storage.Claim, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.claims[slug]; ok {
		if existing.LeaseExpires.After(time.Now()) {
			return nil, fmt.Errorf("spec %q already claimed by %s: %w", slug, existing.Agent, storage.ErrSpecAlreadyClaimed)
		}
	}
	if leaseDuration == 0 {
		leaseDuration = 15 * time.Minute
	}
	now := time.Now()
	claim := &storage.Claim{
		Slug:         slug,
		Agent:        agent,
		ClaimedAt:    now,
		LeaseExpires: now.Add(leaseDuration),
	}
	m.claims[slug] = claim
	return claim, nil
}

func (m *mockClaimBackend) UnclaimSpec(_ context.Context, slug, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.claims, slug)
	return nil
}

func (m *mockClaimBackend) Heartbeat(_ context.Context, slug, _ string, extendBy time.Duration) (*storage.Claim, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	claim, ok := m.claims[slug]
	if !ok {
		return nil, fmt.Errorf("no claim for spec %q", slug)
	}
	if extendBy == 0 {
		extendBy = 15 * time.Minute
	}
	claim.LeaseExpires = time.Now().Add(extendBy)
	return claim, nil
}

func setupClaimServer(t *testing.T) specgraphv1connect.ClaimServiceClient {
	t.Helper()
	mb := newMockClaimBackend()
	scoper := &testScoper{backend: mb}
	mux := http.NewServeMux()
	server.RegisterClaimService(mux, scoper)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewClaimServiceClient(http.DefaultClient, srv.URL)
}

func TestClaimHandler_ClaimAndUnclaim(t *testing.T) {
	client := setupClaimServer(t)
	ctx := context.Background()

	// Claim
	claimResp, err := client.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
		SpecSlug:      "my-spec",
		Agent:         "agent-1",
		LeaseDuration: durationpb.New(10 * time.Minute),
	}))
	require.NoError(t, err)
	require.Equal(t, "my-spec", claimResp.Msg.GetClaim().GetSpecSlug())
	require.Equal(t, "agent-1", claimResp.Msg.GetClaim().GetAgent())
	require.True(t, claimResp.Msg.GetClaim().GetLeaseExpires().AsTime().After(time.Now()))

	// Unclaim
	_, err = client.UnclaimSpec(ctx, connect.NewRequest(&specv1.UnclaimSpecRequest{
		SpecSlug: "my-spec",
		Agent:    "agent-1",
	}))
	require.NoError(t, err)
}

func TestClaimHandler_Heartbeat(t *testing.T) {
	client := setupClaimServer(t)
	ctx := context.Background()

	// Claim first
	_, err := client.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
		SpecSlug: "hb-spec",
		Agent:    "agent-1",
	}))
	require.NoError(t, err)

	// Heartbeat
	hbResp, err := client.Heartbeat(ctx, connect.NewRequest(&specv1.HeartbeatRequest{
		SpecSlug: "hb-spec",
		Agent:    "agent-1",
		ExtendBy: durationpb.New(30 * time.Minute),
	}))
	require.NoError(t, err)
	require.True(t, hbResp.Msg.GetClaim().GetLeaseExpires().AsTime().After(time.Now().Add(29*time.Minute)))

	// Heartbeat on non-existent claim
	_, err = client.Heartbeat(ctx, connect.NewRequest(&specv1.HeartbeatRequest{
		SpecSlug: "no-such-spec",
		Agent:    "agent-1",
	}))
	require.Error(t, err)
}
