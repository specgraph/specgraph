// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/seanb4t/specgraph/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var validSlugRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9_/-]*[a-z0-9])?$`)

const maxSlugLength = 256

// maxFieldLen caps free-text RPC fields to prevent unbounded writes to graph storage.
const maxFieldLen = 10000

func validateSlug(slug string) error {
	if slug == "" {
		return errors.New("slug is required")
	}
	if len(slug) > maxSlugLength {
		return fmt.Errorf("slug exceeds maximum length of %d characters", maxSlugLength)
	}
	if strings.Contains(slug, "..") {
		return fmt.Errorf("slug %q contains path traversal", slug)
	}
	if !validSlugRe.MatchString(slug) {
		return fmt.Errorf("slug %q contains invalid characters", slug)
	}
	return nil
}

// AuthoringHandler implements the ConnectRPC AuthoringService.
// When txBackend is non-nil, multi-step RPCs (Spark, Shape, Specify, Decompose)
// wrap their operations in a transaction for atomicity. When nil, operations
// execute sequentially without rollback on partial failure.
type AuthoringHandler struct {
	store     storage.AuthoringBackend
	backend   storage.Backend
	txBackend storage.TransactionalBackend // optional, may be nil
}

var _ specgraphv1connect.AuthoringServiceHandler = (*AuthoringHandler)(nil)

// Spark handles the Spark RPC, creating a new spec and entering the spark stage.
func (h *AuthoringHandler) Spark(ctx context.Context, req *connect.Request[specv1.SparkRequest]) (*connect.Response[specv1.SparkResponse], error) {
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.Output == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("output is required"))
	}
	if msg.Output.GetSeed() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("output.seed is required"))
	}
	if len(msg.Output.GetSeed()) > maxFieldLen {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("seed exceeds maximum length of %d characters", maxFieldLen))
	}
	// CreateSpec sets stage to "spark" as part of spec creation; no separate
	// TransitionStage call is needed because the initial stage is set atomically.
	sparkDomain := sparkOutputToDomain(msg.Output)
	if err := h.runInTxOrSequential(ctx,
		func(c context.Context) error {
			if _, err := h.backend.CreateSpec(c, msg.Slug, msg.Output.GetSeed(), defaultSpecPriority, defaultSpecComplexity); err != nil {
				return fmt.Errorf("create spec: %w", err)
			}
			return nil
		},
		func(c context.Context) error {
			if err := h.store.StoreSparkOutput(c, msg.Slug, sparkDomain); err != nil {
				return fmt.Errorf("store spark output: %w", err)
			}
			return nil
		},
	); err != nil {
		if errors.Is(err, storage.ErrSpecAlreadyExists) {
			return nil, connect.NewError(connect.CodeAlreadyExists, err)
		}
		return nil, h.stageError(err)
	}
	safetyFlags := authoring.RunSafetyNet(&authoring.SafetyInput{Text: msg.Output.GetSeed()})
	if len(safetyFlags) > 0 {
		if err := h.store.StoreSafetyFlags(ctx, msg.Slug, safetyFlagsToStorage(safetyFlags)); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("store safety flags: %w", err))
		}
	}
	return connect.NewResponse(&specv1.SparkResponse{
		Output:      msg.Output,
		SafetyFlags: authoring.SafetyResultsToProto(safetyFlags),
		NextPrompts: authoring.PromptsToProto(authoring.StageShape),
	}), nil
}

// Shape handles the Shape RPC, transitioning from spark to shape stage.
func (h *AuthoringHandler) Shape(ctx context.Context, req *connect.Request[specv1.ShapeRequest]) (*connect.Response[specv1.ShapeResponse], error) {
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.Output == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("output is required"))
	}
	shapeDomain := shapeOutputToDomain(msg.Output)
	if err := h.runInTxOrSequential(ctx,
		func(c context.Context) error {
			return h.store.TransitionStage(c, msg.Slug, storage.AuthoringStage(authoring.StageSpark), storage.AuthoringStage(authoring.StageShape))
		},
		func(c context.Context) error {
			return h.store.StoreShapeOutput(c, msg.Slug, shapeDomain)
		},
	); err != nil {
		return nil, h.stageError(err)
	}
	scope := make([]string, 0, len(msg.Output.GetScopeIn())+len(msg.Output.GetScopeOut()))
	scope = append(scope, msg.Output.GetScopeIn()...)
	scope = append(scope, msg.Output.GetScopeOut()...)
	// SafetyInput.Text accepts stage-appropriate content; in Shape
	// we scan risks since the spec seed was already checked in Spark.
	safetyFlags := authoring.RunSafetyNet(&authoring.SafetyInput{
		Text:  strings.Join(msg.Output.GetRisks(), " "),
		Scope: scope,
	})
	if len(safetyFlags) > 0 {
		if err := h.store.StoreSafetyFlags(ctx, msg.Slug, safetyFlagsToStorage(safetyFlags)); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("store safety flags: %w", err))
		}
	}
	return connect.NewResponse(&specv1.ShapeResponse{
		Output:      msg.Output,
		SafetyFlags: authoring.SafetyResultsToProto(safetyFlags),
		NextPrompts: authoring.PromptsToProto(authoring.StageSpecify),
	}), nil
}

// Specify handles the Specify RPC, transitioning from shape to specify stage.
func (h *AuthoringHandler) Specify(ctx context.Context, req *connect.Request[specv1.SpecifyRequest]) (*connect.Response[specv1.SpecifyResponse], error) {
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.Output == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("output is required"))
	}
	if len(msg.Output.GetInterfaceContract()) > maxFieldLen {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("interface_contract exceeds maximum length of %d characters", maxFieldLen))
	}
	specifyDomain := specifyOutputToDomain(msg.Output)
	if err := h.runInTxOrSequential(ctx,
		func(c context.Context) error {
			return h.store.TransitionStage(c, msg.Slug, storage.AuthoringStage(authoring.StageShape), storage.AuthoringStage(authoring.StageSpecify))
		},
		func(c context.Context) error {
			return h.store.StoreSpecifyOutput(c, msg.Slug, specifyDomain)
		},
	); err != nil {
		return nil, h.stageError(err)
	}
	safetyFlags := authoring.RunSafetyNet(&authoring.SafetyInput{
		Text:       msg.Output.GetInterfaceContract(),
		Invariants: msg.Output.GetInvariants(),
	})
	if len(safetyFlags) > 0 {
		if err := h.store.StoreSafetyFlags(ctx, msg.Slug, safetyFlagsToStorage(safetyFlags)); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("store safety flags: %w", err))
		}
	}
	return connect.NewResponse(&specv1.SpecifyResponse{
		Output:      msg.Output,
		SafetyFlags: authoring.SafetyResultsToProto(safetyFlags),
		NextPrompts: authoring.PromptsToProto(authoring.StageDecompose),
	}), nil
}

// Decompose handles the Decompose RPC, transitioning from specify to decompose stage.
func (h *AuthoringHandler) Decompose(ctx context.Context, req *connect.Request[specv1.DecomposeRequest]) (*connect.Response[specv1.DecomposeResponse], error) {
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.Output == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("output is required"))
	}
	if msg.Output.GetStrategy() == specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("strategy is required"))
	}
	for _, s := range msg.Output.GetSlices() {
		if len(s.GetIntent()) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slice %q intent exceeds maximum length of %d characters", s.GetId(), maxFieldLen))
		}
	}
	decomposeDomain, domainErr := decomposeOutputToDomain(msg.Output)
	if domainErr != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, domainErr)
	}
	if err := h.runInTxOrSequential(ctx,
		func(c context.Context) error {
			return h.store.TransitionStage(c, msg.Slug, storage.AuthoringStage(authoring.StageSpecify), storage.AuthoringStage(authoring.StageDecompose))
		},
		func(c context.Context) error {
			if _, err := h.store.StoreDecomposeOutput(c, msg.Slug, decomposeDomain); err != nil {
				return fmt.Errorf("store decompose output: %w", err)
			}
			return nil
		},
	); err != nil {
		return nil, h.stageError(err)
	}
	// Collect slice intents for safety scanning.
	var sliceIntents []string
	for _, s := range msg.Output.GetSlices() {
		sliceIntents = append(sliceIntents, s.GetIntent())
	}
	safetyFlags := authoring.RunSafetyNet(&authoring.SafetyInput{
		Text: strings.Join(sliceIntents, " "),
	})
	if len(safetyFlags) > 0 {
		if err := h.store.StoreSafetyFlags(ctx, msg.Slug, safetyFlagsToStorage(safetyFlags)); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("store safety flags: %w", err))
		}
	}
	return connect.NewResponse(&specv1.DecomposeResponse{
		Output:      msg.Output,
		SafetyFlags: authoring.SafetyResultsToProto(safetyFlags),
		NextPrompts: authoring.PromptsToProto(authoring.StageApproved),
		// TODO(spgr-34l.33): Wire simplicity_check and constitution_check analytical
		// passes to populate the Decompose response. The constitution subsystem
		// exists (Slice 2) but handler integration is deferred to a later slice.
	}), nil
}

// Approve handles the Approve RPC, transitioning from decompose to approved stage.
func (h *AuthoringHandler) Approve(ctx context.Context, req *connect.Request[specv1.ApproveRequest]) (*connect.Response[specv1.ApproveResponse], error) {
	if err := validateSlug(req.Msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.store.TransitionStage(ctx, req.Msg.Slug, storage.AuthoringStage(authoring.StageDecompose), storage.AuthoringStage(authoring.StageApproved)); err != nil {
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
	if err := validateSlug(req.Msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if req.Msg.Reason == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reason is required"))
	}
	if len(req.Msg.Reason) > maxFieldLen {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("reason exceeds maximum length of %d characters", maxFieldLen))
	}
	if req.Msg.TargetStage == specv1.AuthoringStage_AUTHORING_STAGE_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target_stage is required"))
	}
	targetStage, ok := protoToStage[req.Msg.TargetStage]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown target_stage %v", req.Msg.TargetStage))
	}
	result, err := h.store.AmendSpec(ctx, req.Msg.Slug, req.Msg.Reason, targetStage)
	if err != nil {
		return nil, h.stageError(err)
	}
	protoStage, ok := stageToProto[string(result.Stage)]
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("unknown stage %q returned from storage", result.Stage))
	}
	return connect.NewResponse(&specv1.AmendResponse{
		Slug:    result.Slug,
		Stage:   protoStage,
		Version: result.Version,
	}), nil
}

// Supersede handles the Supersede RPC, marking a spec as replaced by another.
func (h *AuthoringHandler) Supersede(ctx context.Context, req *connect.Request[specv1.SupersedeRequest]) (*connect.Response[specv1.SupersedeResponse], error) {
	if err := validateSlug(req.Msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if req.Msg.SupersededBy == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("superseded_by is required"))
	}
	if err := validateSlug(req.Msg.SupersededBy); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("superseded_by: %w", err))
	}
	if req.Msg.Slug == req.Msg.SupersededBy {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a spec cannot supersede itself"))
	}
	if err := h.store.SupersedeSpec(ctx, req.Msg.Slug, req.Msg.SupersededBy, req.Msg.Reason); err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("supersede: %w", err))
	}
	return connect.NewResponse(&specv1.SupersedeResponse{
		Slug:         req.Msg.Slug,
		SupersededBy: req.Msg.SupersededBy,
	}), nil
}

// GetPrompts handles the GetPrompts RPC, returning prompt templates for a stage.
func (h *AuthoringHandler) GetPrompts(_ context.Context, req *connect.Request[specv1.GetPromptsRequest]) (*connect.Response[specv1.GetPromptsResponse], error) {
	if req.Msg.Stage == specv1.AuthoringStage_AUTHORING_STAGE_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("stage is required"))
	}
	stage, ok := protoToStage[req.Msg.Stage]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown stage %v", req.Msg.Stage))
	}
	prompts := authoring.PromptsToProto(string(stage))
	if len(prompts) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("no prompts defined for stage %q", stage))
	}
	return connect.NewResponse(&specv1.GetPromptsResponse{
		Prompts: prompts,
	}), nil
}

// RegisterAuthoringService registers the AuthoringService on the given mux.
func RegisterAuthoringService(mux *http.ServeMux, authoringStore storage.AuthoringBackend, backend storage.Backend) {
	if authoringStore == nil {
		panic("RegisterAuthoringService: authoringStore must not be nil")
	}
	if backend == nil {
		panic("RegisterAuthoringService: backend must not be nil")
	}
	handler := &AuthoringHandler{store: authoringStore, backend: backend}
	// If the backend supports transactions, enable atomic multi-operation RPCs.
	if txb, ok := backend.(storage.TransactionalBackend); ok {
		handler.txBackend = txb
	}
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

func decomposeOutputToDomain(p *specv1.DecomposeOutput) (*storage.DecomposeOutput, error) {
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
	strategy, ok := decomposeStrategyMap[p.GetStrategy()]
	if !ok {
		return nil, fmt.Errorf("unknown decomposition strategy: %v", p.GetStrategy())
	}
	return &storage.DecomposeOutput{
		Strategy: strategy,
		Slices:   slices,
	}, nil
}

// --- Stage mapping ---

var stageToProto = map[string]specv1.AuthoringStage{
	authoring.StageSpark:     specv1.AuthoringStage_AUTHORING_STAGE_SPARK,
	authoring.StageShape:     specv1.AuthoringStage_AUTHORING_STAGE_SHAPE,
	authoring.StageSpecify:   specv1.AuthoringStage_AUTHORING_STAGE_SPECIFY,
	authoring.StageDecompose: specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE,
	authoring.StageApproved:  specv1.AuthoringStage_AUTHORING_STAGE_APPROVED,
}

var protoToStage = map[specv1.AuthoringStage]storage.AuthoringStage{
	specv1.AuthoringStage_AUTHORING_STAGE_SPARK:     storage.AuthoringStage(authoring.StageSpark),
	specv1.AuthoringStage_AUTHORING_STAGE_SHAPE:     storage.AuthoringStage(authoring.StageShape),
	specv1.AuthoringStage_AUTHORING_STAGE_SPECIFY:   storage.AuthoringStage(authoring.StageSpecify),
	specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE: storage.AuthoringStage(authoring.StageDecompose),
	specv1.AuthoringStage_AUTHORING_STAGE_APPROVED:  storage.AuthoringStage(authoring.StageApproved),
}

// runInTxOrSequential runs the given operations within a transaction if txBackend
// is available, otherwise runs them sequentially. This eliminates the txBackend
// if/else duplication across RPC handlers.
func (h *AuthoringHandler) runInTxOrSequential(ctx context.Context, ops ...func(context.Context) error) error {
	if h.txBackend != nil {
		if err := h.txBackend.RunInTransaction(ctx, func(txCtx context.Context) error {
			for _, op := range ops {
				if err := op(txCtx); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("transaction: %w", err)
		}
		return nil
	}
	for _, op := range ops {
		if err := op(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (h *AuthoringHandler) stageError(err error) error {
	if errors.Is(err, storage.ErrSpecAlreadyApproved) {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if errors.Is(err, storage.ErrInvalidStageTransition) {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if errors.Is(err, storage.ErrSpecNotFound) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}

// safetyFlagsToStorage converts domain-level safety results to storage types.
func safetyFlagsToStorage(flags []authoring.SafetyFlagResult) []storage.SafetyFlag {
	out := make([]storage.SafetyFlag, len(flags))
	for i, f := range flags {
		out[i] = storage.SafetyFlag{
			Category:    authoring.ToStorageCategory(f.Category),
			Severity:    authoring.ToStorageSeverity(f.Severity),
			Description: f.Description,
		}
	}
	return out
}
