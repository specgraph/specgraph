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
	"github.com/specgraph/specgraph/internal/authoring"
	"github.com/specgraph/specgraph/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// maxElements caps repeated fields to prevent unbounded writes to graph storage.
const maxElements = 100

// AuthoringHandler implements the ConnectRPC AuthoringService.
// When txBackend is non-nil, multi-step RPCs (Spark, Shape, Specify, Decompose)
// wrap their operations in a transaction for atomicity. When nil, operations
// execute sequentially without rollback on partial failure.
type AuthoringHandler struct {
	scoper storage.Scoper
	logger *slog.Logger
}

var _ specgraphv1connect.AuthoringServiceHandler = (*AuthoringHandler)(nil)

// Spark handles the Spark RPC, creating a new spec and entering the spark stage.
func (h *AuthoringHandler) Spark(ctx context.Context, req *connect.Request[specv1.SparkRequest]) (*connect.Response[specv1.SparkResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
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
	if len(msg.Output.GetQuestions()) > maxElements {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("questions exceeds maximum of %d elements", maxElements))
	}
	// CreateSpec sets stage to "spark" as part of spec creation; no separate
	// TransitionStage call is needed because the initial stage is set atomically.
	sparkDomain, err := sparkOutputToDomain(msg.Output)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	safetyInput := &authoring.SafetyInput{Text: msg.Output.GetSeed()}
	var safetyFlags []authoring.SafetyFlagResult
	if err := runInTxOrSequential(ctx, store,
		func(c context.Context) error {
			if _, err := store.CreateSpec(c, msg.Slug, msg.Output.GetSeed(), defaultSpecPriority, defaultSpecComplexity); err != nil {
				return fmt.Errorf("create spec: %w", err)
			}
			return nil
		},
		func(c context.Context) error {
			if err := store.StoreSparkOutput(c, msg.Slug, sparkDomain); err != nil {
				return fmt.Errorf("store spark output: %w", err)
			}
			return nil
		},
		func(c context.Context) error {
			var err error
			safetyFlags, err = persistSafetyFlags(c, store, msg.Slug, safetyInput)
			return err
		},
	); err != nil {
		if errors.Is(err, storage.ErrSpecAlreadyExists) {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("spec with this slug already exists"))
		}
		return nil, h.stageError(err)
	}
	// Output is echoed from the client request, NOT read back from storage. The
	// storage layer stores domain-typed outputs (via StoreSparkOutput) but does not
	// provide a proto-typed round-trip getter. This means the response reflects the
	// input exactly — any server-side enrichment will require adding a read-back
	// path. TODO(Slice 4): add read-back when output enrichment is implemented.
	return connect.NewResponse(&specv1.SparkResponse{
		Output:      msg.Output,
		SafetyFlags: safetyResultsToProto(safetyFlags),
		NextPrompts: promptsToProto(authoring.StageShape),
	}), nil
}

// Shape handles the Shape RPC, transitioning from spark to shape stage.
func (h *AuthoringHandler) Shape(ctx context.Context, req *connect.Request[specv1.ShapeRequest]) (*connect.Response[specv1.ShapeResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.Output == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("output is required"))
	}
	// Required conversation_exchanges per design §Conversation Recording Coupling.
	if err := authoring.ValidateExchanges(msg.GetConversationExchanges(), "shape"); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// Existing field-size validations (preserved verbatim from current handler).
	for _, v := range []struct {
		name  string
		items []string
	}{
		{"scope_in", msg.Output.GetScopeIn()},
		{"scope_out", msg.Output.GetScopeOut()},
		{"risks", msg.Output.GetRisks()},
		{"success_must", msg.Output.GetSuccessMust()},
		{"success_should", msg.Output.GetSuccessShould()},
		{"success_wont", msg.Output.GetSuccessWont()},
	} {
		if err := validateStringSlice(v.name, v.items, maxElements, maxFieldLen); err != nil {
			return nil, err
		}
	}
	if len(msg.Output.GetApproaches()) > maxElements {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("approaches exceeds maximum of %d elements", maxElements))
	}
	if len(msg.Output.GetDecisions()) > maxElements {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("decisions exceeds maximum of %d elements", maxElements))
	}
	shapeDomain, err := shapeOutputToDomain(msg.Output)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	scope := make([]string, 0, len(msg.Output.GetScopeIn())+len(msg.Output.GetScopeOut()))
	scope = append(scope, msg.Output.GetScopeIn()...)
	scope = append(scope, msg.Output.GetScopeOut()...)
	safetyInput := &authoring.SafetyInput{
		Text:  strings.Join(msg.Output.GetRisks(), " "),
		Scope: scope,
	}
	var safetyFlags []authoring.SafetyFlagResult
	exchanges := exchangesFromProto(msg.GetConversationExchanges())
	entry := buildConversationEntry(storage.SpecStageShape, msg.GetPosture(), exchanges, false /* isAmend */)

	// Posture-absent warning (design §Posture).
	if msg.GetPosture() == specv1.Posture_POSTURE_UNSPECIFIED {
		h.logger.Warn("posture-absent", slog.String("stage", "shape"), slog.String("slug", msg.Slug))
	}

	// Four-op transaction: transition → store output → safety → record conversation.
	// All four land in one transaction via RunInTransaction's context threading;
	// any op failing rolls back all prior writes.
	if err := runInTxOrSequential(ctx, store,
		func(c context.Context) error {
			return store.TransitionStage(c, msg.Slug, storage.SpecStageSpark, storage.SpecStageShape)
		},
		func(c context.Context) error {
			return store.StoreShapeOutput(c, msg.Slug, shapeDomain)
		},
		func(c context.Context) error {
			var err error
			safetyFlags, err = persistSafetyFlags(c, store, msg.Slug, safetyInput)
			return err
		},
		func(c context.Context) error {
			_, err := store.RecordConversation(c, msg.Slug, entry)
			return err
		},
	); err != nil {
		return nil, h.stageError(err)
	}
	// Output is returned as-is from the client request. See Spark handler comment.
	return connect.NewResponse(&specv1.ShapeResponse{
		Output:      msg.Output,
		SafetyFlags: safetyResultsToProto(safetyFlags),
		NextPrompts: promptsToProto(authoring.StageSpecify),
	}), nil
}

// Specify handles the Specify RPC, transitioning from shape to specify stage.
func (h *AuthoringHandler) Specify(ctx context.Context, req *connect.Request[specv1.SpecifyRequest]) (*connect.Response[specv1.SpecifyResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.Output == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("output is required"))
	}
	if len(msg.Output.GetInterfaces()) > maxElements {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("interfaces exceeds maximum of %d elements", maxElements))
	}
	for i, iface := range msg.Output.GetInterfaces() {
		if iface.GetName() == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("interfaces[%d]: name is required", i))
		}
		if len(iface.GetName()) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("interfaces[%d]: name exceeds maximum length of %d characters", i, maxFieldLen))
		}
		if len(iface.GetBody()) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("interfaces[%d]: body exceeds maximum length of %d characters", i, maxFieldLen))
		}
	}
	if len(msg.Output.GetVerifyCriteria()) > maxElements {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("verify_criteria exceeds maximum of %d elements", maxElements))
	}
	for i, vc := range msg.Output.GetVerifyCriteria() {
		if len(vc.GetCategory()) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("verify_criteria[%d]: category exceeds maximum length of %d characters", i, maxFieldLen))
		}
		if vc.GetDescription() == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("verify_criteria[%d]: description is required", i))
		}
		if len(vc.GetDescription()) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("verify_criteria[%d]: description exceeds maximum length of %d characters", i, maxFieldLen))
		}
	}
	if err := validateStringSlice("invariants", msg.Output.GetInvariants(), maxElements, maxFieldLen); err != nil {
		return nil, err
	}
	if len(msg.Output.GetTouches()) > maxElements {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("touches exceeds maximum of %d elements", maxElements))
	}
	for i, ft := range msg.Output.GetTouches() {
		if ft.GetPath() == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("touches[%d]: path is required", i))
		}
		if len(ft.GetPath()) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("touches[%d]: path exceeds maximum length of %d characters", i, maxFieldLen))
		}
		if len(ft.GetPurpose()) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("touches[%d]: purpose exceeds maximum length of %d characters", i, maxFieldLen))
		}
		if len(ft.GetChangeType()) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("touches[%d]: change_type exceeds maximum length of %d characters", i, maxFieldLen))
		}
	}
	specifyDomain := specifyOutputToDomain(msg.Output)
	var contractText strings.Builder
	for _, iface := range msg.Output.GetInterfaces() {
		if contractText.Len() > 0 {
			contractText.WriteString("\n\n")
		}
		contractText.WriteString(iface.GetName() + ":\n" + iface.GetBody())
	}
	safetyInput := &authoring.SafetyInput{
		Text:       contractText.String(),
		Invariants: msg.Output.GetInvariants(),
	}
	var safetyFlags []authoring.SafetyFlagResult
	if err := runInTxOrSequential(ctx, store,
		func(c context.Context) error {
			return store.TransitionStage(c, msg.Slug, storage.SpecStageShape, storage.SpecStageSpecify)
		},
		func(c context.Context) error {
			return store.StoreSpecifyOutput(c, msg.Slug, specifyDomain)
		},
		func(c context.Context) error {
			var err error
			safetyFlags, err = persistSafetyFlags(c, store, msg.Slug, safetyInput)
			return err
		},
	); err != nil {
		return nil, h.stageError(err)
	}
	// Output is returned as-is from the client request. See Spark handler comment.
	return connect.NewResponse(&specv1.SpecifyResponse{
		Output:      msg.Output,
		SafetyFlags: safetyResultsToProto(safetyFlags),
		NextPrompts: promptsToProto(authoring.StageDecompose),
	}), nil
}

// Decompose handles the Decompose RPC, transitioning from specify to decompose stage.
func (h *AuthoringHandler) Decompose(ctx context.Context, req *connect.Request[specv1.DecomposeRequest]) (*connect.Response[specv1.DecomposeResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
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
	if len(msg.Output.GetSlices()) > maxElements {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slices exceeds maximum of %d elements", maxElements))
	}
	for _, s := range msg.Output.GetSlices() {
		if len(s.GetIntent()) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slice %q intent exceeds maximum length of %d characters", s.GetId(), maxFieldLen))
		}
	}
	if msg.Output.GetStrategy() == specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD {
		if err := validateSteelThread(msg.Output.GetSlices()); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
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
	var childSlugs []string
	if err := runInTxOrSequential(ctx, store,
		func(c context.Context) error {
			return store.TransitionStage(c, msg.Slug, storage.SpecStageSpecify, storage.SpecStageDecompose)
		},
		func(c context.Context) error {
			slugs, err := store.StoreDecomposeOutput(c, msg.Slug, decomposeDomain)
			if err != nil {
				return fmt.Errorf("store decompose output: %w", err)
			}
			childSlugs = slugs
			return nil
		},
		func(c context.Context) error {
			var err error
			safetyFlags, err = persistSafetyFlags(c, store, msg.Slug, safetyInput)
			return err
		},
	); err != nil {
		return nil, h.stageError(err)
	}
	// Output is returned as-is from the client request. See Spark handler comment.
	return connect.NewResponse(&specv1.DecomposeResponse{
		Output:      msg.Output,
		SafetyFlags: safetyResultsToProto(safetyFlags),
		NextPrompts: promptsToProto(authoring.StageApproved),
		SliceSlugs:  childSlugs,
	}), nil
}

// Approve handles the Approve RPC, transitioning from decompose to approved stage.
// After approval, linked decisions (via DECIDED_IN edges) are transitioned from
// proposed to accepted per ADR-003 (decisions as first-class graph nodes).
func (h *AuthoringHandler) Approve(ctx context.Context, req *connect.Request[specv1.ApproveRequest]) (*connect.Response[specv1.ApproveResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// Wrap TransitionStage and acceptLinkedDecisions in a transaction so that
	// if decision promotion fails, the spec approval is rolled back.
	var spec *storage.Spec
	slug := req.Msg.Slug
	if err := runInTxOrSequential(ctx, store,
		func(txCtx context.Context) error {
			return store.TransitionStage(txCtx, slug, storage.SpecStageDecompose, storage.SpecStageApproved)
		},
		func(txCtx context.Context) error {
			return acceptLinkedDecisions(txCtx, h.logger, store, store, slug)
		},
		func(txCtx context.Context) error {
			var err error
			spec, err = store.GetSpec(txCtx, slug)
			if err != nil {
				return fmt.Errorf("get spec %q: %w", slug, err)
			}
			return nil
		},
	); err != nil {
		return nil, h.stageError(err)
	}
	if spec.UpdatedAt.IsZero() {
		h.logger.ErrorContext(ctx, "spec.UpdatedAt is zero after TransitionStage",
			"slug", slug, "specID", spec.ID, "stage", "approved")
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	approvedAt := timestamppb.New(spec.UpdatedAt)
	return connect.NewResponse(&specv1.ApproveResponse{
		Slug:       req.Msg.Slug,
		Stage:      stageToProto(storage.SpecStageApproved),
		ApprovedAt: approvedAt,
	}), nil
}

// Amend handles the Amend RPC, rolling a spec back to an earlier stage.
func (h *AuthoringHandler) Amend(ctx context.Context, req *connect.Request[specv1.AmendRequest]) (*connect.Response[specv1.AmendResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
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
	if req.Msg.TargetStage == specv1.AuthoringStage_AUTHORING_STAGE_APPROVED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("approved specs cannot be amended to approved; use Approve RPC"))
	}
	targetStage, ok := protoToStage[req.Msg.TargetStage]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown target_stage %v", req.Msg.TargetStage))
	}
	result, err := store.AmendSpec(ctx, req.Msg.Slug, req.Msg.Reason, targetStage)
	if err != nil {
		return nil, h.stageError(err)
	}
	protoStage := stageToProto(result.Stage)
	if protoStage == specv1.AuthoringStage_AUTHORING_STAGE_UNSPECIFIED {
		h.logger.Error("unknown stage returned from storage", slog.String("stage", string(result.Stage)), slog.String("slug", req.Msg.Slug))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	return connect.NewResponse(&specv1.AmendResponse{
		Slug:    result.Slug,
		Stage:   protoStage,
		Version: result.Version,
	}), nil
}

// Supersede handles the Supersede RPC, marking a spec as replaced by another.
func (h *AuthoringHandler) Supersede(ctx context.Context, req *connect.Request[specv1.SupersedeRequest]) (*connect.Response[specv1.SupersedeResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
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
	if req.Msg.Reason == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reason is required for supersede"))
	}
	if len(req.Msg.Reason) > maxFieldLen {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("reason exceeds maximum length of %d", maxFieldLen))
	}
	if err := store.SupersedeSpec(ctx, req.Msg.Slug, req.Msg.SupersededBy, req.Msg.Reason); err != nil {
		return nil, h.stageError(err)
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
	// Approved specs have completed the authoring funnel — return empty
	// prompts, signaling clearly that no further steps exist.
	if req.Msg.Stage == specv1.AuthoringStage_AUTHORING_STAGE_APPROVED {
		return connect.NewResponse(&specv1.GetPromptsResponse{}), nil
	}
	prompts := promptsToProto(stage)
	if len(prompts) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("no prompts defined for stage %q", stage))
	}
	return connect.NewResponse(&specv1.GetPromptsResponse{
		Prompts: prompts,
	}), nil
}

// acceptLinkedDecisions queries for Decision nodes linked to the spec via
// DECIDED_IN edges and transitions them from proposed to accepted. Returns an
// error if any decision acceptance fails. graphBackend and decisionBackend are
// obtained from the scoped store (ScopedBackend embeds both interfaces).
func acceptLinkedDecisions(ctx context.Context, logger *slog.Logger, graphBackend storage.GraphBackend, decisionBackend storage.DecisionBackend, slug string) error {
	edges, err := graphBackend.ListEdges(ctx, slug, storage.EdgeTypeDecidedIn)
	if err != nil {
		return fmt.Errorf("list DECIDED_IN edges for %q: %w", slug, err)
	}
	acceptedStatus := storage.DecisionStatusAccepted
	for _, edge := range edges {
		// ADR-003 mandates spec→decision direction for DECIDED_IN edges.
		// Use ToID as the decision slug; FromID should be the spec.
		decisionSlug := edge.ToID
		if decisionSlug == slug {
			logger.WarnContext(ctx, "DECIDED_IN edge has unexpected direction (ToID is spec, not decision)",
				"slug", slug, "fromID", edge.FromID, "toID", edge.ToID)
			decisionSlug = edge.FromID
		}
		if decisionSlug == "" || decisionSlug == slug {
			return fmt.Errorf("DECIDED_IN edge for %q has no valid decision slug (from=%q, to=%q)", slug, edge.FromID, edge.ToID)
		}
		dec, err := decisionBackend.GetDecision(ctx, decisionSlug)
		if err != nil {
			return fmt.Errorf("get decision %q: %w", decisionSlug, err)
		}
		if dec.Status != storage.DecisionStatusProposed {
			continue
		}
		if _, err := decisionBackend.UpdateDecision(ctx, decisionSlug, 0, nil, &acceptedStatus, nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil); err != nil {
			return fmt.Errorf("accept decision %q: %w", decisionSlug, err)
		}
	}
	return nil
}

// RecordConversation stores authoring conversation exchanges for a spec stage.
func (h *AuthoringHandler) RecordConversation(
	ctx context.Context,
	req *connect.Request[specv1.RecordConversationRequest],
) (*connect.Response[specv1.RecordConversationResponse], error) {
	slug := req.Msg.Slug
	if slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}
	stage := req.Msg.Stage
	if stage == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("stage is required"))
	}
	if len(req.Msg.Exchanges) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("at least one exchange is required"))
	}
	if len(req.Msg.Exchanges) > maxElements {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("exchanges exceed maximum of %d", maxElements))
	}

	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	exchanges, exErr := conversationExchangesFromProto(req.Msg.Exchanges)
	if exErr != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, exErr)
	}
	entry := storage.ConversationLogEntry{
		Stage:         storage.SpecStage(stage),
		IsAmend:       req.Msg.IsAmend,
		Exchanges:     exchanges,
		ExchangeCount: int32(len(req.Msg.Exchanges)), //nolint:gosec // bounded by maxElements (100) validation above
	}

	result, recErr := store.RecordConversation(ctx, slug, entry)
	if recErr != nil {
		return nil, h.stageError(recErr)
	}

	return connect.NewResponse(&specv1.RecordConversationResponse{
		ConversationLog: conversationLogToProto(result),
	}), nil
}

// ListConversations returns conversation logs for a spec in narrative order.
func (h *AuthoringHandler) ListConversations(
	ctx context.Context,
	req *connect.Request[specv1.ListConversationsRequest],
) (*connect.Response[specv1.ListConversationsResponse], error) {
	slug := req.Msg.Slug
	if slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}

	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	entries, listErr := store.ListConversations(ctx, slug, req.Msg.Stage)
	if listErr != nil {
		return nil, h.stageError(listErr)
	}

	logs := make([]*specv1.ConversationLog, len(entries))
	for i, e := range entries {
		logs[i] = conversationLogToProto(e)
	}

	return connect.NewResponse(&specv1.ListConversationsResponse{
		ConversationLogs: logs,
	}), nil
}

// RegisterAuthoringService registers the AuthoringService on the given mux.
func RegisterAuthoringService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption) {
	if scoper == nil {
		panic("RegisterAuthoringService: scoper must not be nil")
	}
	handler := &AuthoringHandler{scoper: scoper, logger: slog.Default()}
	path, h := specgraphv1connect.NewAuthoringServiceHandler(handler, opts...)
	mux.Handle(path, h)
}

// --- Proto → Domain mappers ---

// scopeSniffToStorageMap maps proto ScopeSniff enum values to their storage string representation.
// SCOPE_SNIFF_UNSPECIFIED maps to an empty string (field not provided by caller).
var scopeSniffToStorageMap = map[specv1.ScopeSniff]string{
	specv1.ScopeSniff_SCOPE_SNIFF_UNSPECIFIED: "",
	specv1.ScopeSniff_SCOPE_SNIFF_TINY:        "tiny",
	specv1.ScopeSniff_SCOPE_SNIFF_SMALL:       "small",
	specv1.ScopeSniff_SCOPE_SNIFF_MEDIUM:      "medium",
	specv1.ScopeSniff_SCOPE_SNIFF_LARGE:       "large",
	specv1.ScopeSniff_SCOPE_SNIFF_EPIC:        "epic",
}

// scopeSniffToStorage converts a proto ScopeSniff enum to its storage string.
// It returns an error for unrecognized enum values (not in the known set).
func scopeSniffToStorage(s specv1.ScopeSniff) (string, error) {
	v, ok := scopeSniffToStorageMap[s]
	if !ok {
		return "", fmt.Errorf("unrecognized ScopeSniff value: %v", s)
	}
	return v, nil
}

func sparkOutputToDomain(p *specv1.SparkOutput) (*storage.SparkOutput, error) {
	scope, err := scopeSniffToStorage(p.GetScopeSniff())
	if err != nil {
		return nil, err
	}
	return &storage.SparkOutput{
		Seed:       p.GetSeed(),
		Signal:     p.GetSignal(),
		Questions:  p.GetQuestions(),
		ScopeSniff: scope,
		KillTest:   p.GetKillTest(),
	}, nil
}

func shapeOutputToDomain(p *specv1.ShapeOutput) (*storage.ShapeOutput, error) {
	approaches := make([]storage.Approach, len(p.GetApproaches()))
	for i, a := range p.GetApproaches() {
		if len(a.GetName()) > maxFieldLen {
			return nil, fmt.Errorf("approaches[%d]: name exceeds maximum length of %d characters", i, maxFieldLen)
		}
		if len(a.GetDescription()) > maxFieldLen {
			return nil, fmt.Errorf("approaches[%d]: description exceeds maximum length of %d characters", i, maxFieldLen)
		}
		if len(a.GetTradeoffs()) > maxFieldLen {
			return nil, fmt.Errorf("approaches[%d]: tradeoffs exceeds maximum length of %d characters", i, maxFieldLen)
		}
		approaches[i] = storage.Approach{
			Name:        a.GetName(),
			Description: a.GetDescription(),
			Tradeoffs:   a.GetTradeoffs(),
		}
	}
	if len(p.GetChosenApproach()) > maxFieldLen {
		return nil, fmt.Errorf("chosen_approach exceeds maximum length of %d characters", maxFieldLen)
	}
	for i, r := range p.GetRisks() {
		if len(r) > maxFieldLen {
			return nil, fmt.Errorf("risks[%d]: exceeds maximum length of %d characters", i, maxFieldLen)
		}
	}
	for i, s := range p.GetSuccessMust() {
		if len(s) > maxFieldLen {
			return nil, fmt.Errorf("success_must[%d]: exceeds maximum length of %d characters", i, maxFieldLen)
		}
	}
	for i, s := range p.GetSuccessShould() {
		if len(s) > maxFieldLen {
			return nil, fmt.Errorf("success_should[%d]: exceeds maximum length of %d characters", i, maxFieldLen)
		}
	}
	for i, s := range p.GetSuccessWont() {
		if len(s) > maxFieldLen {
			return nil, fmt.Errorf("success_wont[%d]: exceeds maximum length of %d characters", i, maxFieldLen)
		}
	}
	decisions := make([]storage.DecisionInput, len(p.GetDecisions()))
	for i, d := range p.GetDecisions() {
		if err := validateSlug(d.GetSlug()); err != nil {
			return nil, fmt.Errorf("decision[%d]: %w", i, err)
		}
		if len(d.GetTitle()) > maxFieldLen {
			return nil, fmt.Errorf("decision[%d]: title exceeds maximum length of %d characters", i, maxFieldLen)
		}
		if len(d.GetDecision()) > maxFieldLen {
			return nil, fmt.Errorf("decision[%d]: body exceeds maximum length of %d characters", i, maxFieldLen)
		}
		if len(d.GetRationale()) > maxFieldLen {
			return nil, fmt.Errorf("decision[%d]: rationale exceeds maximum length of %d characters", i, maxFieldLen)
		}
		decisions[i] = storage.DecisionInput{
			Slug:      d.GetSlug(),
			Title:     d.GetTitle(),
			Body:      d.GetDecision(),
			Rationale: d.GetRationale(),
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
		Decisions:      decisions,
	}, nil
}

// exchangesFromProto converts proto ConversationExchange messages to domain
// value-struct exchanges. Unknown role strings pass through; role validation
// happens in authoring.ValidateExchanges before persist.
func exchangesFromProto(ps []*specv1.ConversationExchange) []storage.ConversationExchange {
	out := make([]storage.ConversationExchange, len(ps))
	for i, p := range ps {
		out[i] = storage.ConversationExchange{
			Role:          storage.ConversationRole(p.GetRole()),
			Content:       p.GetContent(),
			Stage:         p.GetStage(),
			Sequence:      p.GetSequence(),
			DecisionPoint: p.GetDecisionPoint(),
		}
	}
	return out
}

// buildConversationEntry constructs a ConversationLogEntry for RecordConversation.
// posture is persisted alongside exchanges for future drift detection (design §Posture).
func buildConversationEntry(stage storage.SpecStage, _ specv1.Posture, exchanges []storage.ConversationExchange, isAmend bool) storage.ConversationLogEntry {
	return storage.ConversationLogEntry{
		Stage:         stage,
		Exchanges:     exchanges,
		ExchangeCount: int32(len(exchanges)),
		IsAmend:       isAmend,
		// Posture recording: when ConversationLogEntry gains a Posture field
		// (Task 10), wire it here. For now, captured as a metadata label if
		// backend supports it.
	}
}

func specifyOutputToDomain(p *specv1.SpecifyOutput) *storage.SpecifyOutput {
	interfaces := make([]storage.InterfaceSection, len(p.GetInterfaces()))
	for i, iface := range p.GetInterfaces() {
		interfaces[i] = storage.InterfaceSection{
			Name: iface.GetName(),
			Body: iface.GetBody(),
		}
	}
	criteria := make([]storage.VerifyCriterion, len(p.GetVerifyCriteria()))
	for i, vc := range p.GetVerifyCriteria() {
		criteria[i] = storage.VerifyCriterion{
			Category:    vc.GetCategory(),
			Description: vc.GetDescription(),
		}
	}
	touches := make([]storage.FileTouch, len(p.GetTouches()))
	for i, ft := range p.GetTouches() {
		touches[i] = storage.FileTouch{
			Path:       ft.GetPath(),
			Purpose:    ft.GetPurpose(),
			ChangeType: ft.GetChangeType(),
		}
	}
	return &storage.SpecifyOutput{
		Interfaces:     interfaces,
		VerifyCriteria: criteria,
		Invariants:     p.GetInvariants(),
		Touches:        touches,
	}
}

var decomposeStrategyMap = map[specv1.DecompositionStrategy]storage.DecompositionStrategy{
	specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE: storage.StrategyVerticalSlice,
	specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_LAYER_CAKE:     storage.StrategyLayerCake,
	specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_SINGLE_UNIT:    storage.StrategySingleUnit,
	specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD:   storage.StrategySteelThread,
}

// validateSteelThread checks steel-thread topology constraints:
// 1. slices[0] (the thread) must have no dependsOn.
// 2. Every other slice must transitively reach slices[0].id.
func validateSteelThread(slices []*specv1.DecompositionSlice) error {
	if len(slices) == 0 {
		return errors.New("steel thread strategy requires at least one slice")
	}
	threadID := slices[0].GetId()
	if len(slices[0].GetDependsOn()) > 0 {
		return fmt.Errorf("steel thread strategy requires slices[0] to have no dependencies (it is the thread)")
	}

	// Build adjacency: child -> parents (dependsOn). Reject duplicate IDs.
	deps := make(map[string][]string, len(slices))
	for _, s := range slices {
		id := s.GetId()
		if _, exists := deps[id]; exists {
			return fmt.Errorf("duplicate slice id %q", id)
		}
		deps[id] = s.GetDependsOn()
	}

	// For each non-root slice, walk dependsOn transitively to check reachability.
	for _, s := range slices[1:] {
		if !reachesRoot(s.GetId(), threadID, deps) {
			return fmt.Errorf("slice %q does not transitively depend on thread slice %q", s.GetId(), threadID)
		}
	}
	return nil
}

// reachesRoot walks the dependsOn graph from start, returning true if threadID is reachable.
func reachesRoot(start, threadID string, deps map[string][]string) bool {
	visited := map[string]bool{}
	queue := []string{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == threadID {
			return true
		}
		if visited[cur] {
			continue
		}
		visited[cur] = true
		queue = append(queue, deps[cur]...)
	}
	return false
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

// stageToProto delegates to the canonical mapping in authoring_convert.go.
func stageToProto(stage storage.SpecStage) specv1.AuthoringStage {
	return authoringStageToProto(stage)
}

var protoToStage = map[specv1.AuthoringStage]storage.SpecStage{
	specv1.AuthoringStage_AUTHORING_STAGE_SPARK:     storage.SpecStageSpark,
	specv1.AuthoringStage_AUTHORING_STAGE_SHAPE:     storage.SpecStageShape,
	specv1.AuthoringStage_AUTHORING_STAGE_SPECIFY:   storage.SpecStageSpecify,
	specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE: storage.SpecStageDecompose,
	specv1.AuthoringStage_AUTHORING_STAGE_APPROVED:  storage.SpecStageApproved,
}

// runInTxOrSequential runs the given operations within a transaction if the
// backend implements TransactionalBackend, otherwise runs them sequentially.
func runInTxOrSequential(ctx context.Context, backend storage.TransactionalBackend, ops ...func(context.Context) error) error {
	if err := backend.RunInTransaction(ctx, func(txCtx context.Context) error {
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

func (h *AuthoringHandler) stageError(err error) error {
	// If the error is already a connect.Error (e.g. from a safety validation
	// op inside runInTxOrSequential), unwrap and return it as-is so the
	// original code is preserved rather than re-wrapped as CodeInternal.
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		return connErr
	}
	if errors.Is(err, storage.ErrConcurrentModification) {
		return connect.NewError(connect.CodeAborted, errors.New("concurrent modification"))
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
	if errors.Is(err, storage.ErrSpecSuperseded) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("spec has been superseded"))
	}
	h.logger.Error("stageError: internal error", slog.Any("error", err))
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

// categoryToStorageMap maps authoring.SafetyCategory to the storage string representation.
var categoryToStorageMap = map[authoring.SafetyCategory]storage.SafetyCategory{
	authoring.SafetyCategorySecurity: "security",
	authoring.SafetyCategoryDataLoss: "data_loss",
}

// persistSafetyFlags runs the safety net, stores any resulting flags, and
// returns the domain-level results for inclusion in the RPC response.
func persistSafetyFlags(ctx context.Context, store storage.AuthoringBackend, slug string, input *authoring.SafetyInput) ([]authoring.SafetyFlagResult, error) {
	if err := input.Validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	flags := authoring.RunSafetyNet(input)
	if len(flags) > 0 {
		storageFlags, err := safetyFlagsToStorage(flags)
		if err != nil {
			return nil, fmt.Errorf("convert safety flags: %w", err)
		}
		if err := store.StoreSafetyFlags(ctx, slug, storageFlags); err != nil {
			return nil, fmt.Errorf("store safety flags: %w", err)
		}
	}
	return flags, nil
}

// safetyFlagsToStorage converts domain-level safety results to storage types.
// It returns an error if any flag contains an unrecognized category or severity.
func safetyFlagsToStorage(flags []authoring.SafetyFlagResult) ([]storage.SafetyFlag, error) {
	out := make([]storage.SafetyFlag, len(flags))
	for i, f := range flags {
		cat, ok := categoryToStorageMap[f.Category]
		if !ok {
			return nil, fmt.Errorf("unrecognized SafetyCategory value: %v", f.Category)
		}
		sev, err := authoring.ToStorageSeverity(f.Severity)
		if err != nil {
			return nil, fmt.Errorf("convert severity: %w", err)
		}
		out[i] = storage.SafetyFlag{
			Category:    cat,
			Severity:    sev,
			Description: f.Description,
		}
	}
	return out, nil
}
