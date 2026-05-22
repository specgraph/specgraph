// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/mcp/skills"
	"github.com/specgraph/specgraph/internal/prime"
	"github.com/specgraph/specgraph/internal/storage"
)

const maxEventsLimit = 500

// ExecutionHandler implements the ConnectRPC ExecutionService.
type ExecutionHandler struct {
	scoper storage.Scoper
	// skills is consumed by the prime Composer when assembling
	// ProjectView responses (project-scope GetPrime). It may be nil in
	// tests that never exercise the empty-slug code path; production
	// wire-up always provides a non-nil source.
	skills skills.Source
}

var _ specgraphv1connect.ExecutionServiceHandler = (*ExecutionHandler)(nil)

// RegisterExecutionService registers the ExecutionService on the given mux.
//
// skillsSrc is required for project-scope GetPrime (empty slug) which
// surfaces a skills count in the ProjectView. Passing nil is permitted
// for tests that never invoke the project-scope path.
func RegisterExecutionService(mux *http.ServeMux, scoper storage.Scoper, skillsSrc skills.Source, opts ...connect.HandlerOption) {
	handler := &ExecutionHandler{scoper: scoper, skills: skillsSrc}
	path, h := specgraphv1connect.NewExecutionServiceHandler(handler, opts...)
	mux.Handle(path, h)
}

// GenerateBundle handles the GenerateBundle RPC.
func (h *ExecutionHandler) GenerateBundle(ctx context.Context, req *connect.Request[specv1.GenerateBundleRequest]) (*connect.Response[specv1.GenerateBundleResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	if msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}

	b, err := store.GenerateBundle(ctx, msg.Slug)
	if err != nil {
		return nil, executionError(err)
	}

	// If the caller supplied a callback endpoint, build the callback config.
	if msg.Endpoint != "" {
		b.Callbacks = &storage.CallbackConfig{
			Endpoint:   msg.Endpoint,
			Prime:      msg.Endpoint + "/prime",
			Progress:   msg.Endpoint + "/progress",
			Blocker:    msg.Endpoint + "/blocker",
			Completion: msg.Endpoint + "/completion",
		}
	}

	pb, err := bundleToProto(b)
	if err != nil {
		return nil, executionError(err)
	}
	pb.BundleContent = renderBundleMarkdown(b)

	return connect.NewResponse(&specv1.GenerateBundleResponse{Bundle: pb}), nil
}

// GetPrime handles the GetPrime RPC.
//
// Routing by request scope:
//   - Empty slug returns a project-scope response with the
//     project_view oneof populated by the prime Composer. Legacy
//     summary fields (1–5) are intentionally left zero per design
//     Section 10 ("Legacy summary fields populated only for spec
//     scope").
//   - Non-empty slug returns a spec-scope response: the spec_view
//     oneof is populated, AND the legacy summary fields (1–5) are
//     populated as before for backward compatibility with existing
//     polecat consumers.
//
// Unknown slugs surface as connect.CodeNotFound.
func (h *ExecutionHandler) GetPrime(ctx context.Context, req *connect.Request[specv1.GetPrimeRequest]) (*connect.Response[specv1.PrimeResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	composer := prime.New(store, h.skills)
	msg := req.Msg

	if msg.Slug == "" {
		return h.getPrimeProject(ctx, composer)
	}
	return h.getPrimeSpec(ctx, store, composer, msg.Slug)
}

// getPrimeProject composes the project-scope PrimeResponse. Legacy
// summary fields 1–5 are left zero by design (Section 10).
func (h *ExecutionHandler) getPrimeProject(ctx context.Context, composer *prime.Composer) (*connect.Response[specv1.PrimeResponse], error) {
	view, err := composer.Project(ctx)
	if err != nil {
		return nil, executionError(err)
	}
	pview, err := primeProjectViewToProto(view)
	if err != nil {
		slog.Error("GetPrime: convert project view", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	return connect.NewResponse(&specv1.PrimeResponse{
		View: &specv1.PrimeResponse_ProjectView{ProjectView: pview},
	}), nil
}

// getPrimeSpec composes the spec-scope PrimeResponse. Populates both
// the spec_view oneof and the legacy summary fields (1–5) so older
// polecats that read the legacy shape continue to work.
func (h *ExecutionHandler) getPrimeSpec(ctx context.Context, store storage.ScopedBackend, composer *prime.Composer, slug string) (*connect.Response[specv1.PrimeResponse], error) {
	sview, err := composer.Spec(ctx, slug)
	if err != nil {
		return nil, executionError(err)
	}
	pSpecView, err := primeSpecViewToProto(sview)
	if err != nil {
		slog.Error("GetPrime: convert spec view", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	// Legacy summary fields — preserved for backward compatibility.
	pd, err := store.GetPrimeData(ctx, slug)
	if err != nil {
		return nil, executionError(err)
	}
	legacyDecisions, err := decisionsToProto(pd.Decisions)
	if err != nil {
		return nil, executionError(err)
	}
	constitutionSummary, codingConventions := legacyConstitutionSummary(pd.Constitution)

	return connect.NewResponse(&specv1.PrimeResponse{
		ConstitutionSummary: constitutionSummary,
		ProjectContext:      pd.Spec.Intent,
		Decisions:           legacyDecisions,
		CodingConventions:   codingConventions,
		CallbackDocs:        "Use ReportProgress, ReportBlocker, and ReportCompletion RPCs to report execution status.",
		View:                &specv1.PrimeResponse_SpecView{SpecView: pSpecView},
	}), nil
}

// legacyConstitutionSummary reproduces the pre-Piece-E formatting used
// by older polecats: "<name> (<layer> layer)" and a semicolon-joined
// list of principle statements + constraints. Returns zero strings
// when c is nil.
func legacyConstitutionSummary(c *storage.Constitution) (summary, conventions string) {
	if c == nil {
		return "", ""
	}
	summary = fmt.Sprintf("%s (%s layer)", c.Name, c.Layer)
	parts := make([]string, 0, len(c.Principles)+len(c.Constraints))
	for _, p := range c.Principles {
		parts = append(parts, p.Statement)
	}
	parts = append(parts, c.Constraints...)
	return summary, strings.Join(parts, "; ")
}

// ReportProgress handles the ReportProgress RPC.
func (h *ExecutionHandler) ReportProgress(ctx context.Context, req *connect.Request[specv1.ReportProgressRequest]) (*connect.Response[specv1.ReportProgressResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	if msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}
	if msg.Agent == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("agent is required"))
	}
	if msg.Message == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("message is required"))
	}

	if err := store.RecordProgress(ctx, msg.Slug, msg.Agent, msg.Message); err != nil {
		return nil, executionError(err)
	}

	return connect.NewResponse(&specv1.ReportProgressResponse{Acknowledged: true}), nil
}

// ReportBlocker handles the ReportBlocker RPC.
func (h *ExecutionHandler) ReportBlocker(ctx context.Context, req *connect.Request[specv1.ReportBlockerRequest]) (*connect.Response[specv1.ReportBlockerResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	if msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}
	if msg.Agent == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("agent is required"))
	}
	if msg.Description == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("description is required"))
	}

	if err := store.RecordBlocker(ctx, msg.Slug, msg.Agent, msg.Description); err != nil {
		return nil, executionError(err)
	}

	return connect.NewResponse(&specv1.ReportBlockerResponse{Acknowledged: true}), nil
}

// ReportCompletion handles the ReportCompletion RPC.
func (h *ExecutionHandler) ReportCompletion(ctx context.Context, req *connect.Request[specv1.ReportCompletionRequest]) (*connect.Response[specv1.ReportCompletionResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	if msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}
	if msg.Agent == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("agent is required"))
	}

	spec, err := store.GetSpec(ctx, msg.Slug)
	if err != nil {
		return nil, executionError(err)
	}
	if spec.Provenance != storage.SpecProvenanceAuthored {
		return nil, connect.NewError(connect.CodeInvalidArgument, storage.ErrCompletionRequiresAuthored)
	}

	if err := store.RecordCompletion(ctx, msg.Slug, msg.Agent); err != nil {
		return nil, executionError(err)
	}

	return connect.NewResponse(&specv1.ReportCompletionResponse{
		Acknowledged: true,
		NewStage:     "done",
	}), nil
}

// GetExecutionEvents handles the GetExecutionEvents RPC.
func (h *ExecutionHandler) GetExecutionEvents(ctx context.Context, req *connect.Request[specv1.GetExecutionEventsRequest]) (*connect.Response[specv1.GetExecutionEventsResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	if msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}

	limit := int(msg.Limit)
	if limit == 0 {
		limit = 50
	}
	if limit > maxEventsLimit {
		limit = maxEventsLimit
	}

	events, err := store.GetExecutionEvents(ctx, msg.Slug, limit)
	if err != nil {
		return nil, executionError(err)
	}

	pbEvents, convErr := executionEventsToProto(events)
	if convErr != nil {
		return nil, executionError(convErr)
	}
	return connect.NewResponse(&specv1.GetExecutionEventsResponse{
		Events: pbEvents,
	}), nil
}

// executionError maps storage errors to sanitized connect error codes.
func executionError(err error) error {
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		return connErr
	}
	switch {
	case errors.Is(err, storage.ErrSpecNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("spec not found"))
	case errors.Is(err, storage.ErrSpecNotApproved):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("spec is not in an approved or in_progress stage"))
	case errors.Is(err, storage.ErrAgentNotClaimOwner):
		return connect.NewError(connect.CodePermissionDenied, errors.New("agent does not hold the claim for this spec"))
	default:
		slog.Error("executionError: internal error", slog.Any("error", err))
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}
