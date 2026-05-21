// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/authoring"
	"google.golang.org/protobuf/encoding/protojson"
)

// RegisterAuthoringTools registers authoring funnel, conversation, and analytical pass tools.
func RegisterAuthoringTools(r *Registry, c *Client) {
	at := &authorTool{client: c}
	r.AddTool(at.def())

	ct := &conversationTool{client: c}
	r.AddTool(ct.def())

	apt := &analyticalPassTool{client: c}
	r.AddTool(apt.def())

	r.AddTool(ToolDef{
		Name: "author_start_stage",
		Description: "Returns composed stage guidance (persona + orchestration + stage-specific content + current state). " +
			"Use when the client does not expose MCP prompts to users, or for mid-conversation re-entry into a stage.",
		Profile: ProfileAuthoring,
		Schema: objectSchema(props{
			"stage":   stringProp("Stage", "spark", "shape", "specify", "decompose", "approve"),
			"slug":    stringProp("Spec slug (required for shape/specify/decompose/approve; optional for spark)"),
			"posture": stringProp("Posture", "drive", "partner", "support"),
		}, "stage"),
		Handler: authoringStartStageHandler(c),
	})
}

// authoringStartStageHandler returns a ToolHandler that composes and returns stage guidance.
func authoringStartStageHandler(c *Client) ToolHandler {
	composer := authoring.NewComposer(&composerBackend{client: c})
	return func(ctx context.Context, params map[string]any) (*ToolResult, error) {
		stage := stringParam(params, "stage")
		if stage == "" {
			return errResult("stage is required (spark|shape|specify|decompose|approve)"), nil
		}
		slug := stringParam(params, "slug")
		if stage != "spark" && slug == "" {
			return errResult(fmt.Sprintf("slug is required for stage %s", stage)), nil
		}
		result, err := composer.ComposeStagePrompt(ctx, authoring.ComposeInput{
			Stage:   authoring.Stage(stage),
			Slug:    slug,
			Posture: stringParam(params, "posture"),
		})
		if err != nil {
			return errResult(fmt.Sprintf("compose: %v", err)), nil
		}
		return &ToolResult{Content: []Content{{Type: "text", Text: result.Body}}}, nil
	}
}

// parseOptionalExchanges parses the "exchanges" param as a JSON array of
// ConversationExchange objects. Returns nil exchanges and nil errResult if the
// param is absent (caller decides whether that is acceptable for the stage).
// Returns an errResult on malformed JSON.
func parseOptionalExchanges(params map[string]any) ([]*specv1.ConversationExchange, *ToolResult) {
	raw := stringParam(params, "exchanges")
	if raw == "" {
		return nil, nil
	}
	// Parse exchanges array in isolation to prevent JSON injection.
	var wrapper specv1.RecordConversationRequest
	if err := protojson.Unmarshal([]byte(`{"exchanges":`+raw+`}`), &wrapper); err != nil {
		return nil, errResult(fmt.Sprintf("invalid exchanges JSON: %v", err))
	}
	return wrapper.Exchanges, nil
}

// validateOptionalPosture checks that a non-empty posture string maps to a known enum.
// Returns the parsed posture and nil, or zero and an errResult if invalid.
func validateOptionalPosture(params map[string]any) (specv1.Posture, *ToolResult) {
	s := stringParam(params, "posture")
	if s == "" {
		return specv1.Posture_POSTURE_UNSPECIFIED, nil
	}
	p := postureFromString(s)
	if p == specv1.Posture_POSTURE_UNSPECIFIED {
		return 0, errResult("invalid posture (valid: drive, partner, support)")
	}
	return p, nil
}

// postureFromString converts a friendly string (e.g. "drive") to a Posture enum.
func postureFromString(s string) specv1.Posture {
	key := "POSTURE_" + strings.ToUpper(s)
	if v, ok := specv1.Posture_value[key]; ok {
		return specv1.Posture(v)
	}
	return specv1.Posture_POSTURE_UNSPECIFIED
}

// authoringStageFromString converts a friendly string (e.g. "spark") to an AuthoringStage enum.
func authoringStageFromString(s string) specv1.AuthoringStage {
	key := "AUTHORING_STAGE_" + strings.ToUpper(s)
	if v, ok := specv1.AuthoringStage_value[key]; ok {
		return specv1.AuthoringStage(v)
	}
	return specv1.AuthoringStage_AUTHORING_STAGE_UNSPECIFIED
}

// ---------------------------------------------------------------------------
// authorTool — authoring funnel: spark, shape, specify, decompose, approve, amend, supersede
// ---------------------------------------------------------------------------

type authorTool struct {
	client *Client
}

func (t *authorTool) def() ToolDef {
	return ToolDef{
		Name: "author",
		Description: "Drive the SpecGraph authoring funnel for a spec. " +
			"Actions: spark, shape, specify, decompose, approve, amend, supersede.",
		Profile: ProfileAuthoring,
		Schema: objectSchema(
			props{
				"action": stringProp("Operation to perform",
					"spark", "shape", "specify", "decompose", "approve", "amend", "supersede"),
				"slug":         stringProp("Spec slug (required for all actions)"),
				"output":       stringProp("Stage output as a JSON string (required for spark/shape/specify/decompose)"),
				"exchanges":    stringProp("JSON array of ConversationExchange objects (required for shape/specify/decompose; optional for spark)"),
				"posture":      stringProp("AI collaboration posture: drive, partner, support"),
				"reason":       stringProp("Reason for amend or supersede"),
				"target_stage": stringProp("Target stage for amend: spark, shape, specify, decompose"),
				"superseded_by": stringProp("Slug of the replacement spec (required for supersede)"),
			},
			"action", "slug",
		),
		Handler: t.handle,
	}
}

func (t *authorTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "spark":
		return t.handleSpark(ctx, params)
	case "shape":
		return t.handleShape(ctx, params)
	case "specify":
		return t.handleSpecify(ctx, params)
	case "decompose":
		return t.handleDecompose(ctx, params)
	case "approve":
		return t.handleApprove(ctx, params)
	case "amend":
		return t.handleAmend(ctx, params)
	case "supersede":
		return t.handleSupersede(ctx, params)
	default:
		return errResult(fmt.Sprintf("unknown action %q — valid: spark, shape, specify, decompose, approve, amend, supersede", action)), nil
	}
}

func (t *authorTool) handleSpark(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for spark"), nil
	}
	raw := stringParam(params, "output")
	if raw == "" {
		return errResult("output is required for spark (JSON-encoded SparkOutput)"), nil
	}
	var out specv1.SparkOutput
	if err := protojson.Unmarshal([]byte(raw), &out); err != nil {
		return errResult(fmt.Sprintf("invalid spark output JSON: %v", err)), nil
	}
	posture, posErr := validateOptionalPosture(params)
	if posErr != nil {
		return posErr, nil
	}
	resp, err := t.client.Authoring.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
		Slug:    slug,
		Output:  &out,
		Posture: posture,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *authorTool) handleShape(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for shape"), nil
	}
	raw := stringParam(params, "output")
	if raw == "" {
		return errResult("output is required for shape (JSON-encoded ShapeOutput)"), nil
	}
	var out specv1.ShapeOutput
	if err := protojson.Unmarshal([]byte(raw), &out); err != nil {
		return errResult(fmt.Sprintf("invalid shape output JSON: %v", err)), nil
	}
	posture, posErr := validateOptionalPosture(params)
	if posErr != nil {
		return posErr, nil
	}
	exchanges, exErr := parseOptionalExchanges(params)
	if exErr != nil {
		return exErr, nil
	}
	resp, err := t.client.Authoring.Shape(ctx, connect.NewRequest(&specv1.ShapeRequest{
		Slug:                  slug,
		Output:                &out,
		Posture:               posture,
		ConversationExchanges: exchanges,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *authorTool) handleSpecify(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for specify"), nil
	}
	raw := stringParam(params, "output")
	if raw == "" {
		return errResult("output is required for specify (JSON-encoded SpecifyOutput)"), nil
	}
	var out specv1.SpecifyOutput
	if err := protojson.Unmarshal([]byte(raw), &out); err != nil {
		return errResult(fmt.Sprintf("invalid specify output JSON: %v", err)), nil
	}
	posture, posErr := validateOptionalPosture(params)
	if posErr != nil {
		return posErr, nil
	}
	exchanges, exErr := parseOptionalExchanges(params)
	if exErr != nil {
		return exErr, nil
	}
	resp, err := t.client.Authoring.Specify(ctx, connect.NewRequest(&specv1.SpecifyRequest{
		Slug:                  slug,
		Output:                &out,
		Posture:               posture,
		ConversationExchanges: exchanges,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *authorTool) handleDecompose(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for decompose"), nil
	}
	raw := stringParam(params, "output")
	if raw == "" {
		return errResult("output is required for decompose (JSON-encoded DecomposeOutput)"), nil
	}
	var out specv1.DecomposeOutput
	if err := protojson.Unmarshal([]byte(raw), &out); err != nil {
		return errResult(fmt.Sprintf("invalid decompose output JSON: %v", err)), nil
	}
	posture, posErr := validateOptionalPosture(params)
	if posErr != nil {
		return posErr, nil
	}
	exchanges, exErr := parseOptionalExchanges(params)
	if exErr != nil {
		return exErr, nil
	}
	resp, err := t.client.Authoring.Decompose(ctx, connect.NewRequest(&specv1.DecomposeRequest{
		Slug:                  slug,
		Output:                &out,
		Posture:               posture,
		ConversationExchanges: exchanges,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *authorTool) handleApprove(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for approve"), nil
	}
	resp, err := t.client.Authoring.Approve(ctx, connect.NewRequest(&specv1.ApproveRequest{
		Slug: slug,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *authorTool) handleAmend(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for amend"), nil
	}
	targetStageStr := stringParam(params, "target_stage")
	targetStage := specv1.AuthoringStage_AUTHORING_STAGE_UNSPECIFIED
	if targetStageStr != "" {
		targetStage = authoringStageFromString(targetStageStr)
		if targetStage == specv1.AuthoringStage_AUTHORING_STAGE_UNSPECIFIED {
			return errResult("invalid target_stage (valid: spark, shape, specify, decompose, approved)"), nil
		}
	}
	resp, err := t.client.Authoring.Amend(ctx, connect.NewRequest(&specv1.AmendRequest{
		Slug:        slug,
		Reason:      stringParam(params, "reason"),
		TargetStage: targetStage,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *authorTool) handleSupersede(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for supersede"), nil
	}
	supersededBy := stringParam(params, "superseded_by")
	if supersededBy == "" {
		return errResult("superseded_by is required for supersede"), nil
	}
	resp, err := t.client.Authoring.Supersede(ctx, connect.NewRequest(&specv1.SupersedeRequest{
		Slug:         slug,
		SupersededBy: supersededBy,
		Reason:       stringParam(params, "reason"),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// ---------------------------------------------------------------------------
// conversationTool — record and list authoring conversation logs
// ---------------------------------------------------------------------------

type conversationTool struct {
	client *Client
}

func (t *conversationTool) def() ToolDef {
	return ToolDef{
		Name: "conversation",
		Description: "Record and list authoring conversation logs for a spec. " +
			"Actions: record, list.",
		Profile: ProfileAuthoring,
		Schema: objectSchema(
			props{
				"action":    stringProp("Operation to perform", "record", "list"),
				"slug":      stringProp("Spec slug (required)"),
				"stage":     stringProp("Authoring stage (required for record; optional filter for list)"),
				"exchanges": stringProp("JSON array of ConversationExchange objects (required for record)"),
				"is_amend":  boolProp("Whether this conversation is part of an amendment"),
			},
			"action", "slug",
		),
		Handler: t.handle,
	}
}

func (t *conversationTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "record":
		return t.handleRecord(ctx, params)
	case "list":
		return t.handleList(ctx, params)
	default:
		return errResult(fmt.Sprintf("unknown action %q — valid: record, list", action)), nil
	}
}

func (t *conversationTool) handleRecord(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for record"), nil
	}
	stage := stringParam(params, "stage")
	if stage == "" {
		return errResult("stage is required for record"), nil
	}
	exchangesRaw := stringParam(params, "exchanges")
	if exchangesRaw == "" {
		return errResult("exchanges is required for record (JSON array of ConversationExchange)"), nil
	}
	// Parse exchanges array in isolation to prevent JSON injection.
	var exchanges specv1.RecordConversationRequest
	if err := protojson.Unmarshal([]byte(`{"exchanges":`+exchangesRaw+`}`), &exchanges); err != nil {
		return errResult(fmt.Sprintf("invalid exchanges JSON: %v", err)), nil
	}
	req := &specv1.RecordConversationRequest{
		Slug:      slug,
		Stage:     stage,
		Exchanges: exchanges.Exchanges,
		IsAmend:   boolParam(params, "is_amend"),
	}
	resp, err := t.client.Authoring.RecordConversation(ctx, connect.NewRequest(req))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *conversationTool) handleList(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for list"), nil
	}
	resp, err := t.client.Authoring.ListConversations(ctx, connect.NewRequest(&specv1.ListConversationsRequest{
		Slug:  slug,
		Stage: stringParam(params, "stage"),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// ---------------------------------------------------------------------------
// analyticalPassTool — run analytical passes and store findings
// ---------------------------------------------------------------------------

type analyticalPassTool struct {
	client *Client
}

func (t *analyticalPassTool) def() ToolDef {
	return ToolDef{
		Name: "analytical_pass",
		Description: "Run analytical passes and store findings for a spec. " +
			"Actions: run, store.",
		Profile: ProfileAuthoring,
		Schema: objectSchema(
			props{
				"action": stringProp("Operation to perform", "run", "store"),
				"slug":   stringProp("Spec slug (required)"),
				"pass_type": stringProp(
					"Pass type: constitution-check, red-team, peripheral-vision, consistency, simplicity",
					"constitution-check", "red-team", "peripheral-vision", "consistency", "simplicity",
				),
				"findings": stringProp("JSON array of AnalyticalFindingInput objects (required for store)"),
			},
			"action", "slug",
		),
		Handler: t.handle,
	}
}

func (t *analyticalPassTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "run":
		return t.handleRun(ctx, params)
	case "store":
		return t.handleStore(ctx, params)
	default:
		return errResult(fmt.Sprintf("unknown action %q — valid: run, store", action)), nil
	}
}

func (t *analyticalPassTool) handleRun(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for run"), nil
	}
	passTypeStr := stringParam(params, "pass_type")
	if passTypeStr == "" {
		return errResult("pass_type is required for run"), nil
	}
	passType := passTypeFromString(passTypeStr)
	if passType == specv1.PassType_PASS_TYPE_UNSPECIFIED {
		return errResult("invalid pass_type for run"), nil
	}
	resp, err := t.client.AnalyticalPass.RunAnalyticalPass(ctx, connect.NewRequest(&specv1.RunAnalyticalPassRequest{
		Slug:     slug,
		PassType: passType,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *analyticalPassTool) handleStore(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for store"), nil
	}
	findingsRaw := stringParam(params, "findings")
	if findingsRaw == "" {
		return errResult("findings is required for store (JSON array of AnalyticalFindingInput)"), nil
	}
	// Parse findings array in isolation to prevent JSON injection.
	var findings specv1.StoreFindingsRequest
	if err := protojson.Unmarshal([]byte(`{"findings":`+findingsRaw+`}`), &findings); err != nil {
		return errResult(fmt.Sprintf("invalid findings JSON: %v", err)), nil
	}
	passTypeStr := stringParam(params, "pass_type")
	if passTypeStr == "" {
		return errResult("pass_type is required for store"), nil
	}
	passType := passTypeFromString(passTypeStr)
	if passType == specv1.PassType_PASS_TYPE_UNSPECIFIED {
		return errResult("invalid pass_type for store"), nil
	}
	req := &specv1.StoreFindingsRequest{
		Slug:     slug,
		PassType: passType,
		Findings: findings.Findings,
	}
	resp, err := t.client.AnalyticalPass.StoreFindings(ctx, connect.NewRequest(req))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}
