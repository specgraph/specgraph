// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/storage"
)

// GraphHandler implements the ConnectRPC GraphService.
type GraphHandler struct {
	scoper storage.Scoper
}

var _ specgraphv1connect.GraphServiceHandler = (*GraphHandler)(nil)

// AddEdge handles the AddEdge RPC.
func (h *GraphHandler) AddEdge(ctx context.Context, req *connect.Request[specv1.AddEdgeRequest]) (*connect.Response[specv1.AddEdgeResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.FromSlug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("from_slug: %w", err))
	}
	if err := validateSlug(req.Msg.ToSlug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("to_slug: %w", err))
	}
	et, err := edgeTypeFromProto(req.Msg.EdgeType)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	edge, err := store.AddEdge(ctx, req.Msg.FromSlug, req.Msg.ToSlug, et)
	if err != nil {
		return nil, graphError(err)
	}
	pb, err := edgeToProto(edge)
	if err != nil {
		return nil, graphError(err)
	}
	return connect.NewResponse(&specv1.AddEdgeResponse{Edge: pb}), nil
}

// RemoveEdge handles the RemoveEdge RPC.
func (h *GraphHandler) RemoveEdge(ctx context.Context, req *connect.Request[specv1.RemoveEdgeRequest]) (*connect.Response[specv1.RemoveEdgeResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.FromSlug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("from_slug: %w", err))
	}
	if err := validateSlug(req.Msg.ToSlug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("to_slug: %w", err))
	}
	et, err := edgeTypeFromProto(req.Msg.EdgeType)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := store.RemoveEdge(ctx, req.Msg.FromSlug, req.Msg.ToSlug, et); err != nil {
		return nil, graphError(err)
	}
	return connect.NewResponse(&specv1.RemoveEdgeResponse{}), nil
}

// ListEdges handles the ListEdges RPC.
func (h *GraphHandler) ListEdges(ctx context.Context, req *connect.Request[specv1.ListEdgesRequest]) (*connect.Response[specv1.ListEdgesResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// UNSPECIFIED means "list all edge types" — pass empty string.
	var et storage.EdgeType
	if req.Msg.EdgeType != specv1.EdgeType_EDGE_TYPE_UNSPECIFIED {
		var err error
		et, err = edgeTypeFromProto(req.Msg.EdgeType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}
	edges, err := store.ListEdges(ctx, req.Msg.Slug, et)
	if err != nil {
		return nil, graphError(err)
	}
	pbs, err := edgesToProto(edges)
	if err != nil {
		return nil, graphError(err)
	}
	return connect.NewResponse(&specv1.ListEdgesResponse{Edges: pbs}), nil
}

// GetDependencies handles the GetDependencies RPC.
func (h *GraphHandler) GetDependencies(ctx context.Context, req *connect.Request[specv1.GetDependenciesRequest]) (*connect.Response[specv1.GetDependenciesResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	refs, err := store.GetDependencies(ctx, req.Msg.Slug)
	if err != nil {
		return nil, graphError(err)
	}
	return connect.NewResponse(&specv1.GetDependenciesResponse{Dependencies: nodeRefsToProto(refs)}), nil
}

// GetTransitiveDeps handles the GetTransitiveDeps RPC.
func (h *GraphHandler) GetTransitiveDeps(ctx context.Context, req *connect.Request[specv1.GetTransitiveDepsRequest]) (*connect.Response[specv1.GetTransitiveDepsResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	refs, err := store.GetTransitiveDeps(ctx, req.Msg.Slug)
	if err != nil {
		return nil, graphError(err)
	}
	return connect.NewResponse(&specv1.GetTransitiveDepsResponse{Dependencies: nodeRefsToProto(refs)}), nil
}

// GetImpact handles the GetImpact RPC.
func (h *GraphHandler) GetImpact(ctx context.Context, req *connect.Request[specv1.GetImpactRequest]) (*connect.Response[specv1.GetImpactResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	refs, err := store.GetImpact(ctx, req.Msg.Slug)
	if err != nil {
		return nil, graphError(err)
	}
	return connect.NewResponse(&specv1.GetImpactResponse{Impacted: nodeRefsToProto(refs)}), nil
}

// GetReady handles the GetReady RPC.
func (h *GraphHandler) GetReady(ctx context.Context, _ *connect.Request[specv1.GetReadyRequest]) (*connect.Response[specv1.GetReadyResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	refs, err := store.GetReady(ctx)
	if err != nil {
		return nil, graphError(err)
	}
	return connect.NewResponse(&specv1.GetReadyResponse{Ready: nodeRefsToProto(refs)}), nil
}

// GetCriticalPath handles the GetCriticalPath RPC.
func (h *GraphHandler) GetCriticalPath(ctx context.Context, req *connect.Request[specv1.GetCriticalPathRequest]) (*connect.Response[specv1.GetCriticalPathResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	refs, err := store.GetCriticalPath(ctx, req.Msg.Slug)
	if err != nil {
		return nil, graphError(err)
	}
	return connect.NewResponse(&specv1.GetCriticalPathResponse{Path: nodeRefsToProto(refs)}), nil
}

// GetFullGraph handles the GetFullGraph RPC.
func (h *GraphHandler) GetFullGraph(ctx context.Context, _ *connect.Request[specv1.GetFullGraphRequest]) (*connect.Response[specv1.GetFullGraphResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	graph, err := store.GetFullGraph(ctx)
	if err != nil {
		return nil, graphError(err)
	}
	pbs, err := edgesToProto(graph.Edges)
	if err != nil {
		return nil, graphError(err)
	}
	return connect.NewResponse(&specv1.GetFullGraphResponse{
		Nodes: graphNodesToProto(graph.Nodes),
		Edges: pbs,
	}), nil
}

// graphError maps storage/conversion errors to sanitized connect error codes.
func graphError(err error) error {
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		return connErr
	}
	switch {
	case errors.Is(err, storage.ErrSpecNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("spec not found"))
	case errors.Is(err, storage.ErrEdgeNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("edge not found"))
	default:
		slog.Error("graphError: internal error", slog.Any("error", err))
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}

// RegisterGraphService registers the GraphService on the given mux.
func RegisterGraphService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption) {
	handler := &GraphHandler{scoper: scoper}
	path, h := specgraphv1connect.NewGraphServiceHandler(handler, opts...)
	mux.Handle(path, h)
}

func graphNodesToProto(nodes []storage.GraphNode) []*specv1.GraphNode {
	result := make([]*specv1.GraphNode, len(nodes))
	for i, n := range nodes {
		result[i] = &specv1.GraphNode{
			Slug:     n.Slug,
			Label:    string(n.Label),
			Stage:    n.Stage,
			Intent:   n.Intent,
			Priority: n.Priority,
		}
	}
	return result
}

func nodeRefsToProto(refs []storage.NodeRef) []*specv1.NodeRef {
	result := make([]*specv1.NodeRef, len(refs))
	for i, r := range refs {
		result[i] = &specv1.NodeRef{
			Id:    r.ID,
			Slug:  r.Slug,
			Label: string(r.Label),
			Stage: r.Stage,
		}
	}
	return result
}
