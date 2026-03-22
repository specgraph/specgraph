// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/storage"
)

const (
	defaultListLimit      = 50
	defaultSpecPriority   = "p2"
	defaultSpecComplexity = "medium"
)

// SpecHandler implements the ConnectRPC SpecService using a storage backend.
type SpecHandler struct {
	scoper storage.Scoper
}

var _ specgraphv1connect.SpecServiceHandler = (*SpecHandler)(nil)

// NewSpecHandler creates a SpecHandler backed by the given storage.Scoper.
func NewSpecHandler(scoper storage.Scoper) *SpecHandler {
	return &SpecHandler{scoper: scoper}
}

// CreateSpec handles the CreateSpec RPC.
func (h *SpecHandler) CreateSpec(ctx context.Context, req *connect.Request[specv1.CreateSpecRequest]) (*connect.Response[specv1.CreateSpecResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
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

	spec, err := store.CreateSpec(ctx, msg.Slug, msg.Intent, priority, complexity)
	if err != nil {
		if errors.Is(err, storage.ErrSpecAlreadyExists) {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("spec with this slug already exists"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pb, err := specToProto(spec)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.CreateSpecResponse{Spec: pb}), nil
}

// GetSpec handles the GetSpec RPC.
func (h *SpecHandler) GetSpec(ctx context.Context, req *connect.Request[specv1.GetSpecRequest]) (*connect.Response[specv1.GetSpecResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	spec, err := store.GetSpec(ctx, req.Msg.Slug)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pb, err := specToProto(spec)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.GetSpecResponse{Spec: pb}), nil
}

// ListSpecs handles the ListSpecs RPC.
func (h *SpecHandler) ListSpecs(ctx context.Context, req *connect.Request[specv1.ListSpecsRequest]) (*connect.Response[specv1.ListSpecsResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	limit := int(msg.Limit)
	if limit == 0 {
		limit = defaultListLimit
	}

	specs, err := store.ListSpecs(ctx, msg.Stage, msg.Priority, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pbs, err := specsToProto(specs)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.ListSpecsResponse{Specs: pbs}), nil
}

// UpdateSpec handles the UpdateSpec RPC.
func (h *SpecHandler) UpdateSpec(ctx context.Context, req *connect.Request[specv1.UpdateSpecRequest]) (*connect.Response[specv1.UpdateSpecResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.Notes != nil && utf8.RuneCountInString(*msg.Notes) > maxNotesLen {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("notes exceeds maximum length of %d characters", maxNotesLen))
	}

	spec, err := store.UpdateSpec(ctx, msg.Slug, msg.Intent, msg.Stage, msg.Priority, msg.Complexity, msg.Notes)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pb, err := specToProto(spec)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.UpdateSpecResponse{Spec: pb}), nil
}
