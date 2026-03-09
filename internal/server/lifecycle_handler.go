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

// TransitionAmend handles the TransitionAmend RPC, transitioning a done spec to
// an earlier authoring stage (or "amended" if no re-entry stage is specified).
func (h *LifecycleHandler) TransitionAmend(ctx context.Context, req *connect.Request[specv1.TransitionAmendRequest]) (*connect.Response[specv1.TransitionAmendResponse], error) {
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
		if stage.IsTerminal() {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("re_entry_stage %q is a terminal state and cannot be used as a re-entry point", msg.ReEntryStage))
		}
	}

	spec, err := h.store.LifecycleAmendSpec(ctx, msg.Slug, msg.Reason, msg.ReEntryStage)
	if err != nil {
		return nil, h.lifecycleError(err)
	}
	pb, err := specToProto(spec)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.TransitionAmendResponse{Spec: pb}), nil
}

// TransitionSupersede handles the TransitionSupersede RPC, marking a spec as replaced by another.
func (h *LifecycleHandler) TransitionSupersede(ctx context.Context, req *connect.Request[specv1.TransitionSupersedeRequest]) (*connect.Response[specv1.TransitionSupersedeResponse], error) {
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

	oldSpec, newSpec, err := h.store.LifecycleSupersedeSpec(ctx, msg.Slug, msg.NewSlug)
	if err != nil {
		return nil, h.lifecycleError(err)
	}
	oldPb, err := specToProto(oldSpec)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	newPb, err := specToProto(newSpec)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.TransitionSupersedeResponse{
		OldSpec: oldPb,
		NewSpec: newPb,
	}), nil
}

// TransitionAbandon handles the TransitionAbandon RPC, transitioning a spec to abandoned (terminal).
func (h *LifecycleHandler) TransitionAbandon(ctx context.Context, req *connect.Request[specv1.TransitionAbandonRequest]) (*connect.Response[specv1.TransitionAbandonResponse], error) {
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := validateRequiredField("reason", msg.Reason); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	spec, err := h.store.LifecycleAbandonSpec(ctx, msg.Slug, msg.Reason)
	if err != nil {
		return nil, h.lifecycleError(err)
	}
	pb, err := specToProto(spec)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.TransitionAbandonResponse{Spec: pb}), nil
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

	scopeStr, ok := driftScopeFromProto(msg.Scope)
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid scope %q (valid: deps, interfaces, verify)", msg.Scope.String()))
	}

	if h.driftChecker == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("drift checking is not configured"))
	}
	reports, err := h.driftChecker.Check(ctx, msg.Slug, scopeStr)
	if err != nil {
		return nil, h.lifecycleError(err)
	}

	// Merge persisted acknowledgment state into drift reports.
	if msg.Slug != "" {
		// Single-spec path: one GetSpec call for the requested slug.
		if spec, specErr := h.store.GetSpec(ctx, msg.Slug); specErr == nil {
			for i := range reports {
				if reports[i].SpecSlug == msg.Slug {
					reports[i].Acknowledged = spec.DriftAcknowledged
					reports[i].AcknowledgeNote = spec.DriftAcknowledgeNote
				}
			}
		} else if !errors.Is(specErr, storage.ErrSpecNotFound) {
			h.logger.Warn("CheckDrift: failed to merge acknowledgment state",
				slog.String("slug", msg.Slug), slog.Any("error", specErr))
			found := false
			for i := range reports {
				if reports[i].SpecSlug == msg.Slug {
					reports[i].ItemsStale = true
					found = true
				}
			}
			if !found {
				reports = append(reports, storage.DriftReport{
					SpecSlug:   msg.Slug,
					ItemsStale: true,
				})
			}
		}
	} else {
		// All-specs path: collect unique slugs from reports and merge each.
		seen := make(map[string]struct{}, len(reports))
		for _, r := range reports {
			seen[r.SpecSlug] = struct{}{}
		}
		for slug := range seen {
			spec, specErr := h.store.GetSpec(ctx, slug)
			if specErr == nil {
				for i := range reports {
					if reports[i].SpecSlug == slug {
						reports[i].Acknowledged = spec.DriftAcknowledged
						reports[i].AcknowledgeNote = spec.DriftAcknowledgeNote
					}
				}
			} else if !errors.Is(specErr, storage.ErrSpecNotFound) {
				h.logger.Warn("CheckDrift: failed to merge acknowledgment state",
					slog.String("slug", slug), slog.Any("error", specErr))
				found := false
				for i := range reports {
					if reports[i].SpecSlug == slug {
						reports[i].ItemsStale = true
						found = true
					}
				}
				if !found {
					reports = append(reports, storage.DriftReport{
						SpecSlug:   slug,
						ItemsStale: true,
					})
				}
			}
		}
	}

	pbReports, err := driftReportsToProto(reports)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.DriftCheckResponse{
		Reports: pbReports,
	}), nil
}

// AcknowledgeDrift handles the AcknowledgeDrift RPC, marking drift as intentional.
// After persisting the acknowledgment, it re-runs drift detection to return the
// actual drift items alongside the acknowledgment fields. If drift checking is
// not configured (driftChecker is nil), the response contains the acknowledgment
// fields but an empty items slice.
func (h *LifecycleHandler) AcknowledgeDrift(ctx context.Context, req *connect.Request[specv1.DriftAcknowledgeRequest]) (*connect.Response[specv1.DriftReport], error) {
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := validateRequiredField("note", msg.Note); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Only done/amended specs are eligible for drift acknowledgment.
	spec, err := h.store.GetSpec(ctx, msg.Slug)
	if err != nil {
		return nil, h.lifecycleError(err)
	}
	if spec.Stage != storage.SpecStageDone && spec.Stage != storage.SpecStageAmended {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("spec %q is in stage %q; only done/amended specs support drift acknowledgment", msg.Slug, spec.Stage))
	}

	report, err := h.store.LifecycleAcknowledgeDrift(ctx, msg.Slug, msg.Note)
	if err != nil {
		return nil, h.lifecycleError(err)
	}

	// Re-run drift detection to populate real drift items in the response.
	// The storage layer only persists the acknowledgment; it cannot compute drift items.
	if h.driftChecker != nil {
		reports, driftErr := h.driftChecker.Check(ctx, msg.Slug, "")
		if driftErr != nil {
			// Acknowledgment was already persisted — log the re-check error
			// but return the stored report rather than failing the entire RPC.
			// Mark items as stale so clients know the re-check failed.
			h.logger.Error("AcknowledgeDrift: drift re-check failed after successful acknowledgment",
				slog.String("slug", msg.Slug), slog.Any("error", driftErr))
			report.ItemsStale = true
		} else {
			for _, r := range reports {
				if r.SpecSlug == msg.Slug {
					report.Items = r.Items
					break
				}
			}
		}
	}

	pbReport, err := driftReportToProto(report)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(pbReport), nil
}

// Lint handles the Lint RPC, validating spec schema and graph integrity.
func (h *LifecycleHandler) Lint(ctx context.Context, req *connect.Request[specv1.LintRequest]) (*connect.Response[specv1.LintResponse], error) {
	msg := req.Msg
	if msg.Slug != "" {
		if err := validateSlug(msg.Slug); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}

	if h.linter == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("linting is not configured"))
	}
	results, err := h.linter.Lint(ctx, msg.Slug)
	if err != nil {
		return nil, h.lifecycleError(err)
	}
	pbResults, err := lintResultsToProto(results)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.LintResponse{
		Results: pbResults,
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
	if errors.Is(err, storage.ErrSpecIneligibleStage) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("spec is not in an eligible stage for this operation"))
	}
	if errors.Is(err, storage.ErrSpecTerminal) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("spec is in a terminal state"))
	}
	if errors.Is(err, storage.ErrNewSpecNotFound) {
		return connect.NewError(connect.CodeNotFound, errors.New("replacement spec not found"))
	}
	if errors.Is(err, storage.ErrConcurrentModification) {
		return connect.NewError(connect.CodeAborted, errors.New("concurrent modification — retry the operation"))
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
