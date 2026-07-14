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
	authload "github.com/specgraph/specgraph/internal/authoring/load"
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
			"Actions: spark, shape, specify, decompose, approve, amend, supersede. " +
			"For spark/shape/specify/decompose pass the stage result in `output` as " +
			"friendly snake_case YAML (e.g. spark `scope_sniff: small`; shape " +
			"`scope_in`/`chosen_approach`; decompose `strategy: vertical_slice`) — " +
			"no protojson/enum names. shape/specify/decompose also require `exchanges` " +
			"(a JSON array; see param doc). " +
			"amend returns an IN-FLIGHT spec (approved/in_progress/review) to an earlier " +
			"authoring stage: it lands the spec ONE stage before `re_entry_stage` so that " +
			"stage's authoring command is the valid next transition — requires `reason` and " +
			"`re_entry_stage` (spark|shape|specify|decompose). supersede replaces a DONE spec " +
			"with another via `new_slug` (optional `reason`).",
		Profile: ProfileAuthoring,
		Schema: objectSchema(
			props{
				"action": stringProp("Operation to perform",
					"spark", "shape", "specify", "decompose", "approve", "amend", "supersede"),
				"slug": stringProp("Spec slug (required for all actions)"),
				"output": stringProp(
					"Stage output as friendly snake_case YAML (required for spark/shape/specify/decompose). " +
						"Per stage — spark: seed, signal, questions[], scope_sniff (tiny|small|medium|large|epic), kill_test. " +
						"shape: scope_in[], scope_out[], approaches[] (name/description/tradeoffs[]), chosen_approach, " +
						"risks[], success_must[]/success_should[]/success_wont[], decisions[] (slug/title/decision/rationale). " +
						"specify: interfaces[] (name/body), verify_criteria[] (category/description), invariants[], " +
						"touches[] (path/purpose/change_type). decompose: strategy " +
						"(vertical_slice|layer_cake|single_unit|steel_thread), slices[] (id/intent/verify[]/touches[]/depends_on[]). " +
						"Use snake_case verbatim — camelCase (scopeIn, chosenApproach) is rejected.",
				),
				"exchanges": stringProp(
					"Conversation log as a JSON array of ConversationExchange objects — " +
						"required for shape/specify/decompose, optional for spark, not needed for approve. " +
						"Each object has: role (\"probe\" = agent asks, \"response\" = user answers), " +
						"content (the text), stage (the authoring stage, e.g. \"shape\"), and " +
						"sequence (strictly increasing integer >= 1; same number pairs a probe with its response). " +
						"Minimal shape example: " +
						`[{"role":"probe","content":"What is out of scope?","stage":"shape","sequence":1},` +
						`{"role":"response","content":"Anything touching billing.","stage":"shape","sequence":2}]`,
				),
				"posture": stringProp("AI collaboration posture: drive, partner, support"),
				"reason":  stringProp("Reason for amend (required) or supersede (optional)"),
				"re_entry_stage": stringProp(
					"Re-entry authoring stage for amend (required) — one of: spark, shape, specify, decompose. " +
						"Name the stage you want to RE-DO. The spec is landed ONE stage before it, so that " +
						"stage's own authoring command is a valid forward transition. " +
						"For example re_entry_stage=shape lands the spec at spark, so you then run " +
						"author action=shape (spark→shape) to re-author. " +
						"approved/in_progress/review/done are rejected — use those only via the normal funnel, " +
						"and use supersede (not amend) to replace a done spec.",
				),
				"new_slug": stringProp(
					"Slug of the replacement spec for supersede (required). The old (done) spec is marked " +
						"superseded and a SUPERSEDES edge is created from new_slug to the old slug. " +
						"For example superseding old-auth with new-auth: slug=old-auth, new_slug=new-auth.",
				),
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
		return errResult("output is required for spark (friendly YAML SparkOutput)"), nil
	}
	out, err := authload.SparkFromYAML([]byte(raw))
	if err != nil {
		// Sanitized: no raw parser internals leaked (T-06-03).
		return errResult("invalid spark output (expected friendly snake_case YAML)"), nil //nolint:nilerr // raw parser error intentionally not surfaced (T-06-03)
	}
	posture, posErr := validateOptionalPosture(params)
	if posErr != nil {
		return posErr, nil
	}
	resp, err := t.client.Authoring.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
		Slug:    slug,
		Output:  out,
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
		return errResult("output is required for shape (friendly YAML ShapeOutput)"), nil
	}
	out, err := authload.ShapeFromYAML([]byte(raw))
	if err != nil {
		// Sanitized: no raw parser internals leaked (T-06-03).
		return errResult("invalid shape output (expected friendly snake_case YAML)"), nil //nolint:nilerr // raw parser error intentionally not surfaced (T-06-03)
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
		Output:                out,
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
		return errResult("output is required for specify (friendly YAML SpecifyOutput)"), nil
	}
	out, err := authload.SpecifyFromYAML([]byte(raw))
	if err != nil {
		// Sanitized: no raw parser internals leaked (T-06-03).
		return errResult("invalid specify output (expected friendly snake_case YAML)"), nil //nolint:nilerr // raw parser error intentionally not surfaced (T-06-03)
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
		Output:                out,
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
		return errResult("output is required for decompose (friendly YAML DecomposeOutput)"), nil
	}
	out, err := authload.DecomposeFromYAML([]byte(raw))
	if err != nil {
		// Sanitized: no raw parser internals leaked (T-06-03).
		return errResult("invalid decompose output (expected friendly snake_case YAML)"), nil //nolint:nilerr // raw parser error intentionally not surfaced (T-06-03)
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
		Output:                out,
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
	reEntryStage := stringParam(params, "re_entry_stage")
	if reEntryStage == "" {
		return errResult("re_entry_stage is required for amend (spark, shape, specify, decompose)"), nil
	}
	reason := stringParam(params, "reason")
	if reason == "" {
		return errResult("reason is required for amend"), nil
	}
	// Pass re_entry_stage straight through: the TransitionAmend handler is the
	// single source of truth for the re-entry allowlist (spark|shape|specify|
	// decompose) and rejects approved/in_progress/review/done. Re-validating here
	// would risk drifting from that gate.
	resp, err := t.client.Lifecycle.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:         slug,
		Reason:       reason,
		ReEntryStage: reEntryStage,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	res := jsonResult(resp.Msg)
	if res.IsError {
		return res, nil
	}
	// D-05: the spec lands one stage BEFORE re_entry_stage, so the author action
	// for re_entry_stage is the valid next transition. Echo it so the agent does
	// not reproduce the #899 no-op.
	hint := fmt.Sprintf("Next step: run author action=%s for spec %q to re-author the %s stage.",
		reEntryStage, slug, reEntryStage)
	res.Content = append(res.Content, Content{Type: "text", Text: hint})
	return res, nil
}

func (t *authorTool) handleSupersede(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for supersede"), nil
	}
	newSlug := stringParam(params, "new_slug")
	if newSlug == "" {
		return errResult("new_slug is required for supersede"), nil
	}
	resp, err := t.client.Lifecycle.TransitionSupersede(ctx, connect.NewRequest(&specv1.TransitionSupersedeRequest{
		Slug:    slug,
		NewSlug: newSlug,
		Reason:  stringParam(params, "reason"),
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
