// SPDX-License-Identifier: MIT
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
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockClaimBackend struct {
	mu     sync.Mutex
	claims map[string]*specv1.Claim
}

func newMockClaimBackend() *mockClaimBackend {
	return &mockClaimBackend{claims: make(map[string]*specv1.Claim)}
}

func (m *mockClaimBackend) ClaimSpec(_ context.Context, slug, agent string, leaseDuration time.Duration) (*specv1.Claim, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.claims[slug]; ok {
		if existing.LeaseExpires.AsTime().After(time.Now()) {
			return nil, fmt.Errorf("spec %q already claimed by %s", slug, existing.Agent)
		}
	}
	if leaseDuration == 0 {
		leaseDuration = 15 * time.Minute
	}
	now := time.Now()
	claim := &specv1.Claim{
		SpecSlug:     slug,
		Agent:        agent,
		ClaimedAt:    timestamppb.New(now),
		LeaseExpires: timestamppb.New(now.Add(leaseDuration)),
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

func (m *mockClaimBackend) Heartbeat(_ context.Context, slug, _ string, extendBy time.Duration) (*specv1.Claim, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	claim, ok := m.claims[slug]
	if !ok {
		return nil, fmt.Errorf("no claim for spec %q", slug)
	}
	if extendBy == 0 {
		extendBy = 15 * time.Minute
	}
	claim.LeaseExpires = timestamppb.New(time.Now().Add(extendBy))
	return claim, nil
}

func setupClaimServer(t *testing.T) specgraphv1connect.ClaimServiceClient {
	t.Helper()
	mb := newMockClaimBackend()
	mux := http.NewServeMux()
	server.RegisterClaimService(mux, mb)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewClaimServiceClient(http.DefaultClient, srv.URL)
}

func TestClaimHandler_ClaimAndUnclaim(t *testing.T) {
	client := setupClaimServer(t)
	ctx := context.Background()

	// Claim
	claimResp, err := client.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
		Slug:          "my-spec",
		Agent:         "agent-1",
		LeaseDuration: durationpb.New(10 * time.Minute),
	}))
	require.NoError(t, err)
	require.Equal(t, "my-spec", claimResp.Msg.SpecSlug)
	require.Equal(t, "agent-1", claimResp.Msg.Agent)
	require.True(t, claimResp.Msg.LeaseExpires.AsTime().After(time.Now()))

	// Unclaim
	_, err = client.UnclaimSpec(ctx, connect.NewRequest(&specv1.UnclaimSpecRequest{
		Slug:  "my-spec",
		Agent: "agent-1",
	}))
	require.NoError(t, err)
}

func TestClaimHandler_Heartbeat(t *testing.T) {
	client := setupClaimServer(t)
	ctx := context.Background()

	// Claim first
	_, err := client.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
		Slug:  "hb-spec",
		Agent: "agent-1",
	}))
	require.NoError(t, err)

	// Heartbeat
	hbResp, err := client.Heartbeat(ctx, connect.NewRequest(&specv1.HeartbeatRequest{
		Slug:     "hb-spec",
		Agent:    "agent-1",
		ExtendBy: durationpb.New(30 * time.Minute),
	}))
	require.NoError(t, err)
	require.True(t, hbResp.Msg.LeaseExpires.AsTime().After(time.Now().Add(29*time.Minute)))

	// Heartbeat on non-existent claim
	_, err = client.Heartbeat(ctx, connect.NewRequest(&specv1.HeartbeatRequest{
		Slug:  "no-such-spec",
		Agent: "agent-1",
	}))
	require.Error(t, err)
}
