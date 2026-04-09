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
	"github.com/specgraph/specgraph/internal/drift"
	"github.com/specgraph/specgraph/internal/storage"
)

// DriftChecker runs drift detection for specs.
type DriftChecker interface {
	Check(ctx context.Context, slug, scope string) (*drift.CheckResult, error)
}

// SpecLinter runs lint validation for specs.
type SpecLinter interface {
	Lint(ctx context.Context, slug string) ([]storage.LintResult, error)
}

// LifecycleHandler implements the ConnectRPC LifecycleService.
type LifecycleHandler struct {
	scoper       storage.Scoper
	driftChecker DriftChecker
	linter       SpecLinter
	logger       *slog.Logger
}

var _ specgraphv1connect.LifecycleServiceHandler = (*LifecycleHandler)(nil)

// TransitionAmend handles the TransitionAmend RPC, transitioning a done spec to
// an earlier authoring stage (or "amended" if no re-entry stage is specified).
func (h *LifecycleHandler) TransitionAmend(ctx context.Context, req *connect.Request[specv1.TransitionAmendRequest]) (*connect.Response[specv1.TransitionAmendResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := validateRequiredField("reason", msg.Reason); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.ReEntryStage != "" {
		stage := storage.SpecStage(msg.ReEntryStage)
		if !stage.IsValid() {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid re_entry_stage %q", msg.ReEntryStage))
		}
		if stage.ExcludesReEntry() {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("re_entry_stage %q cannot be used as a re-entry point", msg.ReEntryStage))
		}
	}

	spec, err := store.LifecycleAmendSpec(ctx, msg.Slug, msg.Reason, msg.ReEntryStage)
	if err != nil {
		return nil, h.lifecycleError("TransitionAmend", msg.Slug, err)
	}
	pb, err := specToProto(spec)
	if err != nil {
		return nil, h.lifecycleError("TransitionAmend", msg.Slug, err)
	}
	return connect.NewResponse(&specv1.TransitionAmendResponse{Spec: pb}), nil
}

// TransitionSupersede handles the TransitionSupersede RPC, marking a spec as replaced by another.
func (h *LifecycleHandler) TransitionSupersede(ctx context.Context, req *connect.Request[specv1.TransitionSupersedeRequest]) (*connect.Response[specv1.TransitionSupersedeResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := validateSlug(msg.NewSlug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("new_slug: %w", err))
	}
	if msg.Slug == msg.NewSlug {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a spec cannot supersede itself"))
	}

	oldSpec, newSpec, err := store.LifecycleSupersedeSpec(ctx, msg.Slug, msg.NewSlug)
	if err != nil {
		return nil, h.lifecycleError("TransitionSupersede", msg.Slug, err)
	}
	oldPb, err := specToProto(oldSpec)
	if err != nil {
		return nil, h.lifecycleError("TransitionSupersede", msg.Slug, err)
	}
	newPb, err := specToProto(newSpec)
	if err != nil {
		return nil, h.lifecycleError("TransitionSupersede", msg.Slug, err)
	}
	return connect.NewResponse(&specv1.TransitionSupersedeResponse{
		OldSpec: oldPb,
		NewSpec: newPb,
	}), nil
}

// TransitionAbandon handles the TransitionAbandon RPC, transitioning a spec to abandoned (terminal).
func (h *LifecycleHandler) TransitionAbandon(ctx context.Context, req *connect.Request[specv1.TransitionAbandonRequest]) (*connect.Response[specv1.TransitionAbandonResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := validateRequiredField("reason", msg.Reason); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	spec, err := store.LifecycleAbandonSpec(ctx, msg.Slug, msg.Reason)
	if err != nil {
		return nil, h.lifecycleError("TransitionAbandon", msg.Slug, err)
	}
	pb, err := specToProto(spec)
	if err != nil {
		return nil, h.lifecycleError("TransitionAbandon", msg.Slug, err)
	}
	return connect.NewResponse(&specv1.TransitionAbandonResponse{Spec: pb}), nil
}

// CheckDrift handles the CheckDrift RPC, returning drift reports for a spec.
// An empty slug checks all eligible (done/amended) specs.
func (h *LifecycleHandler) CheckDrift(ctx context.Context, req *connect.Request[specv1.DriftCheckRequest]) (*connect.Response[specv1.DriftCheckResponse], error) {
	if _, scopeErr := scopeStore(ctx, h.scoper); scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	if msg.Slug != "" {
		if err := validateSlug(msg.Slug); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}

	scopeStr, ok := driftScopeFromProto(msg.Scope)
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid scope %q", msg.Scope.String()))
	}

	if h.driftChecker == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("drift checking is not configured"))
	}
	result, err := h.driftChecker.Check(ctx, msg.Slug, scopeStr)
	if err != nil {
		return nil, h.lifecycleError("CheckDrift", msg.Slug, err)
	}

	protoReports, err := driftReportsToProto(result.Reports)
	if err != nil {
		return nil, h.lifecycleError("CheckDrift", msg.Slug, err)
	}
	return connect.NewResponse(&specv1.DriftCheckResponse{
		Reports:      protoReports,
		SkippedCount: result.SkippedCount,
	}), nil
}

// AcknowledgeDrift handles the AcknowledgeDrift RPC, marking drift as intentional.
// After persisting the acknowledgment, it re-runs drift detection to return the
// current drift state. If drift checking is not configured (driftChecker is nil),
// the response contains an empty items slice.
func (h *LifecycleHandler) AcknowledgeDrift(ctx context.Context, req *connect.Request[specv1.DriftAcknowledgeRequest]) (*connect.Response[specv1.DriftAcknowledgeResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.UpstreamSlug != "" {
		if err := validateSlug(msg.UpstreamSlug); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("upstream_slug: %w", err))
		}
	}
	if msg.UpstreamSlug == "" && !msg.All {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("specify upstream_slug or set all=true"))
	}
	if msg.UpstreamSlug != "" && msg.All {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot specify both upstream_slug and all=true"))
	}
	if len(msg.Note) > maxFieldLen {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("note exceeds maximum of %d characters", maxFieldLen))
	}

	if err := store.LifecycleAcknowledgeDrift(ctx, msg.Slug, msg.UpstreamSlug, msg.Note); err != nil {
		return nil, h.lifecycleError("AcknowledgeDrift", msg.Slug, err)
	}

	// Re-run drift check to return updated report.
	report := &storage.DriftReport{SpecSlug: msg.Slug, Items: []storage.DriftItem{}}
	if h.driftChecker != nil {
		result, driftErr := h.driftChecker.Check(ctx, msg.Slug, "")
		if driftErr != nil {
			h.logger.Error("AcknowledgeDrift: drift re-check failed",
				slog.String("slug", msg.Slug), slog.Any("error", driftErr))
			report.ErrorMessage = "drift re-check failed after acknowledgment"
		} else if result != nil {
			for _, r := range result.Reports {
				if r.SpecSlug == msg.Slug {
					report = &r
					break
				}
			}
		}
	}

	protoReport, err := driftReportToProto(report)
	if err != nil {
		return nil, h.lifecycleError("AcknowledgeDrift", msg.Slug, err)
	}
	return connect.NewResponse(&specv1.DriftAcknowledgeResponse{Report: protoReport}), nil
}

// Lint handles the Lint RPC, validating spec schema and graph integrity.
func (h *LifecycleHandler) Lint(ctx context.Context, req *connect.Request[specv1.LintRequest]) (*connect.Response[specv1.LintResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	if msg.Slug != "" {
		if err := validateSlug(msg.Slug); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		// Verify the spec exists in the scoped store before delegating to the linter.
		if _, err := store.GetSpec(ctx, msg.Slug); err != nil {
			return nil, h.lifecycleError("Lint", msg.Slug, err)
		}
	}

	if h.linter == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("linting is not configured"))
	}
	results, err := h.linter.Lint(ctx, msg.Slug)
	if err != nil {
		return nil, h.lifecycleError("Lint", msg.Slug, err)
	}
	protoResults, err := lintResultsToProto(results)
	if err != nil {
		return nil, h.lifecycleError("Lint", msg.Slug, err)
	}
	return connect.NewResponse(&specv1.LintResponse{
		Results: protoResults,
	}), nil
}

// specMsg returns a slug-qualified message when slug is non-empty, or a plain
// "spec <base>" message when slug is empty (e.g. all-specs operations).
func specMsg(slug, base string) string {
	if slug != "" {
		return fmt.Sprintf("spec %q %s", slug, base)
	}
	return "spec " + base
}

// lifecycleError maps storage errors to connect error codes.
// slug is the client-provided spec identifier (safe to echo in error messages).
func (h *LifecycleHandler) lifecycleError(op, slug string, err error) error {
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		return connErr
	}
	if errors.Is(err, storage.ErrSpecNotFound) {
		return connect.NewError(connect.CodeNotFound, errors.New(specMsg(slug, "not found")))
	}
	if errors.Is(err, storage.ErrSpecNotDone) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New(specMsg(slug, "must be in done stage")))
	}
	if errors.Is(err, storage.ErrSpecIneligibleStage) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New(specMsg(slug, "is not in an eligible stage for this operation")))
	}
	if errors.Is(err, storage.ErrSpecTerminal) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New(specMsg(slug, "is in a terminal state")))
	}
	if errors.Is(err, storage.ErrSpecIneligibleForDrift) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New(specMsg(slug, "is not eligible for drift checking (must be done or amended)")))
	}
	if errors.Is(err, storage.ErrNewSpecNotFound) {
		return connect.NewError(connect.CodeNotFound, errors.New("replacement spec not found"))
	}
	if errors.Is(err, storage.ErrNewSpecTerminal) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("replacement spec is in a terminal state"))
	}
	if errors.Is(err, storage.ErrConcurrentModification) {
		return connect.NewError(connect.CodeAborted, errors.New("concurrent modification — retry the operation"))
	}
	if errors.Is(err, storage.ErrInternalGuardFailure) {
		h.logger.Error("lifecycleError: internal guard failure", slog.String("op", op), slog.String("slug", slug), slog.Any("error", err))
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	if errors.Is(err, storage.ErrInvalidReEntryStage) {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("re-entry stage is not allowed"))
	}
	if errors.Is(err, storage.ErrEdgeNotFound) {
		return connect.NewError(connect.CodeNotFound, errors.New("no matching dependency edge found"))
	}
	if errors.Is(err, storage.ErrSameSlugs) {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("old and new slugs must differ"))
	}
	h.logger.Error("lifecycleError: internal error", slog.String("op", op), slog.String("slug", slug), slog.Any("error", err))
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

// RegisterLifecycleService registers the LifecycleService on the given mux.
// If logger is nil, slog.Default() is used.
func RegisterLifecycleService(mux *http.ServeMux, scoper storage.Scoper, dc DriftChecker, l SpecLinter, logger *slog.Logger, opts ...connect.HandlerOption) {
	if scoper == nil {
		panic("RegisterLifecycleService: scoper must not be nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	handler := &LifecycleHandler{
		scoper:       scoper,
		driftChecker: dc,
		linter:       l,
		logger:       logger,
	}
	path, h := specgraphv1connect.NewLifecycleServiceHandler(handler, opts...)
	mux.Handle(path, h)
}
