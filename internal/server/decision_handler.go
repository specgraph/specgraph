// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
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
	if err := validateSlug(msg.GetSlug()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	d, err := h.store.CreateDecision(ctx, msg.Slug, msg.Title, msg.Decision, msg.Rationale)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pb, err := decisionToProto(d)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(pb), nil
}

// GetDecision handles the GetDecision RPC.
func (h *DecisionHandler) GetDecision(ctx context.Context, req *connect.Request[specv1.GetDecisionRequest]) (*connect.Response[specv1.Decision], error) {
	if err := validateSlug(req.Msg.GetSlug()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	d, err := h.store.GetDecision(ctx, req.Msg.Slug)
	if err != nil {
		if errors.Is(err, storage.ErrDecisionNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pb, err := decisionToProto(d)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(pb), nil
}

// ListDecisions handles the ListDecisions RPC.
func (h *DecisionHandler) ListDecisions(ctx context.Context, req *connect.Request[specv1.ListDecisionsRequest]) (*connect.Response[specv1.ListDecisionsResponse], error) {
	msg := req.Msg
	limit := int(msg.Limit)
	if limit == 0 {
		limit = defaultListLimit
	}

	// UNSPECIFIED means "list all" — pass empty string to skip filtering.
	var status storage.DecisionStatus
	if msg.GetStatus() != specv1.DecisionStatus_DECISION_STATUS_UNSPECIFIED {
		s, err := decisionStatusFromProto(msg.GetStatus())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		status = s
	}
	decisions, err := h.store.ListDecisions(ctx, status, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pbs, err := decisionsToProto(decisions)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.ListDecisionsResponse{Decisions: pbs}), nil
}

// UpdateDecision handles the UpdateDecision RPC.
func (h *DecisionHandler) UpdateDecision(ctx context.Context, req *connect.Request[specv1.UpdateDecisionRequest]) (*connect.Response[specv1.Decision], error) {
	msg := req.Msg
	if err := validateSlug(msg.GetSlug()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	var domainStatus *storage.DecisionStatus
	if msg.Status != nil {
		s, err := decisionStatusFromProto(*msg.Status)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		domainStatus = &s
	}
	d, err := h.store.UpdateDecision(ctx, msg.Slug, msg.Title, domainStatus, msg.Decision, msg.Rationale, msg.SupersededBy)
	if err != nil {
		if errors.Is(err, storage.ErrDecisionNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if errors.Is(err, storage.ErrSupersededByRequired) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	pb, err := decisionToProto(d)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(pb), nil
}

// RegisterDecisionService registers the DecisionService on the given mux.
func RegisterDecisionService(mux *http.ServeMux, store storage.DecisionBackend) {
	handler := &DecisionHandler{store: store}
	path, h := specgraphv1connect.NewDecisionServiceHandler(handler)
	mux.Handle(path, h)
}
