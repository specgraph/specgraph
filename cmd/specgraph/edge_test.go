// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- map tests ---

func TestEdgeTypeMap_Completeness(t *testing.T) {
	expected := []string{"depends_on", "blocks", "composes", "relates_to", "informs"}
	for _, name := range expected {
		_, ok := edgeTypeMap[name]
		assert.True(t, ok, "expected %q in edgeTypeMap", name)
	}
	assert.Len(t, edgeTypeMap, len(expected), "edgeTypeMap has unexpected entries")
}

func TestEdgeTypeMap_AllEntriesAreValidProto(t *testing.T) {
	for name, et := range edgeTypeMap {
		_, ok := specv1.EdgeType_name[int32(et)]
		assert.True(t, ok, "edgeTypeMap[%q] = %d is not a valid EdgeType enum value", name, et)
		assert.NotEqual(t, specv1.EdgeType_EDGE_TYPE_UNSPECIFIED, et,
			"edgeTypeMap[%q] should not map to UNSPECIFIED", name)
	}
}

// --- fake handlers ---

type fakeEdgeAddHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeEdgeAddHandler) AddEdge(_ context.Context, req *connect.Request[specv1.AddEdgeRequest]) (*connect.Response[specv1.AddEdgeResponse], error) {
	return connect.NewResponse(&specv1.AddEdgeResponse{
		Edge: &specv1.Edge{
			FromId:   req.Msg.GetFromSlug(),
			ToId:     req.Msg.GetToSlug(),
			EdgeType: req.Msg.GetEdgeType(),
		},
	}), nil
}

type fakeEdgeRemoveHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeEdgeRemoveHandler) RemoveEdge(_ context.Context, _ *connect.Request[specv1.RemoveEdgeRequest]) (*connect.Response[specv1.RemoveEdgeResponse], error) {
	return connect.NewResponse(&specv1.RemoveEdgeResponse{}), nil
}

type fakeEdgeListHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeEdgeListHandler) ListEdges(_ context.Context, _ *connect.Request[specv1.ListEdgesRequest]) (*connect.Response[specv1.ListEdgesResponse], error) {
	return connect.NewResponse(&specv1.ListEdgesResponse{
		Edges: []*specv1.Edge{
			{FromId: "spec-a", ToId: "spec-b", EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON},
			{FromId: "spec-a", ToId: "spec-c", EdgeType: specv1.EdgeType_EDGE_TYPE_COMPOSES},
		},
	}), nil
}

type fakeEdgeAddErrorHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeEdgeAddErrorHandler) AddEdge(_ context.Context, _ *connect.Request[specv1.AddEdgeRequest]) (*connect.Response[specv1.AddEdgeResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, nil)
}

type fakeEdgeRemoveErrorHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeEdgeRemoveErrorHandler) RemoveEdge(_ context.Context, _ *connect.Request[specv1.RemoveEdgeRequest]) (*connect.Response[specv1.RemoveEdgeResponse], error) {
	return nil, connect.NewError(connect.CodeNotFound, nil)
}

type fakeEdgeListErrorHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeEdgeListErrorHandler) ListEdges(_ context.Context, _ *connect.Request[specv1.ListEdgesRequest]) (*connect.Response[specv1.ListEdgesResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, nil)
}

// --- happy path tests ---

func TestRunEdgeAdd_HappyPath(t *testing.T) {
	startFakeGraphServer(t, fakeEdgeAddHandler{})

	old := edgeAddType
	edgeAddType = "depends_on"
	t.Cleanup(func() { edgeAddType = old })

	err := runEdgeAdd(newCmdWithCtx(), []string{"spec-a", "spec-b"})
	require.NoError(t, err)
}

func TestRunEdgeRemove_HappyPath(t *testing.T) {
	startFakeGraphServer(t, fakeEdgeRemoveHandler{})

	old := edgeRemoveType
	edgeRemoveType = "blocks"
	t.Cleanup(func() { edgeRemoveType = old })

	err := runEdgeRemove(newCmdWithCtx(), []string{"spec-a", "spec-b"})
	require.NoError(t, err)
}

func TestRunEdgeList_HappyPath(t *testing.T) {
	startFakeGraphServer(t, fakeEdgeListHandler{})

	old := edgeListType
	edgeListType = ""
	t.Cleanup(func() { edgeListType = old })

	oldJSON := edgeListJSON
	edgeListJSON = false
	t.Cleanup(func() { edgeListJSON = oldJSON })

	err := runEdgeList(newCmdWithCtx(), []string{"spec-a"})
	require.NoError(t, err)
}

func TestRunEdgeList_HappyPath_JSON(t *testing.T) {
	startFakeGraphServer(t, fakeEdgeListHandler{})

	old := edgeListType
	edgeListType = ""
	t.Cleanup(func() { edgeListType = old })

	oldJSON := edgeListJSON
	edgeListJSON = true
	t.Cleanup(func() { edgeListJSON = oldJSON })

	err := runEdgeList(newCmdWithCtx(), []string{"spec-a"})
	require.NoError(t, err)
}

func TestRunEdgeList_WithTypeFilter(t *testing.T) {
	startFakeGraphServer(t, fakeEdgeListHandler{})

	old := edgeListType
	edgeListType = "composes"
	t.Cleanup(func() { edgeListType = old })

	oldJSON := edgeListJSON
	edgeListJSON = false
	t.Cleanup(func() { edgeListJSON = oldJSON })

	err := runEdgeList(newCmdWithCtx(), []string{"spec-a"})
	require.NoError(t, err)
}

// --- negative path: unknown type ---

func TestRunEdgeAdd_UnknownType_WithServer(t *testing.T) {
	startFakeGraphServer(t, fakeEdgeAddHandler{})

	old := edgeAddType
	edgeAddType = "invalid"
	t.Cleanup(func() { edgeAddType = old })

	err := runEdgeAdd(newCmdWithCtx(), []string{"spec-a", "spec-b"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown edge type")
}

func TestRunEdgeRemove_UnknownType_WithServer(t *testing.T) {
	startFakeGraphServer(t, fakeEdgeRemoveHandler{})

	old := edgeRemoveType
	edgeRemoveType = "invalid"
	t.Cleanup(func() { edgeRemoveType = old })

	err := runEdgeRemove(newCmdWithCtx(), []string{"spec-a", "spec-b"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown edge type")
}

func TestRunEdgeList_UnknownType_WithServer(t *testing.T) {
	startFakeGraphServer(t, fakeEdgeListHandler{})

	old := edgeListType
	edgeListType = "invalid"
	t.Cleanup(func() { edgeListType = old })

	err := runEdgeList(newCmdWithCtx(), []string{"spec-a"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown edge type")
}

// --- negative path: RPC error ---

func TestRunEdgeAdd_RPCError(t *testing.T) {
	startFakeGraphServer(t, fakeEdgeAddErrorHandler{})

	old := edgeAddType
	edgeAddType = "depends_on"
	t.Cleanup(func() { edgeAddType = old })

	err := runEdgeAdd(newCmdWithCtx(), []string{"spec-a", "spec-b"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "add edge")
}

func TestRunEdgeRemove_RPCError(t *testing.T) {
	startFakeGraphServer(t, fakeEdgeRemoveErrorHandler{})

	old := edgeRemoveType
	edgeRemoveType = "blocks"
	t.Cleanup(func() { edgeRemoveType = old })

	err := runEdgeRemove(newCmdWithCtx(), []string{"spec-a", "spec-b"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove edge")
}

func TestRunEdgeList_RPCError(t *testing.T) {
	startFakeGraphServer(t, fakeEdgeListErrorHandler{})

	old := edgeListType
	edgeListType = ""
	t.Cleanup(func() { edgeListType = old })

	err := runEdgeList(newCmdWithCtx(), []string{"spec-a"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list edges")
}

// --- cobra arg validation ---

func TestEdgeAddCmd_RequiresTwoArgs(t *testing.T) {
	err := edgeAddCmd.Args(edgeAddCmd, []string{})
	require.Error(t, err)

	err = edgeAddCmd.Args(edgeAddCmd, []string{"only-one"})
	require.Error(t, err)
}

func TestEdgeRemoveCmd_RequiresTwoArgs(t *testing.T) {
	err := edgeRemoveCmd.Args(edgeRemoveCmd, []string{})
	require.Error(t, err)

	err = edgeRemoveCmd.Args(edgeRemoveCmd, []string{"only-one"})
	require.Error(t, err)
}

func TestEdgeListCmd_RequiresSlug(t *testing.T) {
	err := edgeListCmd.Args(edgeListCmd, []string{})
	require.Error(t, err)
}
