// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// --- fake handlers ---

type fakeClaimHandler struct {
	specgraphv1connect.UnimplementedClaimServiceHandler
}

func (fakeClaimHandler) ClaimSpec(_ context.Context, req *connect.Request[specv1.ClaimSpecRequest]) (*connect.Response[specv1.ClaimSpecResponse], error) {
	return connect.NewResponse(&specv1.ClaimSpecResponse{
		Claim: &specv1.Claim{
			SpecSlug:     req.Msg.GetSpecSlug(),
			Agent:        req.Msg.GetAgent(),
			LeaseExpires: timestamppb.Now(),
		},
	}), nil
}

func (fakeClaimHandler) UnclaimSpec(_ context.Context, _ *connect.Request[specv1.UnclaimSpecRequest]) (*connect.Response[specv1.UnclaimSpecResponse], error) {
	return connect.NewResponse(&specv1.UnclaimSpecResponse{}), nil
}

type fakeClaimErrorHandler struct {
	specgraphv1connect.UnimplementedClaimServiceHandler
}

func (fakeClaimErrorHandler) ClaimSpec(_ context.Context, _ *connect.Request[specv1.ClaimSpecRequest]) (*connect.Response[specv1.ClaimSpecResponse], error) {
	return nil, connect.NewError(connect.CodeAlreadyExists, nil)
}

type fakeUnclaimErrorHandler struct {
	specgraphv1connect.UnimplementedClaimServiceHandler
}

func (fakeUnclaimErrorHandler) UnclaimSpec(_ context.Context, _ *connect.Request[specv1.UnclaimSpecRequest]) (*connect.Response[specv1.UnclaimSpecResponse], error) {
	return nil, connect.NewError(connect.CodeNotFound, nil)
}

// --- happy path tests ---

func TestRunClaim_HappyPath(t *testing.T) {
	startFakeServer[specgraphv1connect.ClaimServiceHandler](t, fakeClaimHandler{}, specgraphv1connect.NewClaimServiceHandler)

	old := claimAgent
	claimAgent = "test-agent"
	t.Cleanup(func() { claimAgent = old })

	oldDur := claimDuration
	claimDuration = 10 * time.Minute
	t.Cleanup(func() { claimDuration = oldDur })

	err := runClaim(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunUnclaim_HappyPath(t *testing.T) {
	startFakeServer[specgraphv1connect.ClaimServiceHandler](t, fakeClaimHandler{}, specgraphv1connect.NewClaimServiceHandler)

	old := unclaimAgent
	unclaimAgent = "test-agent"
	t.Cleanup(func() { unclaimAgent = old })

	err := runUnclaim(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

// --- RPC error tests ---

func TestRunClaim_RPCError(t *testing.T) {
	startFakeServer[specgraphv1connect.ClaimServiceHandler](t, fakeClaimErrorHandler{}, specgraphv1connect.NewClaimServiceHandler)

	old := claimAgent
	claimAgent = "test-agent"
	t.Cleanup(func() { claimAgent = old })

	oldDur := claimDuration
	claimDuration = 10 * time.Minute
	t.Cleanup(func() { claimDuration = oldDur })

	err := runClaim(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "claim spec")
}

func TestRunUnclaim_RPCError(t *testing.T) {
	startFakeServer[specgraphv1connect.ClaimServiceHandler](t, fakeUnclaimErrorHandler{}, specgraphv1connect.NewClaimServiceHandler)

	old := unclaimAgent
	unclaimAgent = "test-agent"
	t.Cleanup(func() { unclaimAgent = old })

	err := runUnclaim(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unclaim spec")
}

// --- cobra arg validation ---

func TestClaimCmd_RequiresSlug(t *testing.T) {
	err := claimCmd.Args(claimCmd, []string{})
	require.Error(t, err)
}

func TestUnclaimCmd_RequiresSlug(t *testing.T) {
	err := unclaimCmd.Args(unclaimCmd, []string{})
	require.Error(t, err)
}
