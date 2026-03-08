// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/storage"
)

// DriftChecker runs drift detection for specs.
type DriftChecker interface {
	Check(ctx context.Context, slug, scope string) ([]storage.DriftReport, error)
}

// SpecLinter runs lint validation for specs.
type SpecLinter interface {
	Lint(ctx context.Context, slug string) ([]storage.LintResult, error)
}

// LifecycleHandler implements the ConnectRPC LifecycleService.
type LifecycleHandler struct {
	store        storage.LifecycleBackend
	driftChecker DriftChecker
	linter       SpecLinter
	logger       *slog.Logger
}

var _ specgraphv1connect.LifecycleServiceHandler = (*LifecycleHandler)(nil)

// Amend handles the Amend RPC, transitioning a done spec back into authoring.
func (h *LifecycleHandler) Amend(ctx context.Context, req *connect.Request[specv1.LifecycleAmendRequest]) (*connect.Response[specv1.Spec], error) {
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.Reason == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reason is required"))
	}
	if len(msg.Reason) > maxFieldLen {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("reason exceeds maximum length of %d characters", maxFieldLen))
	}
	if msg.ReEntryStage != "" {
		if !storage.SpecStage(msg.ReEntryStage).IsValid() {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid re_entry_stage %q", msg.ReEntryStage))
		}
	}

	spec, err := h.store.LifecycleAmendSpec(ctx, msg.Slug, msg.Reason, msg.ReEntryStage)
	if err != nil {
		return nil, h.lifecycleError(err)
	}
	return connect.NewResponse(specToProto(spec)), nil
}

// Supersede handles the Supersede RPC, marking a spec as replaced by another.
func (h *LifecycleHandler) Supersede(ctx context.Context, req *connect.Request[specv1.LifecycleSupersedeRequest]) (*connect.Response[specv1.LifecycleSupersedeResponse], error) {
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.NewSlug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("new_slug is required"))
	}
	if err := validateSlug(msg.NewSlug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.Slug == msg.NewSlug {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a spec cannot supersede itself"))
	}

	oldSpec, newSpec, err := h.store.LifecycleSupersedeSpec(ctx, msg.Slug, msg.NewSlug)
	if err != nil {
		return nil, h.lifecycleError(err)
	}
	return connect.NewResponse(&specv1.LifecycleSupersedeResponse{
		OldSpec: specToProto(oldSpec),
		NewSpec: specToProto(newSpec),
	}), nil
}

// Abandon handles the Abandon RPC, transitioning a spec to abandoned (terminal).
func (h *LifecycleHandler) Abandon(ctx context.Context, req *connect.Request[specv1.LifecycleAbandonRequest]) (*connect.Response[specv1.Spec], error) {
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.Reason == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reason is required"))
	}
	if len(msg.Reason) > maxFieldLen {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("reason exceeds maximum length of %d characters", maxFieldLen))
	}

	spec, err := h.store.LifecycleAbandonSpec(ctx, msg.Slug, msg.Reason)
	if err != nil {
		return nil, h.lifecycleError(err)
	}
	return connect.NewResponse(specToProto(spec)), nil
}

// CheckDrift handles the CheckDrift RPC, returning drift reports for a spec.
// An empty slug checks all eligible (done/amended) specs.
func (h *LifecycleHandler) CheckDrift(ctx context.Context, req *connect.Request[specv1.DriftCheckRequest]) (*connect.Response[specv1.DriftCheckResponse], error) {
	msg := req.Msg
	if msg.Slug != "" {
		if err := validateSlug(msg.Slug); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}

	reports, err := h.driftChecker.Check(ctx, msg.Slug, msg.Scope)
	if err != nil {
		return nil, h.lifecycleError(err)
	}
	return connect.NewResponse(&specv1.DriftCheckResponse{
		Reports: driftReportsToProto(reports),
	}), nil
}

// AcknowledgeDrift handles the AcknowledgeDrift RPC, marking drift as intentional.
func (h *LifecycleHandler) AcknowledgeDrift(ctx context.Context, req *connect.Request[specv1.DriftAcknowledgeRequest]) (*connect.Response[specv1.DriftReport], error) {
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if len(msg.Note) > maxFieldLen {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("note exceeds maximum length of %d characters", maxFieldLen))
	}

	report, err := h.store.LifecycleAcknowledgeDrift(ctx, msg.Slug, msg.Note)
	if err != nil {
		return nil, h.lifecycleError(err)
	}
	return connect.NewResponse(driftReportToProto(report)), nil
}

// Lint handles the Lint RPC, validating spec schema and graph integrity.
func (h *LifecycleHandler) Lint(ctx context.Context, req *connect.Request[specv1.LintRequest]) (*connect.Response[specv1.LintResponse], error) {
	msg := req.Msg
	if msg.Slug != "" {
		if err := validateSlug(msg.Slug); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}

	results, err := h.linter.Lint(ctx, msg.Slug)
	if err != nil {
		return nil, h.lifecycleError(err)
	}
	return connect.NewResponse(&specv1.LintResponse{
		Results: lintResultsToProto(results),
	}), nil
}

// lifecycleError maps storage errors to connect error codes.
func (h *LifecycleHandler) lifecycleError(err error) error {
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		return connErr
	}
	if errors.Is(err, storage.ErrSpecNotFound) {
		return connect.NewError(connect.CodeNotFound, errors.New("spec not found"))
	}
	if errors.Is(err, storage.ErrSpecNotDone) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("spec must be in done stage"))
	}
	if errors.Is(err, storage.ErrSpecTerminal) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("spec is in a terminal state"))
	}
	if errors.Is(err, storage.ErrNewSpecNotFound) {
		return connect.NewError(connect.CodeNotFound, errors.New("replacement spec not found"))
	}
	h.logger.Error("lifecycleError: internal error", slog.Any("error", err))
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

// RegisterLifecycleService registers the LifecycleService on the given mux.
func RegisterLifecycleService(mux *http.ServeMux, store storage.LifecycleBackend, dc DriftChecker, l SpecLinter) {
	if store == nil {
		panic("RegisterLifecycleService: store must not be nil")
	}
	handler := &LifecycleHandler{
		store:        store,
		driftChecker: dc,
		linter:       l,
		logger:       slog.Default(),
	}
	path, h := specgraphv1connect.NewLifecycleServiceHandler(handler)
	mux.Handle(path, h)
}
