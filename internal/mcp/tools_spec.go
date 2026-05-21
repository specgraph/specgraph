// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// RegisterSpecTools registers the spec and decision tools into the registry.
func RegisterSpecTools(r *Registry, c *Client) {
	st := &specTool{client: c}
	r.AddTool(st.def())

	dt := &decisionTool{client: c}
	r.AddTool(dt.def())
}

// ---------------------------------------------------------------------------
// specTool — CRUD + history operations on Specs
// ---------------------------------------------------------------------------

type specTool struct {
	client *Client
}

func (t *specTool) def() ToolDef {
	return ToolDef{
		Name: "spec",
		Description: "Read and write SpecGraph specs. " +
			"Actions: get, list, create, update, changes, compare.",
		Profile: ProfileCore,
		Schema: objectSchema(
			props{
				"action": stringProp("Operation to perform",
					"get", "list", "create", "update", "changes", "compare"),
				"slug":               stringProp("Spec slug (required for get/update/changes/compare)"),
				"intent":             stringProp("What the spec is about (required for create)"),
				"stage":              stringProp("Authoring stage filter or update value"),
				"priority":           stringProp("Priority: p0, p1, p2, p3"),
				"complexity":         stringProp("Complexity: low, medium, high"),
				"notes":              stringProp("Free-text notes to attach to the spec"),
				"limit":              intProp("Max results to return for list (default 50)"),
				"from_version":       intProp("From-version for compare"),
				"to_version":         intProp("To-version for compare"),
				"provenance":         stringProp("Provenance type for create: authored | retroactive_from_pr | declared"),
				"provenance_detail":  stringProp(`JSON-encoded provenance detail object. For retroactive_from_pr: {"url":"...","sha":"...","title":"...","merged_at":"RFC3339"}. For declared: {"declared_by":"...","note":"..."}`),
				"spark_output":       stringProp("protojson-encoded SparkOutput (optional, non-AUTHORED creates)"),
				"shape_output":       stringProp("protojson-encoded ShapeOutput (optional, non-AUTHORED creates)"),
				"specify_output":     stringProp("protojson-encoded SpecifyOutput (optional, non-AUTHORED creates)"),
				"decompose_output":   stringProp("protojson-encoded DecomposeOutput (optional, non-AUTHORED creates)"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *specTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "get":
		return t.handleGet(ctx, params)
	case "list":
		return t.handleList(ctx, params)
	case "create":
		return t.handleCreate(ctx, params)
	case "update":
		return t.handleUpdate(ctx, params)
	case "changes":
		return t.handleChanges(ctx, params)
	case "compare":
		return t.handleCompare(ctx, params)
	default:
		return errResult(fmt.Sprintf("unknown action %q — valid: get, list, create, update, changes, compare", action)), nil
	}
}

func (t *specTool) handleGet(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for get"), nil
	}
	resp, err := t.client.Spec.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
		Slug: slug,
	}))
	if err != nil {
		result, rerr := connectErrResult(err)
		return result, rerr
	}
	return jsonResult(resp.Msg), nil
}

func (t *specTool) handleList(ctx context.Context, params map[string]any) (*ToolResult, error) {
	resp, err := t.client.Spec.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{
		Stage:    stringParam(params, "stage"),
		Priority: stringParam(params, "priority"),
		Limit:    int32Param(params, "limit"),
	}))
	if err != nil {
		result, rerr := connectErrResult(err)
		return result, rerr
	}
	return jsonResult(resp.Msg), nil
}

func (t *specTool) handleCreate(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	intent := stringParam(params, "intent")
	if slug == "" {
		return errResult("slug is required for create"), nil
	}
	if intent == "" {
		return errResult("intent is required for create"), nil
	}

	provFlag := stringParam(params, "provenance")
	var provEnum specv1.SpecProvenance
	switch strings.ToLower(provFlag) {
	case "", "authored":
		provEnum = specv1.SpecProvenance_SPEC_PROVENANCE_AUTHORED
	case "retroactive_from_pr", "retroactive-from-pr", "retroactive":
		provEnum = specv1.SpecProvenance_SPEC_PROVENANCE_RETROACTIVE_FROM_PR
	case "declared":
		provEnum = specv1.SpecProvenance_SPEC_PROVENANCE_DECLARED
	default:
		return errResult(fmt.Sprintf("invalid provenance %q (valid: authored, retroactive_from_pr, declared)", provFlag)), nil
	}

	req := &specv1.CreateSpecRequest{
		Slug:           slug,
		Intent:         intent,
		Priority:       stringParam(params, "priority"),
		Complexity:     stringParam(params, "complexity"),
		ProvenanceType: provEnum,
	}

	// Parse provenance_detail JSON envelope when provided.
	if detailJSON := stringParam(params, "provenance_detail"); detailJSON != "" {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(detailJSON), &raw); err != nil {
			return errResult(fmt.Sprintf("invalid provenance_detail JSON: %v", err)), nil
		}
		switch provEnum {
		case specv1.SpecProvenance_SPEC_PROVENANCE_RETROACTIVE_FROM_PR:
			var pd struct {
				URL      string `json:"url"`
				SHA      string `json:"sha"`
				Title    string `json:"title"`
				MergedAt string `json:"merged_at"`
			}
			if err := json.Unmarshal([]byte(detailJSON), &pd); err != nil {
				return errResult(fmt.Sprintf("invalid retroactive_from_pr provenance_detail: %v", err)), nil
			}
			prov := &specv1.RetroactiveFromPrProvenance{
				Url:   pd.URL,
				Sha:   pd.SHA,
				Title: pd.Title,
			}
			if pd.MergedAt != "" {
				t, err := time.Parse(time.RFC3339, pd.MergedAt)
				if err != nil {
					return errResult(fmt.Sprintf("invalid merged_at timestamp: %v", err)), nil
				}
				prov.MergedAt = timestamppb.New(t)
			}
			req.ProvenanceDetail = &specv1.CreateSpecRequest_RetroactiveFromPr{RetroactiveFromPr: prov}
		case specv1.SpecProvenance_SPEC_PROVENANCE_DECLARED:
			var pd struct {
				DeclaredBy string `json:"declared_by"`
				Note       string `json:"note"`
			}
			if err := json.Unmarshal([]byte(detailJSON), &pd); err != nil {
				return errResult(fmt.Sprintf("invalid declared provenance_detail: %v", err)), nil
			}
			req.ProvenanceDetail = &specv1.CreateSpecRequest_Declared{
				Declared: &specv1.DeclaredProvenance{DeclaredBy: pd.DeclaredBy, Note: pd.Note},
			}
		}
	}

	// Parse stage output params via protojson.
	if s := stringParam(params, "spark_output"); s != "" {
		msg := &specv1.SparkOutput{}
		if err := unmarshalProtoJSON(s, msg); err != nil {
			return errResult(fmt.Sprintf("invalid spark_output: %v", err)), nil
		}
		req.SparkOutput = msg
	}
	if s := stringParam(params, "shape_output"); s != "" {
		msg := &specv1.ShapeOutput{}
		if err := unmarshalProtoJSON(s, msg); err != nil {
			return errResult(fmt.Sprintf("invalid shape_output: %v", err)), nil
		}
		req.ShapeOutput = msg
	}
	if s := stringParam(params, "specify_output"); s != "" {
		msg := &specv1.SpecifyOutput{}
		if err := unmarshalProtoJSON(s, msg); err != nil {
			return errResult(fmt.Sprintf("invalid specify_output: %v", err)), nil
		}
		req.SpecifyOutput = msg
	}
	if s := stringParam(params, "decompose_output"); s != "" {
		msg := &specv1.DecomposeOutput{}
		if err := unmarshalProtoJSON(s, msg); err != nil {
			return errResult(fmt.Sprintf("invalid decompose_output: %v", err)), nil
		}
		req.DecomposeOutput = msg
	}

	resp, err := t.client.Spec.CreateSpec(ctx, connect.NewRequest(req))
	if err != nil {
		result, rerr := connectErrResult(err)
		return result, rerr
	}
	return jsonResult(resp.Msg), nil
}

// unmarshalProtoJSON decodes a protojson string into a proto.Message.
func unmarshalProtoJSON(s string, msg proto.Message) error {
	opts := protojson.UnmarshalOptions{DiscardUnknown: false}
	if err := opts.Unmarshal([]byte(s), msg); err != nil {
		return fmt.Errorf("unmarshal proto JSON: %w", err)
	}
	return nil
}

func (t *specTool) handleUpdate(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for update"), nil
	}
	resp, err := t.client.Spec.UpdateSpec(ctx, connect.NewRequest(&specv1.UpdateSpecRequest{
		Slug:       slug,
		Intent:     stringPtrParam(params, "intent"),
		Stage:      stringPtrParam(params, "stage"),
		Priority:   stringPtrParam(params, "priority"),
		Complexity: stringPtrParam(params, "complexity"),
		Notes:      stringPtrParam(params, "notes"),
	}))
	if err != nil {
		result, rerr := connectErrResult(err)
		return result, rerr
	}
	return jsonResult(resp.Msg), nil
}

func (t *specTool) handleChanges(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for changes"), nil
	}
	resp, err := t.client.Spec.ListChanges(ctx, connect.NewRequest(&specv1.ListChangesRequest{
		Slug:  slug,
		Limit: int32Param(params, "limit"),
	}))
	if err != nil {
		result, rerr := connectErrResult(err)
		return result, rerr
	}
	return jsonResult(resp.Msg), nil
}

func (t *specTool) handleCompare(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for compare"), nil
	}
	fromVersion := int32Param(params, "from_version")
	toVersion := int32Param(params, "to_version")
	if fromVersion == 0 || toVersion == 0 {
		return errResult("from_version and to_version are required for compare"), nil
	}
	resp, err := t.client.Spec.CompareVersions(ctx, connect.NewRequest(&specv1.CompareVersionsRequest{
		Slug:        slug,
		FromVersion: fromVersion,
		ToVersion:   toVersion,
	}))
	if err != nil {
		result, rerr := connectErrResult(err)
		return result, rerr
	}
	return jsonResult(resp.Msg), nil
}

// ---------------------------------------------------------------------------
// decisionTool — CRUD operations on Decisions (ADRs)
// ---------------------------------------------------------------------------

type decisionTool struct {
	client *Client
}

func (t *decisionTool) def() ToolDef {
	return ToolDef{
		Name: "decision",
		Description: "Read and write SpecGraph decisions (ADRs). " +
			"Actions: get, list, create, update.",
		Profile: ProfileCore,
		Schema: objectSchema(
			props{
				"action":    stringProp("Operation to perform", "get", "list", "create", "update"),
				"slug":      stringProp("Decision slug (required for get/update)"),
				"title":     stringProp("Short title for the decision (required for create)"),
				"decision":  stringProp("The decision text (required for create)"),
				"rationale": stringProp("Why this decision was made"),
				"question":  stringProp("The question this decision answers"),
				"tags":      arrayProp("Tags to associate with the decision", map[string]any{"type": "string"}),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *decisionTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "get":
		return t.handleGet(ctx, params)
	case "list":
		return t.handleList(ctx, params)
	case "create":
		return t.handleCreate(ctx, params)
	case "update":
		return t.handleUpdate(ctx, params)
	default:
		return errResult(fmt.Sprintf("unknown action %q — valid: get, list, create, update", action)), nil
	}
}

func (t *decisionTool) handleGet(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for get"), nil
	}
	resp, err := t.client.Decision.GetDecision(ctx, connect.NewRequest(&specv1.GetDecisionRequest{
		Slug: slug,
	}))
	if err != nil {
		result, rerr := connectErrResult(err)
		return result, rerr
	}
	return jsonResult(resp.Msg), nil
}

func (t *decisionTool) handleList(ctx context.Context, _ map[string]any) (*ToolResult, error) {
	resp, err := t.client.Decision.ListDecisions(ctx, connect.NewRequest(&specv1.ListDecisionsRequest{}))
	if err != nil {
		result, rerr := connectErrResult(err)
		return result, rerr
	}
	return jsonResult(resp.Msg), nil
}

func (t *decisionTool) handleCreate(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	title := stringParam(params, "title")
	decision := stringParam(params, "decision")
	if slug == "" {
		return errResult("slug is required for create"), nil
	}
	if title == "" {
		return errResult("title is required for create"), nil
	}
	if decision == "" {
		return errResult("decision is required for create"), nil
	}
	resp, err := t.client.Decision.CreateDecision(ctx, connect.NewRequest(&specv1.CreateDecisionRequest{
		Slug:      slug,
		Title:     title,
		Decision:  decision,
		Rationale: stringParam(params, "rationale"),
		Question:  stringParam(params, "question"),
		Tags:      stringSliceParam(params, "tags"),
	}))
	if err != nil {
		result, rerr := connectErrResult(err)
		return result, rerr
	}
	return jsonResult(resp.Msg), nil
}

func (t *decisionTool) handleUpdate(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for update"), nil
	}
	resp, err := t.client.Decision.UpdateDecision(ctx, connect.NewRequest(&specv1.UpdateDecisionRequest{
		Slug:      slug,
		Title:     stringPtrParam(params, "title"),
		Decision:  stringPtrParam(params, "decision"),
		Rationale: stringPtrParam(params, "rationale"),
		Question:  stringPtrParam(params, "question"),
		Tags:      stringSliceParam(params, "tags"),
	}))
	if err != nil {
		result, rerr := connectErrResult(err)
		return result, rerr
	}
	return jsonResult(resp.Msg), nil
}
