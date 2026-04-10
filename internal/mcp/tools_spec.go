// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
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
		Tier: TierCore,
		Schema: objectSchema(
			props{
				"action": stringProp("Operation to perform",
					"get", "list", "create", "update", "changes", "compare"),
				"slug":         stringProp("Spec slug (required for get/update/changes/compare)"),
				"intent":       stringProp("What the spec is about (required for create)"),
				"stage":        stringProp("Authoring stage filter or update value"),
				"priority":     stringProp("Priority: p0, p1, p2, p3"),
				"complexity":   stringProp("Complexity: low, medium, high"),
				"notes":        stringProp("Free-text notes to attach to the spec"),
				"limit":        intProp("Max results to return for list (default 50)"),
				"from_version": intProp("From-version for compare"),
				"to_version":   intProp("To-version for compare"),
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
	resp, err := t.client.Spec.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:       slug,
		Intent:     intent,
		Priority:   stringParam(params, "priority"),
		Complexity: stringParam(params, "complexity"),
	}))
	if err != nil {
		result, rerr := connectErrResult(err)
		return result, rerr
	}
	return jsonResult(resp.Msg), nil
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
	resp, err := t.client.Spec.CompareVersions(ctx, connect.NewRequest(&specv1.CompareVersionsRequest{
		Slug:        slug,
		FromVersion: int32Param(params, "from_version"),
		ToVersion:   int32Param(params, "to_version"),
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
		Tier: TierCore,
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
