// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"io"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fake handlers ---

type fakeSliceListHandler struct {
	specgraphv1connect.UnimplementedSliceServiceHandler
}

func (fakeSliceListHandler) ListSlices(_ context.Context, _ *connect.Request[specv1.ListSlicesRequest]) (*connect.Response[specv1.ListSlicesResponse], error) {
	return connect.NewResponse(&specv1.ListSlicesResponse{
		Slices: []*specv1.Slice{
			{Slug: "parent/slice-1", SliceId: "slice-1", Intent: "First", Status: specv1.SliceStatus_SLICE_STATUS_OPEN},
		},
	}), nil
}

type fakeSliceListErrorHandler struct {
	specgraphv1connect.UnimplementedSliceServiceHandler
}

func (fakeSliceListErrorHandler) ListSlices(_ context.Context, _ *connect.Request[specv1.ListSlicesRequest]) (*connect.Response[specv1.ListSlicesResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, nil)
}

type fakeSliceGetHandler struct {
	specgraphv1connect.UnimplementedSliceServiceHandler
}

func (fakeSliceGetHandler) GetSlice(_ context.Context, req *connect.Request[specv1.GetSliceRequest]) (*connect.Response[specv1.GetSliceResponse], error) {
	return connect.NewResponse(&specv1.GetSliceResponse{
		Slice: &specv1.Slice{
			Slug:    req.Msg.GetSlug(),
			SliceId: "slice-1",
			Intent:  "Do the thing",
			Status:  specv1.SliceStatus_SLICE_STATUS_OPEN,
		},
	}), nil
}

type fakeSliceGetErrorHandler struct {
	specgraphv1connect.UnimplementedSliceServiceHandler
}

func (fakeSliceGetErrorHandler) GetSlice(_ context.Context, _ *connect.Request[specv1.GetSliceRequest]) (*connect.Response[specv1.GetSliceResponse], error) {
	return nil, connect.NewError(connect.CodeNotFound, nil)
}

type fakeSliceClaimHandler struct {
	specgraphv1connect.UnimplementedSliceServiceHandler
}

func (fakeSliceClaimHandler) ClaimSlice(_ context.Context, req *connect.Request[specv1.ClaimSliceRequest]) (*connect.Response[specv1.ClaimSliceResponse], error) {
	return connect.NewResponse(&specv1.ClaimSliceResponse{
		Slice: &specv1.Slice{
			Slug:       req.Msg.GetSlug(),
			AssignedTo: req.Msg.GetAssignee(),
			Status:     specv1.SliceStatus_SLICE_STATUS_CLAIMED,
		},
	}), nil
}

type fakeSliceClaimErrorHandler struct {
	specgraphv1connect.UnimplementedSliceServiceHandler
}

func (fakeSliceClaimErrorHandler) ClaimSlice(_ context.Context, _ *connect.Request[specv1.ClaimSliceRequest]) (*connect.Response[specv1.ClaimSliceResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, nil)
}

type fakeSliceCompleteHandler struct {
	specgraphv1connect.UnimplementedSliceServiceHandler
}

func (fakeSliceCompleteHandler) CompleteSlice(_ context.Context, req *connect.Request[specv1.CompleteSliceRequest]) (*connect.Response[specv1.CompleteSliceResponse], error) {
	return connect.NewResponse(&specv1.CompleteSliceResponse{
		Slice: &specv1.Slice{
			Slug:   req.Msg.GetSlug(),
			Status: specv1.SliceStatus_SLICE_STATUS_DONE,
		},
	}), nil
}

type fakeSliceCompleteErrorHandler struct {
	specgraphv1connect.UnimplementedSliceServiceHandler
}

func (fakeSliceCompleteErrorHandler) CompleteSlice(_ context.Context, _ *connect.Request[specv1.CompleteSliceRequest]) (*connect.Response[specv1.CompleteSliceResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, nil)
}

// --- list tests ---

func TestRunSliceList_HappyPath(t *testing.T) {
	startFakeServer[specgraphv1connect.SliceServiceHandler](t, fakeSliceListHandler{}, specgraphv1connect.NewSliceServiceHandler)

	oldJSON := sliceListJSON
	sliceListJSON = false
	t.Cleanup(func() { sliceListJSON = oldJSON })

	err := runSliceList(newCmdWithCtx(), []string{"parent-spec"})
	require.NoError(t, err)
}

func TestRunSliceList_HappyPath_JSON(t *testing.T) {
	startFakeServer[specgraphv1connect.SliceServiceHandler](t, fakeSliceListHandler{}, specgraphv1connect.NewSliceServiceHandler)

	oldJSON := sliceListJSON
	sliceListJSON = true
	t.Cleanup(func() { sliceListJSON = oldJSON })

	cmd := newCmdWithCtx()
	cmd.SetOut(io.Discard)
	err := runSliceList(cmd, []string{"parent-spec"})
	require.NoError(t, err)
}

func TestRunSliceList_RPCError(t *testing.T) {
	startFakeServer[specgraphv1connect.SliceServiceHandler](t, fakeSliceListErrorHandler{}, specgraphv1connect.NewSliceServiceHandler)

	oldJSON := sliceListJSON
	sliceListJSON = false
	t.Cleanup(func() { sliceListJSON = oldJSON })

	err := runSliceList(newCmdWithCtx(), []string{"parent-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list slices")
}

// --- get tests ---

func TestRunSliceGet_HappyPath(t *testing.T) {
	startFakeServer[specgraphv1connect.SliceServiceHandler](t, fakeSliceGetHandler{}, specgraphv1connect.NewSliceServiceHandler)

	oldJSON := sliceGetJSON
	sliceGetJSON = false
	t.Cleanup(func() { sliceGetJSON = oldJSON })

	err := runSliceGet(newCmdWithCtx(), []string{"parent/slice-1"})
	require.NoError(t, err)
}

func TestRunSliceGet_HappyPath_JSON(t *testing.T) {
	startFakeServer[specgraphv1connect.SliceServiceHandler](t, fakeSliceGetHandler{}, specgraphv1connect.NewSliceServiceHandler)

	oldJSON := sliceGetJSON
	sliceGetJSON = true
	t.Cleanup(func() { sliceGetJSON = oldJSON })

	cmd := newCmdWithCtx()
	cmd.SetOut(io.Discard)
	err := runSliceGet(cmd, []string{"parent/slice-1"})
	require.NoError(t, err)
}

func TestRunSliceGet_RPCError(t *testing.T) {
	startFakeServer[specgraphv1connect.SliceServiceHandler](t, fakeSliceGetErrorHandler{}, specgraphv1connect.NewSliceServiceHandler)

	oldJSON := sliceGetJSON
	sliceGetJSON = false
	t.Cleanup(func() { sliceGetJSON = oldJSON })

	err := runSliceGet(newCmdWithCtx(), []string{"parent/slice-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get slice")
}

// --- claim tests ---

func TestRunSliceClaim_HappyPath(t *testing.T) {
	startFakeServer[specgraphv1connect.SliceServiceHandler](t, fakeSliceClaimHandler{}, specgraphv1connect.NewSliceServiceHandler)

	oldAssignee := sliceClaimAssignee
	sliceClaimAssignee = "alice"
	t.Cleanup(func() { sliceClaimAssignee = oldAssignee })

	err := runSliceClaim(newCmdWithCtx(), []string{"parent/slice-1"})
	require.NoError(t, err)
}

func TestRunSliceClaim_RPCError(t *testing.T) {
	startFakeServer[specgraphv1connect.SliceServiceHandler](t, fakeSliceClaimErrorHandler{}, specgraphv1connect.NewSliceServiceHandler)

	oldAssignee := sliceClaimAssignee
	sliceClaimAssignee = "alice"
	t.Cleanup(func() { sliceClaimAssignee = oldAssignee })

	err := runSliceClaim(newCmdWithCtx(), []string{"parent/slice-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "claim slice")
}

// --- complete tests ---

func TestRunSliceComplete_HappyPath(t *testing.T) {
	startFakeServer[specgraphv1connect.SliceServiceHandler](t, fakeSliceCompleteHandler{}, specgraphv1connect.NewSliceServiceHandler)

	err := runSliceComplete(newCmdWithCtx(), []string{"parent/slice-1"})
	require.NoError(t, err)
}

func TestRunSliceComplete_RPCError(t *testing.T) {
	startFakeServer[specgraphv1connect.SliceServiceHandler](t, fakeSliceCompleteErrorHandler{}, specgraphv1connect.NewSliceServiceHandler)

	err := runSliceComplete(newCmdWithCtx(), []string{"parent/slice-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "complete slice")
}

// --- cobra arg validation ---

func TestSliceListCmd_RequiresParentSlug(t *testing.T) {
	err := sliceListCmd.Args(sliceListCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestSliceGetCmd_RequiresSlug(t *testing.T) {
	err := sliceGetCmd.Args(sliceGetCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestSliceClaimCmd_RequiresSlug(t *testing.T) {
	err := sliceClaimCmd.Args(sliceClaimCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestSliceCompleteCmd_RequiresSlug(t *testing.T) {
	err := sliceCompleteCmd.Args(sliceCompleteCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}
