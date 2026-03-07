// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/storage"
)

// GraphHandler implements the ConnectRPC GraphService.
type GraphHandler struct {
	store storage.GraphBackend
}

var _ specgraphv1connect.GraphServiceHandler = (*GraphHandler)(nil)

// AddEdge handles the AddEdge RPC.
func (h *GraphHandler) AddEdge(ctx context.Context, req *connect.Request[specv1.AddEdgeRequest]) (*connect.Response[specv1.Edge], error) {
	et, err := edgeTypeFromProto(req.Msg.EdgeType)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	edge, err := h.store.AddEdge(ctx, req.Msg.FromSlug, req.Msg.ToSlug, et)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(edgeToProto(edge)), nil
}

// RemoveEdge handles the RemoveEdge RPC.
func (h *GraphHandler) RemoveEdge(ctx context.Context, req *connect.Request[specv1.RemoveEdgeRequest]) (*connect.Response[specv1.RemoveEdgeResponse], error) {
	et, err := edgeTypeFromProto(req.Msg.EdgeType)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.store.RemoveEdge(ctx, req.Msg.FromSlug, req.Msg.ToSlug, et); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.RemoveEdgeResponse{}), nil
}

// ListEdges handles the ListEdges RPC.
func (h *GraphHandler) ListEdges(ctx context.Context, req *connect.Request[specv1.ListEdgesRequest]) (*connect.Response[specv1.ListEdgesResponse], error) {
	// UNSPECIFIED means "list all edge types" — pass empty string.
	var et storage.EdgeType
	if req.Msg.EdgeType != specv1.EdgeType_EDGE_TYPE_UNSPECIFIED {
		var err error
		et, err = edgeTypeFromProto(req.Msg.EdgeType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}
	edges, err := h.store.ListEdges(ctx, req.Msg.Slug, et)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.ListEdgesResponse{Edges: edgesToProto(edges)}), nil
}

// GetDependencies handles the GetDependencies RPC.
func (h *GraphHandler) GetDependencies(ctx context.Context, req *connect.Request[specv1.GetDependenciesRequest]) (*connect.Response[specv1.GetDependenciesResponse], error) {
	refs, err := h.store.GetDependencies(ctx, req.Msg.Slug)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.GetDependenciesResponse{Dependencies: nodeRefsToProto(refs)}), nil
}

// GetTransitiveDeps handles the GetTransitiveDeps RPC.
func (h *GraphHandler) GetTransitiveDeps(ctx context.Context, req *connect.Request[specv1.GetTransitiveDepsRequest]) (*connect.Response[specv1.GetTransitiveDepsResponse], error) {
	refs, err := h.store.GetTransitiveDeps(ctx, req.Msg.Slug)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.GetTransitiveDepsResponse{Dependencies: nodeRefsToProto(refs)}), nil
}

// GetImpact handles the GetImpact RPC.
func (h *GraphHandler) GetImpact(ctx context.Context, req *connect.Request[specv1.GetImpactRequest]) (*connect.Response[specv1.GetImpactResponse], error) {
	refs, err := h.store.GetImpact(ctx, req.Msg.Slug)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.GetImpactResponse{Impacted: nodeRefsToProto(refs)}), nil
}

// GetReady handles the GetReady RPC.
func (h *GraphHandler) GetReady(ctx context.Context, _ *connect.Request[specv1.GetReadyRequest]) (*connect.Response[specv1.GetReadyResponse], error) {
	refs, err := h.store.GetReady(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.GetReadyResponse{Ready: nodeRefsToProto(refs)}), nil
}

// GetCriticalPath handles the GetCriticalPath RPC.
func (h *GraphHandler) GetCriticalPath(ctx context.Context, req *connect.Request[specv1.GetCriticalPathRequest]) (*connect.Response[specv1.GetCriticalPathResponse], error) {
	refs, err := h.store.GetCriticalPath(ctx, req.Msg.Slug)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.GetCriticalPathResponse{Path: nodeRefsToProto(refs)}), nil
}

// RegisterGraphService registers the GraphService on the given mux.
func RegisterGraphService(mux *http.ServeMux, store storage.GraphBackend) {
	handler := &GraphHandler{store: store}
	path, h := specgraphv1connect.NewGraphServiceHandler(handler)
	mux.Handle(path, h)
}

func nodeRefsToProto(refs []storage.NodeRef) []*specv1.NodeRef {
	result := make([]*specv1.NodeRef, len(refs))
	for i, r := range refs {
		result[i] = &specv1.NodeRef{
			Id:    r.ID,
			Slug:  r.Slug,
			Label: r.Label,
			Stage: r.Stage,
		}
	}
	return result
}
