// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSyncBeadsCmd_Flags(t *testing.T) {
	require.NotNil(t, syncBeadsCmd)
	assert.Equal(t, "beads", syncBeadsCmd.Use)

	f := syncBeadsCmd.Flags()
	assert.NotNil(t, f.Lookup("stage"))
	assert.NotNil(t, f.Lookup("priority"))
	assert.NotNil(t, f.Lookup("dry-run"))
}

func TestSyncGitHubCmd_Flags(t *testing.T) {
	require.NotNil(t, syncGitHubCmd)
	assert.Equal(t, "github", syncGitHubCmd.Use)

	f := syncGitHubCmd.Flags()
	assert.NotNil(t, f.Lookup("stage"))
	assert.NotNil(t, f.Lookup("priority"))
	assert.NotNil(t, f.Lookup("dry-run"))
}

func TestSyncStatusCmd_Flags(t *testing.T) {
	require.NotNil(t, syncStatusCmd)
	assert.Equal(t, "status", syncStatusCmd.Use)

	f := syncStatusCmd.Flags()
	assert.NotNil(t, f.Lookup("adapter"))
	assert.NotNil(t, f.Lookup("spec"))
}

// --- fake sync service handler ---

func startFakeSyncServer(t *testing.T, h specgraphv1connect.SyncServiceHandler) {
	t.Helper()
	startFakeServer[specgraphv1connect.SyncServiceHandler](t, h, specgraphv1connect.NewSyncServiceHandler)
}

type fakeSyncBeadsHandler struct {
	specgraphv1connect.UnimplementedSyncServiceHandler
}

func (fakeSyncBeadsHandler) SyncBeads(_ context.Context, _ *connect.Request[specv1.SyncBeadsRequest]) (*connect.Response[specv1.SyncResponse], error) {
	return connect.NewResponse(&specv1.SyncResponse{
		Synced:  1,
		Skipped: 0,
		Results: []*specv1.SyncResult{
			{SpecSlug: "my-spec", State: specv1.SyncState_SYNC_STATE_SYNCED, ExternalId: "beads-123"},
		},
	}), nil
}

func TestRunSyncBeads_HappyPath(t *testing.T) {
	startFakeSyncServer(t, fakeSyncBeadsHandler{})
	old := beadsDryRun
	beadsDryRun = false
	t.Cleanup(func() { beadsDryRun = old })

	err := runSyncBeads(newCmdWithCtx())
	require.NoError(t, err)
}

func TestRunSyncBeads_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runSyncBeads(newCmdWithCtx())
	require.Error(t, err)
}

type fakeSyncGitHubHandler struct {
	specgraphv1connect.UnimplementedSyncServiceHandler
}

func (fakeSyncGitHubHandler) SyncGitHub(_ context.Context, _ *connect.Request[specv1.SyncGitHubRequest]) (*connect.Response[specv1.SyncResponse], error) {
	return connect.NewResponse(&specv1.SyncResponse{
		Synced: 2,
		Results: []*specv1.SyncResult{
			{SpecSlug: "spec-a", State: specv1.SyncState_SYNC_STATE_SYNCED, ExternalId: "gh-1"},
			{SpecSlug: "spec-b", State: specv1.SyncState_SYNC_STATE_PENDING, Message: "rate limited"},
		},
	}), nil
}

func TestRunSyncGitHub_HappyPath(t *testing.T) {
	startFakeSyncServer(t, fakeSyncGitHubHandler{})
	err := runSyncGitHub(newCmdWithCtx())
	require.NoError(t, err)
}

func TestRunSyncGitHub_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runSyncGitHub(newCmdWithCtx())
	require.Error(t, err)
}

type fakeSyncStatusHandler struct {
	specgraphv1connect.UnimplementedSyncServiceHandler
}

func (fakeSyncStatusHandler) GetSyncStatus(_ context.Context, _ *connect.Request[specv1.SyncStatusRequest]) (*connect.Response[specv1.SyncStatusResponse], error) {
	return connect.NewResponse(&specv1.SyncStatusResponse{
		Mappings: []*specv1.SyncMapping{
			{
				SpecSlug:   "my-spec",
				Adapter:    specv1.SyncAdapter_SYNC_ADAPTER_BEADS,
				ExternalId: "beads-1",
				State:      specv1.SyncState_SYNC_STATE_SYNCED,
				LastSync:   timestamppb.Now(),
			},
		},
	}), nil
}

func TestRunSyncStatus_HappyPath(t *testing.T) {
	startFakeSyncServer(t, fakeSyncStatusHandler{})
	old := statusAdapter
	statusAdapter = ""
	t.Cleanup(func() { statusAdapter = old })

	err := runSyncStatus(newCmdWithCtx(), nil)
	require.NoError(t, err)
}

func TestRunSyncStatus_WithAdapterFilter(t *testing.T) {
	startFakeSyncServer(t, fakeSyncStatusHandler{})
	old := statusAdapter
	statusAdapter = "beads"
	t.Cleanup(func() { statusAdapter = old })

	err := runSyncStatus(newCmdWithCtx(), nil)
	require.NoError(t, err)
}

func TestRunSyncStatus_InvalidAdapter(t *testing.T) {
	startFakeSyncServer(t, fakeSyncStatusHandler{})
	old := statusAdapter
	statusAdapter = "bogus"
	t.Cleanup(func() { statusAdapter = old })

	err := runSyncStatus(newCmdWithCtx(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported adapter")
}

type fakeSyncStatusEmptyHandler struct {
	specgraphv1connect.UnimplementedSyncServiceHandler
}

func (fakeSyncStatusEmptyHandler) GetSyncStatus(_ context.Context, _ *connect.Request[specv1.SyncStatusRequest]) (*connect.Response[specv1.SyncStatusResponse], error) {
	return connect.NewResponse(&specv1.SyncStatusResponse{Mappings: nil}), nil
}

func TestRunSyncStatus_Empty(t *testing.T) {
	startFakeSyncServer(t, fakeSyncStatusEmptyHandler{})
	old := statusAdapter
	statusAdapter = ""
	t.Cleanup(func() { statusAdapter = old })

	err := runSyncStatus(newCmdWithCtx(), nil)
	require.NoError(t, err)
}

func TestRunSyncStatus_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runSyncStatus(newCmdWithCtx(), nil)
	require.Error(t, err)
}

// --- sync RPC error tests ---

type fakeSyncBeadsErrHandler struct {
	specgraphv1connect.UnimplementedSyncServiceHandler
}

func (fakeSyncBeadsErrHandler) SyncBeads(_ context.Context, _ *connect.Request[specv1.SyncBeadsRequest]) (*connect.Response[specv1.SyncResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("sync failed"))
}

func TestRunSyncBeads_RPCError(t *testing.T) {
	startFakeSyncServer(t, fakeSyncBeadsErrHandler{})
	err := runSyncBeads(newCmdWithCtx())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sync beads")
}

type fakeSyncGitHubErrHandler struct {
	specgraphv1connect.UnimplementedSyncServiceHandler
}

func (fakeSyncGitHubErrHandler) SyncGitHub(_ context.Context, _ *connect.Request[specv1.SyncGitHubRequest]) (*connect.Response[specv1.SyncResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("github sync failed"))
}

func TestRunSyncGitHub_RPCError(t *testing.T) {
	startFakeSyncServer(t, fakeSyncGitHubErrHandler{})
	err := runSyncGitHub(newCmdWithCtx())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sync github")
}
