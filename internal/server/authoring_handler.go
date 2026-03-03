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
	if msg.Output == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("output is required"))
	}
	// CreateSpec sets stage to "spark" as part of spec creation; no separate
	// TransitionStage call is needed because the initial stage is set atomically.
	_, err := h.backend.CreateSpec(ctx, msg.Slug, msg.Output.GetSeed(), defaultSpecPriority, defaultSpecComplexity)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := h.store.StoreSparkOutput(ctx, msg.Slug, sparkOutputToDomain(msg.Output)); err != nil {
		// NOTE: CreateSpec succeeded but StoreSparkOutput failed. The spec exists
		// without output data. A retry of Spark will fail due to duplicate slug.
		// TODO: Add transaction support to make this atomic.
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("store spark output (spec %q created but output not stored): %w", msg.Slug, err))
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
	if msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}
	if msg.Output == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("output is required"))
	}
	if err := h.store.TransitionStage(ctx, msg.Slug, authoring.StageSpark, authoring.StageShape); err != nil {
		return nil, h.stageError(err)
	}
	if err := h.store.StoreShapeOutput(ctx, msg.Slug, shapeOutputToDomain(msg.Output)); err != nil {
		// NOTE: TransitionStage succeeded but StoreShapeOutput failed. The spec
		// is now in the shape stage but has no shape output stored.
		// TODO: Add transaction support to make this atomic.
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("store shape output (spec %q transitioned but output not stored): %w", msg.Slug, err))
	}
	scope := make([]string, 0, len(msg.Output.GetScopeIn())+len(msg.Output.GetScopeOut()))
	scope = append(scope, msg.Output.GetScopeIn()...)
	scope = append(scope, msg.Output.GetScopeOut()...)
	safetyFlags := authoring.RunSafetyNet(&authoring.SafetyInput{
		Intent: strings.Join(msg.Output.GetRisks(), " "),
		Scope:  scope,
	})
	return connect.NewResponse(&specv1.ShapeResponse{
		Output:      msg.Output,
		SafetyFlags: authoring.SafetyResultsToProto(safetyFlags),
		NextPrompts: authoring.PromptsToProto(authoring.StageSpecify),
	}), nil
}

// Specify handles the Specify RPC, transitioning from shape to specify stage.
func (h *AuthoringHandler) Specify(ctx context.Context, req *connect.Request[specv1.SpecifyRequest]) (*connect.Response[specv1.SpecifyResponse], error) {
	msg := req.Msg
	if msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}
	if msg.Output == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("output is required"))
	}
	if err := h.store.TransitionStage(ctx, msg.Slug, authoring.StageShape, authoring.StageSpecify); err != nil {
		return nil, h.stageError(err)
	}
	if err := h.store.StoreSpecifyOutput(ctx, msg.Slug, specifyOutputToDomain(msg.Output)); err != nil {
		// NOTE: TransitionStage succeeded but StoreSpecifyOutput failed. The spec
		// is now in the specify stage but has no specify output stored.
		// TODO: Add transaction support to make this atomic.
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("store specify output (spec %q transitioned but output not stored): %w", msg.Slug, err))
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
	if msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}
	if msg.Output == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("output is required"))
	}
	if err := h.store.TransitionStage(ctx, msg.Slug, authoring.StageSpecify, authoring.StageDecompose); err != nil {
		return nil, h.stageError(err)
	}
	if _, err := h.store.StoreDecomposeOutput(ctx, msg.Slug, decomposeOutputToDomain(msg.Output)); err != nil {
		// NOTE: TransitionStage succeeded but StoreDecomposeOutput failed. The spec
		// is now in the decompose stage but has no decompose output stored.
		// TODO: Add transaction support to make this atomic.
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("store decompose output (spec %q transitioned but output not stored): %w", msg.Slug, err))
	}
	return connect.NewResponse(&specv1.DecomposeResponse{Output: msg.Output}), nil
}

// Approve handles the Approve RPC, transitioning from decompose to approved stage.
func (h *AuthoringHandler) Approve(ctx context.Context, req *connect.Request[specv1.ApproveRequest]) (*connect.Response[specv1.ApproveResponse], error) {
	if req.Msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}
	if err := h.store.TransitionStage(ctx, req.Msg.Slug, authoring.StageDecompose, authoring.StageApproved); err != nil {
		return nil, h.stageError(err)
	}
	return connect.NewResponse(&specv1.ApproveResponse{
		Slug:       req.Msg.Slug,
		Stage:      stageToProto[authoring.StageApproved],
		ApprovedAt: timestamppb.Now(),
	}), nil
}

// Amend handles the Amend RPC, rolling a spec back to an earlier stage.
func (h *AuthoringHandler) Amend(ctx context.Context, req *connect.Request[specv1.AmendRequest]) (*connect.Response[specv1.AmendResponse], error) {
	if req.Msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}
	spec, err := h.store.AmendSpec(ctx, req.Msg.Slug, req.Msg.Reason, protoToStage[req.Msg.TargetStage])
	if err != nil {
		return nil, h.stageError(err)
	}
	return connect.NewResponse(&specv1.AmendResponse{
		Slug:    spec.Slug,
		Stage:   stageToProto[spec.Stage],
		Version: spec.Version,
	}), nil
}

// Supersede handles the Supersede RPC, marking a spec as replaced by another.
func (h *AuthoringHandler) Supersede(ctx context.Context, req *connect.Request[specv1.SupersedeRequest]) (*connect.Response[specv1.SupersedeResponse], error) {
	if req.Msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}
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
	stage := req.Msg.Stage
	valid := false
	for _, s := range authoring.AllStages() {
		if s == stage {
			valid = true
			break
		}
	}
	if !valid {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown stage %q", stage))
	}
	return connect.NewResponse(&specv1.GetPromptsResponse{
		Prompts: authoring.PromptsToProto(stage),
	}), nil
}

// RegisterAuthoringService registers the AuthoringService on the given mux.
func RegisterAuthoringService(mux *http.ServeMux, authoringStore storage.AuthoringBackend, backend storage.Backend) {
	handler := &AuthoringHandler{store: authoringStore, backend: backend}
	path, h := specgraphv1connect.NewAuthoringServiceHandler(handler)
	mux.Handle(path, h)
}

// --- Proto → Domain mappers ---

func sparkOutputToDomain(p *specv1.SparkOutput) *storage.SparkOutput {
	return &storage.SparkOutput{
		Seed:       p.GetSeed(),
		Signal:     p.GetSignal(),
		Questions:  p.GetQuestions(),
		ScopeSniff: p.GetScopeSniff(),
		KillTest:   p.GetKillTest(),
	}
}

func shapeOutputToDomain(p *specv1.ShapeOutput) *storage.ShapeOutput {
	approaches := make([]storage.Approach, len(p.GetApproaches()))
	for i, a := range p.GetApproaches() {
		approaches[i] = storage.Approach{
			Name:        a.GetName(),
			Description: a.GetDescription(),
			Tradeoffs:   a.GetTradeoffs(),
		}
	}
	return &storage.ShapeOutput{
		ScopeIn:        p.GetScopeIn(),
		ScopeOut:       p.GetScopeOut(),
		Approaches:     approaches,
		ChosenApproach: p.GetChosenApproach(),
		Risks:          p.GetRisks(),
		SuccessMust:    p.GetSuccessMust(),
		SuccessShould:  p.GetSuccessShould(),
		SuccessWont:    p.GetSuccessWont(),
		Decisions:      p.GetDecisions(),
	}
}

func specifyOutputToDomain(p *specv1.SpecifyOutput) *storage.SpecifyOutput {
	return &storage.SpecifyOutput{
		InterfaceContract: p.GetInterfaceContract(),
		VerifyCriteria:    p.GetVerifyCriteria(),
		Invariants:        p.GetInvariants(),
		Touches:           p.GetTouches(),
	}
}

var decomposeStrategyMap = map[specv1.DecompositionStrategy]storage.DecompositionStrategy{
	specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE: storage.StrategyVerticalSlice,
	specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_LAYER_CAKE:     storage.StrategyLayerCake,
	specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_SINGLE_UNIT:    storage.StrategySingleUnit,
}

func decomposeOutputToDomain(p *specv1.DecomposeOutput) *storage.DecomposeOutput {
	slices := make([]storage.DecomposeSlice, len(p.GetSlices()))
	for i, s := range p.GetSlices() {
		slices[i] = storage.DecomposeSlice{
			ID:        s.GetId(),
			Intent:    s.GetIntent(),
			Verify:    s.GetVerify(),
			Touches:   s.GetTouches(),
			DependsOn: s.GetDependsOn(),
		}
	}
	return &storage.DecomposeOutput{
		Strategy: decomposeStrategyMap[p.GetStrategy()],
		Slices:   slices,
	}
}

// --- Stage mapping ---

var stageToProto = map[string]specv1.AuthoringStage{
	authoring.StageSpark:     specv1.AuthoringStage_AUTHORING_STAGE_SPARK,
	authoring.StageShape:     specv1.AuthoringStage_AUTHORING_STAGE_SHAPE,
	authoring.StageSpecify:   specv1.AuthoringStage_AUTHORING_STAGE_SPECIFY,
	authoring.StageDecompose: specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE,
	authoring.StageApproved:  specv1.AuthoringStage_AUTHORING_STAGE_APPROVED,
}

var protoToStage = map[specv1.AuthoringStage]string{
	specv1.AuthoringStage_AUTHORING_STAGE_SPARK:     authoring.StageSpark,
	specv1.AuthoringStage_AUTHORING_STAGE_SHAPE:     authoring.StageShape,
	specv1.AuthoringStage_AUTHORING_STAGE_SPECIFY:   authoring.StageSpecify,
	specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE: authoring.StageDecompose,
	specv1.AuthoringStage_AUTHORING_STAGE_APPROVED:  authoring.StageApproved,
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
