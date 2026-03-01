// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/storage"
)

// DecisionHandler implements the ConnectRPC DecisionService.
type DecisionHandler struct {
	store storage.DecisionBackend
}

var _ specgraphv1connect.DecisionServiceHandler = (*DecisionHandler)(nil)

// CreateDecision handles the CreateDecision RPC.
func (h *DecisionHandler) CreateDecision(ctx context.Context, req *connect.Request[specv1.CreateDecisionRequest]) (*connect.Response[specv1.Decision], error) {
	msg := req.Msg
	d, err := h.store.CreateDecision(ctx, msg.Slug, msg.Title, msg.Decision, msg.Rationale)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(d), nil
}

// GetDecision handles the GetDecision RPC.
func (h *DecisionHandler) GetDecision(ctx context.Context, req *connect.Request[specv1.GetDecisionRequest]) (*connect.Response[specv1.Decision], error) {
	d, err := h.store.GetDecision(ctx, req.Msg.Slug)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(d), nil
}

// ListDecisions handles the ListDecisions RPC.
func (h *DecisionHandler) ListDecisions(ctx context.Context, req *connect.Request[specv1.ListDecisionsRequest]) (*connect.Response[specv1.ListDecisionsResponse], error) {
	msg := req.Msg
	limit := int(msg.Limit)
	if limit == 0 {
		limit = defaultListLimit
	}

	decisions, err := h.store.ListDecisions(ctx, msg.GetStatus(), limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.ListDecisionsResponse{Decisions: decisions}), nil
}

// UpdateDecision handles the UpdateDecision RPC.
func (h *DecisionHandler) UpdateDecision(ctx context.Context, req *connect.Request[specv1.UpdateDecisionRequest]) (*connect.Response[specv1.Decision], error) {
	msg := req.Msg
	if msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}

	d, err := h.store.UpdateDecision(ctx, msg.Slug, msg.Title, msg.Status, msg.Decision, msg.Rationale, msg.SupersededBy)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(d), nil
}

// RegisterDecisionService registers the DecisionService on the given mux.
func RegisterDecisionService(mux *http.ServeMux, store storage.DecisionBackend) {
	handler := &DecisionHandler{store: store}
	path, h := specgraphv1connect.NewDecisionServiceHandler(handler)
	mux.Handle(path, h)
}
