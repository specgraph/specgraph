// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/storage"
	"gopkg.in/yaml.v3"
)

const maxEventsLimit = 500

// ExecutionHandler implements the ConnectRPC ExecutionService.
type ExecutionHandler struct {
	scoper storage.Scoper
}

var _ specgraphv1connect.ExecutionServiceHandler = (*ExecutionHandler)(nil)

// RegisterExecutionService registers the ExecutionService on the given mux.
func RegisterExecutionService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption) {
	handler := &ExecutionHandler{scoper: scoper}
	path, h := specgraphv1connect.NewExecutionServiceHandler(handler, opts...)
	mux.Handle(path, h)
}

// GenerateBundle handles the GenerateBundle RPC.
func (h *ExecutionHandler) GenerateBundle(ctx context.Context, req *connect.Request[specv1.GenerateBundleRequest]) (*connect.Response[specv1.Bundle], error) {
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
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("convert bundle: %w", err))
	}
	pb.BundleYaml = renderBundleYAML(b)

	return connect.NewResponse(pb), nil
}

// GetPrime handles the GetPrime RPC.
func (h *ExecutionHandler) GetPrime(ctx context.Context, req *connect.Request[specv1.GetPrimeRequest]) (*connect.Response[specv1.PrimeResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	if msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}

	pd, err := store.GetPrimeData(ctx, msg.Slug)
	if err != nil {
		return nil, executionError(err)
	}

	decisions, err := decisionsToProto(pd.Decisions)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("convert decisions: %w", err))
	}

	var constitutionSummary string
	var codingConventions string
	if pd.Constitution != nil {
		constitutionSummary = fmt.Sprintf("%s (%s layer)", pd.Constitution.Name, pd.Constitution.Layer)
		var parts []string
		for _, p := range pd.Constitution.Principles {
			parts = append(parts, p.Statement)
		}
		if len(pd.Constitution.Constraints) > 0 {
			parts = append(parts, pd.Constitution.Constraints...)
		}
		codingConventions = strings.Join(parts, "; ")
	}

	resp := &specv1.PrimeResponse{
		ConstitutionSummary: constitutionSummary,
		ProjectContext:      pd.Spec.Intent,
		Decisions:           decisions,
		CodingConventions:   codingConventions,
		CallbackDocs:        "Use ReportProgress, ReportBlocker, and ReportCompletion RPCs to report execution status.",
	}

	return connect.NewResponse(resp), nil
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

	return connect.NewResponse(&specv1.GetExecutionEventsResponse{
		Events: executionEventsToProto(events),
	}), nil
}

// executionError maps storage errors to appropriate connect error codes.
func executionError(err error) error {
	switch {
	case errors.Is(err, storage.ErrSpecNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, storage.ErrSpecNotApproved):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, storage.ErrAgentNotClaimOwner):
		return connect.NewError(connect.CodePermissionDenied, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

// renderBundleYAML produces a human-readable YAML representation of a bundle.
func renderBundleYAML(b *storage.Bundle) string {
	type bundleDecision struct {
		Slug   string `yaml:"slug"`
		Title  string `yaml:"title"`
		Status string `yaml:"status"`
	}
	type bundleCallbacks struct {
		Endpoint string `yaml:"endpoint"`
	}
	type bundleSpec struct {
		Slug   string            `yaml:"slug"`
		Intent string            `yaml:"intent"`
		Stage  storage.SpecStage `yaml:"stage"`
	}
	type bundleYAML struct {
		Version   int32            `yaml:"version"`
		Spec      bundleSpec       `yaml:"spec"`
		Decisions []bundleDecision `yaml:"decisions,omitempty"`
		Callbacks *bundleCallbacks `yaml:"callbacks,omitempty"`
	}

	doc := bundleYAML{
		Version: b.Version,
		Spec: bundleSpec{
			Slug:   b.Spec.Slug,
			Intent: b.Spec.Intent,
			Stage:  b.Spec.Stage,
		},
	}
	for _, d := range b.Decisions {
		doc.Decisions = append(doc.Decisions, bundleDecision{
			Slug:   d.Slug,
			Title:  d.Title,
			Status: string(d.Status),
		})
	}
	if b.Callbacks != nil {
		doc.Callbacks = &bundleCallbacks{Endpoint: b.Callbacks.Endpoint}
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Sprintf("# error rendering bundle: %v\n", err)
	}
	return "# SpecGraph Execution Bundle\n" + string(out)
}
