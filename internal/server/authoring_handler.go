// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
		return errors.New("slug contains path traversal")
	}
	if !validSlugRe.MatchString(slug) {
		return errors.New("slug contains invalid characters")
	}
	return nil
}

// AuthoringHandler implements the ConnectRPC AuthoringService.
// When txBackend is non-nil, multi-step RPCs (Spark, Shape, Specify, Decompose)
// wrap their operations in a transaction for atomicity. When nil, operations
// execute sequentially without rollback on partial failure.
type AuthoringHandler struct {
	store           storage.AuthoringBackend
	backend         storage.Backend
	txBackend       storage.TransactionalBackend // optional, may be nil
	decisionBackend storage.DecisionBackend      // optional; when non-nil, Approve transitions linked decisions
	graphBackend    storage.GraphBackend         // optional; used to discover decision→spec INFORMS edges
	logger          *slog.Logger
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
	safetyInput := &authoring.SafetyInput{Text: msg.Output.GetSeed()}
	var safetyFlags []authoring.SafetyFlagResult
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
		func(c context.Context) error {
			if err := safetyInput.Validate(); err != nil {
				return connect.NewError(connect.CodeInvalidArgument, err)
			}
			safetyFlags = authoring.RunSafetyNet(safetyInput)
			if len(safetyFlags) > 0 {
				if err := h.store.StoreSafetyFlags(c, msg.Slug, safetyFlagsToStorage(safetyFlags)); err != nil {
					return fmt.Errorf("store safety flags: %w", err)
				}
			}
			return nil
		},
	); err != nil {
		if errors.Is(err, storage.ErrSpecAlreadyExists) {
			return nil, connect.NewError(connect.CodeAlreadyExists, err)
		}
		return nil, h.stageError(err)
	}
	// Output is returned as-is from the client request. The storage layer stores
	// domain-typed stage outputs (via StoreSparkOutput) but does not provide a
	// proto-typed round-trip getter. This is acceptable because the output was
	// just persisted in the same request and has not been enriched server-side.
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
	for _, item := range msg.Output.GetScopeIn() {
		if len(item) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope_in item exceeds maximum length of %d characters", maxFieldLen))
		}
	}
	for _, item := range msg.Output.GetScopeOut() {
		if len(item) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("scope_out item exceeds maximum length of %d characters", maxFieldLen))
		}
	}
	shapeDomain := shapeOutputToDomain(msg.Output)
	scope := make([]string, 0, len(msg.Output.GetScopeIn())+len(msg.Output.GetScopeOut()))
	scope = append(scope, msg.Output.GetScopeIn()...)
	scope = append(scope, msg.Output.GetScopeOut()...)
	// SafetyInput.Text accepts stage-appropriate content; in Shape
	// we scan risks since the spec seed was already checked in Spark.
	safetyInput := &authoring.SafetyInput{
		Text:  strings.Join(msg.Output.GetRisks(), " "),
		Scope: scope,
	}
	var safetyFlags []authoring.SafetyFlagResult
	if err := h.runInTxOrSequential(ctx,
		func(c context.Context) error {
			return h.store.TransitionStage(c, msg.Slug, storage.AuthoringStage(authoring.StageSpark), storage.AuthoringStage(authoring.StageShape))
		},
		func(c context.Context) error {
			return h.store.StoreShapeOutput(c, msg.Slug, shapeDomain)
		},
		func(c context.Context) error {
			if err := safetyInput.Validate(); err != nil {
				return connect.NewError(connect.CodeInvalidArgument, err)
			}
			safetyFlags = authoring.RunSafetyNet(safetyInput)
			if len(safetyFlags) > 0 {
				if err := h.store.StoreSafetyFlags(c, msg.Slug, safetyFlagsToStorage(safetyFlags)); err != nil {
					return fmt.Errorf("store safety flags: %w", err)
				}
			}
			return nil
		},
	); err != nil {
		return nil, h.stageError(err)
	}
	peripheralVision, _, _, _ := runAnalyticalPasses(authoring.StageShape, msg.Posture)
	// Output is returned as-is from the client request. See Spark handler comment.
	return connect.NewResponse(&specv1.ShapeResponse{
		Output:           msg.Output,
		PeripheralVision: peripheralVision,
		SafetyFlags:      authoring.SafetyResultsToProto(safetyFlags),
		NextPrompts:      authoring.PromptsToProto(authoring.StageSpecify),
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
	for _, item := range msg.Output.GetVerifyCriteria() {
		if len(item) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("verify_criteria item exceeds maximum length of %d characters", maxFieldLen))
		}
	}
	for _, item := range msg.Output.GetInvariants() {
		if len(item) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invariants item exceeds maximum length of %d characters", maxFieldLen))
		}
	}
	specifyDomain := specifyOutputToDomain(msg.Output)
	safetyInput := &authoring.SafetyInput{
		Text:       msg.Output.GetInterfaceContract(),
		Invariants: msg.Output.GetInvariants(),
	}
	var safetyFlags []authoring.SafetyFlagResult
	if err := h.runInTxOrSequential(ctx,
		func(c context.Context) error {
			return h.store.TransitionStage(c, msg.Slug, storage.AuthoringStage(authoring.StageShape), storage.AuthoringStage(authoring.StageSpecify))
		},
		func(c context.Context) error {
			return h.store.StoreSpecifyOutput(c, msg.Slug, specifyDomain)
		},
		func(c context.Context) error {
			if err := safetyInput.Validate(); err != nil {
				return connect.NewError(connect.CodeInvalidArgument, err)
			}
			safetyFlags = authoring.RunSafetyNet(safetyInput)
			if len(safetyFlags) > 0 {
				if err := h.store.StoreSafetyFlags(c, msg.Slug, safetyFlagsToStorage(safetyFlags)); err != nil {
					return fmt.Errorf("store safety flags: %w", err)
				}
			}
			return nil
		},
	); err != nil {
		return nil, h.stageError(err)
	}
	_, redTeam, consistencyIssues, _ := runAnalyticalPasses(authoring.StageSpecify, msg.Posture)
	// Output is returned as-is from the client request. See Spark handler comment.
	return connect.NewResponse(&specv1.SpecifyResponse{
		Output:            msg.Output,
		RedTeam:           redTeam,
		ConsistencyIssues: consistencyIssues,
		SafetyFlags:       authoring.SafetyResultsToProto(safetyFlags),
		NextPrompts:       authoring.PromptsToProto(authoring.StageDecompose),
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
	// Collect slice intents for safety scanning.
	var intentBuilder strings.Builder
	for i, s := range msg.Output.GetSlices() {
		if i > 0 {
			intentBuilder.WriteByte(' ')
		}
		intentBuilder.WriteString(s.GetIntent())
	}
	safetyInput := &authoring.SafetyInput{
		Text: intentBuilder.String(),
	}
	var safetyFlags []authoring.SafetyFlagResult
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
		func(c context.Context) error {
			if err := safetyInput.Validate(); err != nil {
				return connect.NewError(connect.CodeInvalidArgument, err)
			}
			safetyFlags = authoring.RunSafetyNet(safetyInput)
			if len(safetyFlags) > 0 {
				if err := h.store.StoreSafetyFlags(c, msg.Slug, safetyFlagsToStorage(safetyFlags)); err != nil {
					return fmt.Errorf("store safety flags: %w", err)
				}
			}
			return nil
		},
	); err != nil {
		return nil, h.stageError(err)
	}
	_, _, _, simplicity := runAnalyticalPasses(authoring.StageDecompose, msg.Posture)
	// Output is returned as-is from the client request. See Spark handler comment.
	return connect.NewResponse(&specv1.DecomposeResponse{
		Output:      msg.Output,
		Simplicity:  simplicity,
		SafetyFlags: authoring.SafetyResultsToProto(safetyFlags),
		NextPrompts: authoring.PromptsToProto(authoring.StageApproved),
	}), nil
}

// Approve handles the Approve RPC, transitioning from decompose to approved stage.
// After approval, linked decisions (via INFORMS edges) are transitioned from
// proposed to accepted per ADR-003 (decisions as first-class graph nodes).
func (h *AuthoringHandler) Approve(ctx context.Context, req *connect.Request[specv1.ApproveRequest]) (*connect.Response[specv1.ApproveResponse], error) {
	if err := validateSlug(req.Msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.store.TransitionStage(ctx, req.Msg.Slug, storage.AuthoringStage(authoring.StageDecompose), storage.AuthoringStage(authoring.StageApproved)); err != nil {
		return nil, h.stageError(err)
	}
	// Read back the spec from storage so approved_at reflects the persisted
	// updated_at timestamp set during TransitionStage, rather than being
	// computed at response time.
	spec, err := h.backend.GetSpec(ctx, req.Msg.Slug)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("approve: read back spec: %w", err))
	}
	approvedAt := spec.GetUpdatedAt()
	if approvedAt == nil {
		approvedAt = timestamppb.Now()
	}
	// ADR-003: transition linked decisions from proposed to accepted.
	h.acceptLinkedDecisions(ctx, req.Msg.Slug)
	return connect.NewResponse(&specv1.ApproveResponse{
		Slug:       req.Msg.Slug,
		Stage:      stageToProto(authoring.StageApproved),
		ApprovedAt: approvedAt,
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
	protoStage := stageToProto(authoring.Stage(result.Stage))
	if protoStage == specv1.AuthoringStage_AUTHORING_STAGE_UNSPECIFIED {
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
	prompts := authoring.PromptsToProto(authoring.Stage(stage))
	if len(prompts) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("no prompts defined for stage %q", stage))
	}
	return connect.NewResponse(&specv1.GetPromptsResponse{
		Prompts: prompts,
	}), nil
}

// acceptLinkedDecisions queries for Decision nodes linked to the spec via
// INFORMS edges and transitions them from proposed to accepted. Errors are
// logged but do not fail the Approve RPC — decision acceptance is best-effort
// since the spec approval itself has already succeeded.
func (h *AuthoringHandler) acceptLinkedDecisions(ctx context.Context, slug string) {
	if h.graphBackend == nil || h.decisionBackend == nil {
		return
	}
	edges, err := h.graphBackend.ListEdges(ctx, slug, specv1.EdgeType_EDGE_TYPE_INFORMS)
	if err != nil {
		h.logger.WarnContext(ctx, "acceptLinkedDecisions: failed to list edges",
			slog.String("slug", slug),
			slog.Any("error", err),
		)
		return
	}
	acceptedStatus := specv1.DecisionStatus_DECISION_STATUS_ACCEPTED
	for _, edge := range edges {
		// INFORMS edges: Decision → Spec. ListEdges returns both directions,
		// so the decision slug is the from_id when it differs from the spec slug.
		decisionSlug := edge.GetFromId()
		if decisionSlug == "" || decisionSlug == slug {
			// This edge has the spec as the source — try the to side.
			decisionSlug = edge.GetToId()
			if decisionSlug == "" || decisionSlug == slug {
				continue
			}
		}
		dec, err := h.decisionBackend.GetDecision(ctx, decisionSlug)
		if err != nil {
			h.logger.WarnContext(ctx, "acceptLinkedDecisions: failed to get decision",
				slog.String("slug", slug),
				slog.String("decisionSlug", decisionSlug),
				slog.Any("error", err),
			)
			continue
		}
		if dec.GetStatus() != specv1.DecisionStatus_DECISION_STATUS_PROPOSED {
			continue
		}
		if _, err := h.decisionBackend.UpdateDecision(ctx, decisionSlug, nil, &acceptedStatus, nil, nil, nil); err != nil {
			h.logger.WarnContext(ctx, "acceptLinkedDecisions: failed to update decision",
				slog.String("slug", slug),
				slog.String("decisionSlug", decisionSlug),
				slog.Any("error", err),
			)
		}
	}
}

// RegisterAuthoringService registers the AuthoringService on the given mux.
func RegisterAuthoringService(mux *http.ServeMux, authoringStore storage.AuthoringBackend, backend storage.Backend) {
	if authoringStore == nil {
		panic("RegisterAuthoringService: authoringStore must not be nil")
	}
	if backend == nil {
		panic("RegisterAuthoringService: backend must not be nil")
	}
	handler := &AuthoringHandler{store: authoringStore, backend: backend, logger: slog.Default()}
	// If the backend supports transactions, enable atomic multi-operation RPCs.
	if txb, ok := backend.(storage.TransactionalBackend); ok {
		handler.txBackend = txb
	}
	// If the backend supports decision and graph operations, enable ADR-003
	// decision acceptance on spec approval.
	if db, ok := backend.(storage.DecisionBackend); ok {
		handler.decisionBackend = db
	}
	if gb, ok := backend.(storage.GraphBackend); ok {
		handler.graphBackend = gb
	}
	path, h := specgraphv1connect.NewAuthoringServiceHandler(handler)
	mux.Handle(path, h)
}

// --- Proto → Domain mappers ---

// scopeSniffToStorage maps proto ScopeSniff enum values to their storage string representation.
var scopeSniffToStorage = map[specv1.ScopeSniff]string{
	specv1.ScopeSniff_SCOPE_SNIFF_TINY:   "tiny",
	specv1.ScopeSniff_SCOPE_SNIFF_SMALL:  "small",
	specv1.ScopeSniff_SCOPE_SNIFF_MEDIUM: "medium",
	specv1.ScopeSniff_SCOPE_SNIFF_LARGE:  "large",
	specv1.ScopeSniff_SCOPE_SNIFF_EPIC:   "epic",
}

func sparkOutputToDomain(p *specv1.SparkOutput) *storage.SparkOutput {
	return &storage.SparkOutput{
		Seed:       p.GetSeed(),
		Signal:     p.GetSignal(),
		Questions:  p.GetQuestions(),
		ScopeSniff: scopeSniffToStorage[p.GetScopeSniff()],
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

// stageToProto delegates to the canonical mapping in the authoring package.
// Kept as a local alias so call sites within this file remain unchanged.
func stageToProto(stage authoring.Stage) specv1.AuthoringStage {
	return authoring.StageToProto(stage)
}

var protoToStage = map[specv1.AuthoringStage]storage.AuthoringStage{
	specv1.AuthoringStage_AUTHORING_STAGE_SPARK:     authoring.StageSpark.ToStorage(),
	specv1.AuthoringStage_AUTHORING_STAGE_SHAPE:     authoring.StageShape.ToStorage(),
	specv1.AuthoringStage_AUTHORING_STAGE_SPECIFY:   authoring.StageSpecify.ToStorage(),
	specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE: authoring.StageDecompose.ToStorage(),
	specv1.AuthoringStage_AUTHORING_STAGE_APPROVED:  authoring.StageApproved.ToStorage(),
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
	// If the error is already a connect.Error (e.g. from a safety validation
	// op inside runInTxOrSequential), unwrap and return it as-is so the
	// original code is preserved rather than re-wrapped as CodeInternal.
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		return connErr
	}
	if errors.Is(err, storage.ErrSpecAlreadyApproved) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("spec is already approved"))
	}
	if errors.Is(err, storage.ErrInvalidStageTransition) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("invalid stage transition"))
	}
	if errors.Is(err, storage.ErrSpecNotFound) {
		return connect.NewError(connect.CodeNotFound, errors.New("spec not found"))
	}
	h.logger.Error("stageError: internal error", slog.Any("error", err))
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

// runAnalyticalPasses executes the passes returned by PassesForStage for the
// given stage and posture and converts the results into proto finding types.
// All returned slices contain placeholder data; real LLM-driven pass
// execution is deferred to a later slice.
func runAnalyticalPasses(stage authoring.Stage, posture specv1.Posture) (
	peripheralVision []*specv1.PeripheralVisionItem,
	redTeam []*specv1.RedTeamFinding,
	consistencyIssues []*specv1.ConsistencyIssue,
	simplicity []*specv1.SimplicityFinding,
) {
	for _, name := range authoring.PassesForStage(stage, posture) {
		switch authoring.PassName(name) {
		case authoring.PassPeripheralVision:
			peripheralVision = append(peripheralVision, &specv1.PeripheralVisionItem{
				Item:        "peripheral_vision pass placeholder",
				Disposition: specv1.PeripheralDisposition_PERIPHERAL_DISPOSITION_UNSPECIFIED,
			})
		case authoring.PassRedTeam:
			redTeam = append(redTeam, &specv1.RedTeamFinding{
				Severity: specv1.FindingSeverity_FINDING_SEVERITY_NOTE,
				Finding:  "red_team pass placeholder",
			})
		case authoring.PassConsistencyCheck:
			consistencyIssues = append(consistencyIssues, &specv1.ConsistencyIssue{
				IssueKind:   specv1.IssueKind_ISSUE_KIND_UNSPECIFIED,
				Description: "consistency_check pass placeholder",
			})
		case authoring.PassSimplicityCheck:
			simplicity = append(simplicity, &specv1.SimplicityFinding{
				Area:       "simplicity_check pass placeholder",
				Suggestion: "",
			})
		}
	}
	return
}

// categoryToStorage maps authoring.SafetyCategory to the storage string representation.
var categoryToStorage = map[authoring.SafetyCategory]storage.SafetyCategory{
	authoring.SafetyCategorySecurity: "security",
	authoring.SafetyCategoryDataLoss: "data_loss",
}

// safetyFlagsToStorage converts domain-level safety results to storage types.
func safetyFlagsToStorage(flags []authoring.SafetyFlagResult) []storage.SafetyFlag {
	out := make([]storage.SafetyFlag, len(flags))
	for i, f := range flags {
		out[i] = storage.SafetyFlag{
			Category:    categoryToStorage[f.Category],
			Severity:    authoring.ToStorageSeverity(f.Severity),
			Description: f.Description,
		}
	}
	return out
}
