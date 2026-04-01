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
func (h *DecisionHandler) CreateDecision(ctx context.Context, req *connect.Request[specv1.CreateDecisionRequest]) (*connect.Response[specv1.CreateDecisionResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	if err := validateSlug(msg.GetSlug()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	d, err := store.CreateDecision(ctx, msg.Slug, msg.Title, msg.Decision, msg.Rationale,
		msg.Question, rejectedAltsFromProto(msg.RejectedAlternatives),
		decisionConfidenceFromProto(msg.Confidence), msg.Tags,
		decisionScopeFromProto(msg.Scope), msg.OriginSpec, msg.OriginStage)
	if err != nil {
		return nil, h.decisionError(ctx, err)
	}
	pb, err := decisionToProto(d)
	if err != nil {
		return nil, h.decisionError(ctx, err)
	}
	return connect.NewResponse(&specv1.CreateDecisionResponse{Decision: pb}), nil
}

// GetDecision handles the GetDecision RPC.
func (h *DecisionHandler) GetDecision(ctx context.Context, req *connect.Request[specv1.GetDecisionRequest]) (*connect.Response[specv1.GetDecisionResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.GetSlug()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	d, err := store.GetDecision(ctx, req.Msg.Slug)
	if err != nil {
		return nil, h.decisionError(ctx, err)
	}
	pb, err := decisionToProto(d)
	if err != nil {
		return nil, h.decisionError(ctx, err)
	}
	return connect.NewResponse(&specv1.GetDecisionResponse{Decision: pb}), nil
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
		return nil, h.decisionError(ctx, err)
	}
	pbs, err := decisionsToProto(decisions)
	if err != nil {
		return nil, h.decisionError(ctx, err)
	}
	return connect.NewResponse(&specv1.ListDecisionsResponse{Decisions: pbs}), nil
}

// UpdateDecision handles the UpdateDecision RPC.
func (h *DecisionHandler) UpdateDecision(ctx context.Context, req *connect.Request[specv1.UpdateDecisionRequest]) (*connect.Response[specv1.UpdateDecisionResponse], error) {
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

	var question *string
	if msg.Question != nil {
		question = msg.Question
	}
	var confidence *storage.DecisionConfidence
	if msg.Confidence != nil {
		c := decisionConfidenceFromProto(*msg.Confidence)
		confidence = &c
	}
	var scope *storage.DecisionScope
	if msg.Scope != nil {
		sc := decisionScopeFromProto(*msg.Scope)
		scope = &sc
	}
	var rejectedAlts *[]storage.RejectedAlternative
	if len(msg.RejectedAlternatives) > 0 {
		alts := rejectedAltsFromProto(msg.RejectedAlternatives)
		rejectedAlts = &alts
	}
	var tags *[]string
	if len(msg.Tags) > 0 {
		t := msg.Tags
		tags = &t
	}

	if msg.GetExpectedVersion() < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("expected_version must be non-negative"))
	}

	d, err := store.UpdateDecision(ctx, msg.Slug, msg.GetExpectedVersion(), msg.Title, domainStatus, msg.Decision, msg.Rationale, msg.SupersededBy,
		question, rejectedAlts, confidence, tags, scope, msg.OriginSpec, msg.OriginStage)
	if err != nil {
		return nil, h.decisionError(ctx, err)
	}
	pb, err := decisionToProto(d)
	if err != nil {
		return nil, h.decisionError(ctx, err)
	}
	return connect.NewResponse(&specv1.UpdateDecisionResponse{Decision: pb}), nil
}

// decisionError maps storage errors to sanitized connect error codes.
func (h *DecisionHandler) decisionError(ctx context.Context, err error) error {
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		return connErr
	}
	switch {
	case errors.Is(err, storage.ErrDecisionNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("decision not found"))
	case errors.Is(err, storage.ErrSupersededByRequired):
		return connect.NewError(connect.CodeInvalidArgument, errors.New("superseded_by is required when status is superseded"))
	case errors.Is(err, storage.ErrConcurrentModification):
		return connect.NewError(connect.CodeAborted, errors.New("concurrent modification — retry the operation"))
	default:
		h.logger.ErrorContext(ctx, "decisionError: internal error", slog.Any("error", err))
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}

// RegisterDecisionService registers the DecisionService on the given mux.
func RegisterDecisionService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption) {
	handler := &DecisionHandler{scoper: scoper, logger: slog.Default()}
	path, h := specgraphv1connect.NewDecisionServiceHandler(handler, opts...)
	mux.Handle(path, h)
}
