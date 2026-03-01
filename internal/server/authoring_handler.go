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
	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/seanb4t/specgraph/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AuthoringHandler implements the ConnectRPC AuthoringService.
type AuthoringHandler struct {
	store   storage.AuthoringBackend
	backend storage.Backend
}

var _ specgraphv1connect.AuthoringServiceHandler = (*AuthoringHandler)(nil)

// Spark handles the Spark RPC, creating a new spec and entering the spark stage.
func (h *AuthoringHandler) Spark(ctx context.Context, req *connect.Request[specv1.SparkRequest]) (*connect.Response[specv1.SparkResponse], error) {
	msg := req.Msg
	if msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}
	// CreateSpec already sets stage to "spark", so no TransitionStage call needed.
	_, err := h.backend.CreateSpec(ctx, msg.Slug, msg.Output.GetSeed(), defaultSpecPriority, defaultSpecComplexity)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if msg.Output != nil {
		if err := h.store.StoreSparkOutput(ctx, msg.Slug, msg.Output); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	safetyFlags := authoring.RunSafetyNet(&authoring.SafetyInput{Intent: msg.Output.GetSeed()})
	return connect.NewResponse(&specv1.SparkResponse{
		Output:      msg.Output,
		SafetyFlags: authoring.SafetyResultsToProto(safetyFlags),
		NextPrompts: authoring.PromptsToProto(authoring.StageShape),
	}), nil
}

// Shape handles the Shape RPC, transitioning from spark to shape stage.
func (h *AuthoringHandler) Shape(ctx context.Context, req *connect.Request[specv1.ShapeRequest]) (*connect.Response[specv1.ShapeResponse], error) {
	msg := req.Msg
	if err := h.store.TransitionStage(ctx, msg.Slug, authoring.StageSpark, authoring.StageShape); err != nil {
		return nil, h.stageError(err)
	}
	if msg.Output != nil {
		if err := h.store.StoreShapeOutput(ctx, msg.Slug, msg.Output); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	safetyFlags := authoring.RunSafetyNet(&authoring.SafetyInput{Scope: msg.Output.GetScopeIn()})
	return connect.NewResponse(&specv1.ShapeResponse{
		Output:      msg.Output,
		SafetyFlags: authoring.SafetyResultsToProto(safetyFlags),
		NextPrompts: authoring.PromptsToProto(authoring.StageSpecify),
	}), nil
}

// Specify handles the Specify RPC, transitioning from shape to specify stage.
func (h *AuthoringHandler) Specify(ctx context.Context, req *connect.Request[specv1.SpecifyRequest]) (*connect.Response[specv1.SpecifyResponse], error) {
	msg := req.Msg
	if err := h.store.TransitionStage(ctx, msg.Slug, authoring.StageShape, authoring.StageSpecify); err != nil {
		return nil, h.stageError(err)
	}
	if msg.Output != nil {
		if err := h.store.StoreSpecifyOutput(ctx, msg.Slug, msg.Output); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	safetyFlags := authoring.RunSafetyNet(&authoring.SafetyInput{
		Intent:     msg.Output.GetInterfaceContract(),
		Invariants: msg.Output.GetInvariants(),
	})
	return connect.NewResponse(&specv1.SpecifyResponse{
		Output:      msg.Output,
		SafetyFlags: authoring.SafetyResultsToProto(safetyFlags),
		NextPrompts: authoring.PromptsToProto(authoring.StageDecompose),
	}), nil
}

// Decompose handles the Decompose RPC, transitioning from specify to decompose stage.
func (h *AuthoringHandler) Decompose(ctx context.Context, req *connect.Request[specv1.DecomposeRequest]) (*connect.Response[specv1.DecomposeResponse], error) {
	msg := req.Msg
	if err := h.store.TransitionStage(ctx, msg.Slug, authoring.StageSpecify, authoring.StageDecompose); err != nil {
		return nil, h.stageError(err)
	}
	if msg.Output != nil {
		if _, err := h.store.StoreDecomposeOutput(ctx, msg.Slug, msg.Output); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&specv1.DecomposeResponse{Output: msg.Output}), nil
}

// Approve handles the Approve RPC, transitioning from decompose to approved stage.
func (h *AuthoringHandler) Approve(ctx context.Context, req *connect.Request[specv1.ApproveRequest]) (*connect.Response[specv1.ApproveResponse], error) {
	if err := h.store.TransitionStage(ctx, req.Msg.Slug, authoring.StageDecompose, authoring.StageApproved); err != nil {
		return nil, h.stageError(err)
	}
	return connect.NewResponse(&specv1.ApproveResponse{
		Slug:       req.Msg.Slug,
		Stage:      authoring.StageApproved,
		ApprovedAt: timestamppb.Now(),
	}), nil
}

// Amend handles the Amend RPC, rolling a spec back to an earlier stage.
func (h *AuthoringHandler) Amend(ctx context.Context, req *connect.Request[specv1.AmendRequest]) (*connect.Response[specv1.AmendResponse], error) {
	spec, err := h.store.AmendSpec(ctx, req.Msg.Slug, req.Msg.Reason, req.Msg.TargetStage)
	if err != nil {
		return nil, h.stageError(err)
	}
	return connect.NewResponse(&specv1.AmendResponse{
		Slug:    spec.Slug,
		Stage:   spec.Stage,
		Version: spec.Version,
	}), nil
}

// Supersede handles the Supersede RPC, marking a spec as replaced by another.
func (h *AuthoringHandler) Supersede(ctx context.Context, req *connect.Request[specv1.SupersedeRequest]) (*connect.Response[specv1.SupersedeResponse], error) {
	if err := h.store.SupersedeSpec(ctx, req.Msg.Slug, req.Msg.SupersededBy, req.Msg.Reason); err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.SupersedeResponse{
		Slug:         req.Msg.Slug,
		SupersededBy: req.Msg.SupersededBy,
	}), nil
}

// GetPrompts handles the GetPrompts RPC, returning prompt templates for a stage.
func (h *AuthoringHandler) GetPrompts(_ context.Context, req *connect.Request[specv1.GetPromptsRequest]) (*connect.Response[specv1.GetPromptsResponse], error) {
	return connect.NewResponse(&specv1.GetPromptsResponse{
		Prompts: authoring.PromptsToProto(req.Msg.Stage),
	}), nil
}

// RegisterAuthoringService registers the AuthoringService on the given mux.
func RegisterAuthoringService(mux *http.ServeMux, authoringStore storage.AuthoringBackend, backend storage.Backend) {
	handler := &AuthoringHandler{store: authoringStore, backend: backend}
	path, h := specgraphv1connect.NewAuthoringServiceHandler(handler)
	mux.Handle(path, h)
}

func (h *AuthoringHandler) stageError(err error) error {
	if errors.Is(err, storage.ErrInvalidStageTransition) {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if errors.Is(err, storage.ErrSpecNotFound) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}
