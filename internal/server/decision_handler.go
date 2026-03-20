// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/storage"
)

// DecisionHandler implements the ConnectRPC DecisionService.
type DecisionHandler struct {
	scoper storage.Scoper
	logger *slog.Logger
}

var _ specgraphv1connect.DecisionServiceHandler = (*DecisionHandler)(nil)

// CreateDecision handles the CreateDecision RPC.
func (h *DecisionHandler) CreateDecision(ctx context.Context, req *connect.Request[specv1.CreateDecisionRequest]) (*connect.Response[specv1.Decision], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	if err := validateSlug(msg.GetSlug()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	d, err := store.CreateDecision(ctx, msg.Slug, msg.Title, msg.Decision, msg.Rationale)
	if err != nil {
		h.logger.ErrorContext(ctx, "create decision failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	pb, err := decisionToProto(d)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(pb), nil
}

// GetDecision handles the GetDecision RPC.
func (h *DecisionHandler) GetDecision(ctx context.Context, req *connect.Request[specv1.GetDecisionRequest]) (*connect.Response[specv1.Decision], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.GetSlug()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	d, err := store.GetDecision(ctx, req.Msg.Slug)
	if err != nil {
		if errors.Is(err, storage.ErrDecisionNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("decision not found"))
		}
		h.logger.ErrorContext(ctx, "get decision failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	pb, err := decisionToProto(d)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(pb), nil
}

// ListDecisions handles the ListDecisions RPC.
func (h *DecisionHandler) ListDecisions(ctx context.Context, req *connect.Request[specv1.ListDecisionsRequest]) (*connect.Response[specv1.ListDecisionsResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
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
	decisions, err := store.ListDecisions(ctx, status, limit)
	if err != nil {
		h.logger.ErrorContext(ctx, "list decisions failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	pbs, err := decisionsToProto(decisions)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.ListDecisionsResponse{Decisions: pbs}), nil
}

// UpdateDecision handles the UpdateDecision RPC.
func (h *DecisionHandler) UpdateDecision(ctx context.Context, req *connect.Request[specv1.UpdateDecisionRequest]) (*connect.Response[specv1.Decision], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
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
	d, err := store.UpdateDecision(ctx, msg.Slug, msg.Title, domainStatus, msg.Decision, msg.Rationale, msg.SupersededBy)
	if err != nil {
		if errors.Is(err, storage.ErrDecisionNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("decision not found"))
		}
		if errors.Is(err, storage.ErrSupersededByRequired) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("superseded_by is required when status is superseded"))
		}
		if errors.Is(err, storage.ErrConcurrentModification) {
			return nil, connect.NewError(connect.CodeAborted, errors.New("concurrent modification — retry the operation"))
		}
		h.logger.ErrorContext(ctx, "update decision failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	pb, err := decisionToProto(d)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(pb), nil
}

// RegisterDecisionService registers the DecisionService on the given mux.
func RegisterDecisionService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption) {
	handler := &DecisionHandler{scoper: scoper, logger: slog.Default()}
	path, h := specgraphv1connect.NewDecisionServiceHandler(handler, opts...)
	mux.Handle(path, h)
}
