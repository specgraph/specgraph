// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/storage"
)

const (
	defaultListLimit      = 50
	defaultSpecPriority   = "p2"
	defaultSpecComplexity = "medium"
)

// SpecHandler implements the ConnectRPC SpecService using a storage backend.
type SpecHandler struct {
	backend storage.Backend
}

var _ specgraphv1connect.SpecServiceHandler = (*SpecHandler)(nil)

// NewSpecHandler creates a SpecHandler backed by the given storage.Backend.
func NewSpecHandler(backend storage.Backend) *SpecHandler {
	return &SpecHandler{backend: backend}
}

// CreateSpec handles the CreateSpec RPC.
func (h *SpecHandler) CreateSpec(ctx context.Context, req *connect.Request[specv1.CreateSpecRequest]) (*connect.Response[specv1.Spec], error) {
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	priority := msg.Priority
	if priority == "" {
		priority = defaultSpecPriority
	}
	complexity := msg.Complexity
	if complexity == "" {
		complexity = defaultSpecComplexity
	}

	spec, err := h.backend.CreateSpec(ctx, msg.Slug, msg.Intent, priority, complexity)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(specToProto(spec)), nil
}

// GetSpec handles the GetSpec RPC.
func (h *SpecHandler) GetSpec(ctx context.Context, req *connect.Request[specv1.GetSpecRequest]) (*connect.Response[specv1.Spec], error) {
	if err := validateSlug(req.Msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	spec, err := h.backend.GetSpec(ctx, req.Msg.Slug)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(specToProto(spec)), nil
}

// ListSpecs handles the ListSpecs RPC.
func (h *SpecHandler) ListSpecs(ctx context.Context, req *connect.Request[specv1.ListSpecsRequest]) (*connect.Response[specv1.ListSpecsResponse], error) {
	msg := req.Msg
	limit := int(msg.Limit)
	if limit == 0 {
		limit = defaultListLimit
	}

	specs, err := h.backend.ListSpecs(ctx, msg.Stage, msg.Priority, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.ListSpecsResponse{Specs: specsToProto(specs)}), nil
}

// UpdateSpec handles the UpdateSpec RPC.
func (h *SpecHandler) UpdateSpec(ctx context.Context, req *connect.Request[specv1.UpdateSpecRequest]) (*connect.Response[specv1.Spec], error) {
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	spec, err := h.backend.UpdateSpec(ctx, msg.Slug, msg.Intent, msg.Stage, msg.Priority, msg.Complexity)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(specToProto(spec)), nil
}
