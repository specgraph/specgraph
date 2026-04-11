// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

// RegisterExecutionTools registers claim, slice, bundle, prime, report, and
// execution_events tools into the registry.
func RegisterExecutionTools(r *Registry, c *Client) {
	ct := &claimTool{client: c}
	r.AddTool(ct.def())

	st := &sliceTool{client: c}
	r.AddTool(st.def())

	bt := &bundleTool{client: c}
	r.AddTool(bt.def())

	pt := &primeTool{client: c}
	r.AddTool(pt.def())

	rt := &reportTool{client: c}
	r.AddTool(rt.def())

	et := &executionEventsTool{client: c}
	r.AddTool(et.def())
}

// ---------------------------------------------------------------------------
// claimTool — claim, unclaim, heartbeat on specs
// ---------------------------------------------------------------------------

type claimTool struct {
	client *Client
}

func (t *claimTool) def() ToolDef {
	return ToolDef{
		Name: "claim",
		Description: "Manage spec execution claims for agents. " +
			"Actions: claim, unclaim, heartbeat.",
		Tier: TierExecution,
		Schema: objectSchema(
			props{
				"action":         stringProp("Operation to perform", "claim", "unclaim", "heartbeat"),
				"spec_slug":      stringProp("Spec slug (required for all actions)"),
				"agent":          stringProp("Agent identifier (required for all actions)"),
				"lease_duration": stringProp("Lease duration string e.g. '30m', '1h' (optional, default 15m)"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *claimTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "claim":
		return t.handleClaim(ctx, params)
	case "unclaim":
		return t.handleUnclaim(ctx, params)
	case "heartbeat":
		return t.handleHeartbeat(ctx, params)
	default:
		return errResult(fmt.Sprintf("unknown action %q — valid: claim, unclaim, heartbeat", action)), nil
	}
}

func (t *claimTool) handleClaim(ctx context.Context, params map[string]any) (*ToolResult, error) {
	specSlug := stringParam(params, "spec_slug")
	if specSlug == "" {
		return errResult("spec_slug is required for claim"), nil
	}
	agent := stringParam(params, "agent")
	if agent == "" {
		return errResult("agent is required for claim"), nil
	}
	req := &specv1.ClaimSpecRequest{
		SpecSlug: specSlug,
		Agent:    agent,
	}
	if durStr := stringParam(params, "lease_duration"); durStr != "" {
		d, err := time.ParseDuration(durStr)
		if err != nil {
			return errResult(fmt.Sprintf("invalid lease_duration %q: %v", durStr, err)), nil
		}
		req.LeaseDuration = durationpb.New(d)
	}
	resp, err := t.client.Claim.ClaimSpec(ctx, connect.NewRequest(req))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *claimTool) handleUnclaim(ctx context.Context, params map[string]any) (*ToolResult, error) {
	specSlug := stringParam(params, "spec_slug")
	if specSlug == "" {
		return errResult("spec_slug is required for unclaim"), nil
	}
	agent := stringParam(params, "agent")
	if agent == "" {
		return errResult("agent is required for unclaim"), nil
	}
	resp, err := t.client.Claim.UnclaimSpec(ctx, connect.NewRequest(&specv1.UnclaimSpecRequest{
		SpecSlug: specSlug,
		Agent:    agent,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *claimTool) handleHeartbeat(ctx context.Context, params map[string]any) (*ToolResult, error) {
	specSlug := stringParam(params, "spec_slug")
	if specSlug == "" {
		return errResult("spec_slug is required for heartbeat"), nil
	}
	agent := stringParam(params, "agent")
	if agent == "" {
		return errResult("agent is required for heartbeat"), nil
	}
	resp, err := t.client.Claim.Heartbeat(ctx, connect.NewRequest(&specv1.HeartbeatRequest{
		SpecSlug: specSlug,
		Agent:    agent,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// ---------------------------------------------------------------------------
// sliceTool — list, get, claim, complete slices
// ---------------------------------------------------------------------------

type sliceTool struct {
	client *Client
}

func (t *sliceTool) def() ToolDef {
	return ToolDef{
		Name: "slice",
		Description: "Manage execution slices (discrete work items from the decompose stage). " +
			"Actions: list, get, claim, complete.",
		Tier: TierExecution,
		Schema: objectSchema(
			props{
				"action":      stringProp("Operation to perform", "list", "get", "claim", "complete"),
				"parent_slug": stringProp("Parent spec slug (required for list)"),
				"slug":        stringProp("Full slice slug e.g. 'parent/slc-01' (required for get/claim/complete)"),
				"assignee":    stringProp("Who is claiming the slice (required for claim)"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *sliceTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "list":
		return t.handleList(ctx, params)
	case "get":
		return t.handleGet(ctx, params)
	case "claim":
		return t.handleClaim(ctx, params)
	case "complete":
		return t.handleComplete(ctx, params)
	default:
		return errResult(fmt.Sprintf("unknown action %q — valid: list, get, claim, complete", action)), nil
	}
}

func (t *sliceTool) handleList(ctx context.Context, params map[string]any) (*ToolResult, error) {
	parentSlug := stringParam(params, "parent_slug")
	if parentSlug == "" {
		return errResult("parent_slug is required for list"), nil
	}
	resp, err := t.client.Slice.ListSlices(ctx, connect.NewRequest(&specv1.ListSlicesRequest{
		ParentSlug: parentSlug,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *sliceTool) handleGet(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for get"), nil
	}
	resp, err := t.client.Slice.GetSlice(ctx, connect.NewRequest(&specv1.GetSliceRequest{
		Slug: slug,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *sliceTool) handleClaim(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for claim"), nil
	}
	assignee := stringParam(params, "assignee")
	if assignee == "" {
		return errResult("assignee is required for claim"), nil
	}
	resp, err := t.client.Slice.ClaimSlice(ctx, connect.NewRequest(&specv1.ClaimSliceRequest{
		Slug:     slug,
		Assignee: assignee,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *sliceTool) handleComplete(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for complete"), nil
	}
	resp, err := t.client.Slice.CompleteSlice(ctx, connect.NewRequest(&specv1.CompleteSliceRequest{
		Slug: slug,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// ---------------------------------------------------------------------------
// bundleTool — generate an execution bundle for a spec
// ---------------------------------------------------------------------------

type bundleTool struct {
	client *Client
}

func (t *bundleTool) def() ToolDef {
	return ToolDef{
		Name: "bundle",
		Description: "Generate an execution bundle for a spec. " +
			"The bundle includes the spec, linked decisions, bootstrap instructions, and callback config.",
		Tier: TierExecution,
		Schema: objectSchema(
			props{
				"slug":     stringProp("Spec slug (required)"),
				"endpoint": stringProp("Callback base URL for the executing agent (optional)"),
			},
			"slug",
		),
		Handler: t.handle,
	}
}

func (t *bundleTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required"), nil
	}
	resp, err := t.client.Execution.GenerateBundle(ctx, connect.NewRequest(&specv1.GenerateBundleRequest{
		Slug:     slug,
		Endpoint: stringParam(params, "endpoint"),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// ---------------------------------------------------------------------------
// primeTool — get project priming context for a spec
// ---------------------------------------------------------------------------

type primeTool struct {
	client *Client
}

func (t *primeTool) def() ToolDef {
	return ToolDef{
		Name: "prime",
		Description: "Retrieve project priming context for a spec: constitution summary, " +
			"project context, decisions, coding conventions, and callback docs.",
		Tier: TierExecution,
		Schema: objectSchema(
			props{
				"slug": stringProp("Spec slug (required)"),
			},
			"slug",
		),
		Handler: t.handle,
	}
}

func (t *primeTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required"), nil
	}
	resp, err := t.client.Execution.GetPrime(ctx, connect.NewRequest(&specv1.GetPrimeRequest{
		Slug: slug,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// ---------------------------------------------------------------------------
// reportTool — report progress, blockers, or completion
// ---------------------------------------------------------------------------

type reportTool struct {
	client *Client
}

func (t *reportTool) def() ToolDef {
	return ToolDef{
		Name: "report",
		Description: "Report execution progress, blockers, or completion for a spec. " +
			"Actions: progress, blocker, completion.",
		Tier: TierExecution,
		Schema: objectSchema(
			props{
				"action":      stringProp("Operation to perform", "progress", "blocker", "completion"),
				"slug":        stringProp("Spec slug (required for all actions)"),
				"agent":       stringProp("Agent identifier (required for all actions)"),
				"message":     stringProp("Progress message (required for progress)"),
				"description": stringProp("Blocker description (required for blocker)"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *reportTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "progress":
		return t.handleProgress(ctx, params)
	case "blocker":
		return t.handleBlocker(ctx, params)
	case "completion":
		return t.handleCompletion(ctx, params)
	default:
		return errResult(fmt.Sprintf("unknown action %q — valid: progress, blocker, completion", action)), nil
	}
}

func (t *reportTool) handleProgress(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for progress"), nil
	}
	agent := stringParam(params, "agent")
	if agent == "" {
		return errResult("agent is required for progress"), nil
	}
	message := stringParam(params, "message")
	if message == "" {
		return errResult("message is required for progress"), nil
	}
	resp, err := t.client.Execution.ReportProgress(ctx, connect.NewRequest(&specv1.ReportProgressRequest{
		Slug:    slug,
		Agent:   agent,
		Message: message,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *reportTool) handleBlocker(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for blocker"), nil
	}
	agent := stringParam(params, "agent")
	if agent == "" {
		return errResult("agent is required for blocker"), nil
	}
	description := stringParam(params, "description")
	if description == "" {
		return errResult("description is required for blocker"), nil
	}
	resp, err := t.client.Execution.ReportBlocker(ctx, connect.NewRequest(&specv1.ReportBlockerRequest{
		Slug:        slug,
		Agent:       agent,
		Description: description,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *reportTool) handleCompletion(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for completion"), nil
	}
	agent := stringParam(params, "agent")
	if agent == "" {
		return errResult("agent is required for completion"), nil
	}
	resp, err := t.client.Execution.ReportCompletion(ctx, connect.NewRequest(&specv1.ReportCompletionRequest{
		Slug:  slug,
		Agent: agent,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// ---------------------------------------------------------------------------
// executionEventsTool — list execution events for a spec
// ---------------------------------------------------------------------------

type executionEventsTool struct {
	client *Client
}

func (t *executionEventsTool) def() ToolDef {
	return ToolDef{
		Name: "execution_events",
		Description: "List execution events (progress, blockers, completions) recorded against a spec. " +
			"Returns events in reverse-chronological order.",
		Tier: TierExecution,
		Schema: objectSchema(
			props{
				"slug":  stringProp("Spec slug (required)"),
				"limit": intProp("Maximum number of events to return (default: all)"),
			},
			"slug",
		),
		Handler: t.handle,
	}
}

func (t *executionEventsTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required"), nil
	}
	limit := int32Param(params, "limit")
	resp, err := t.client.Execution.GetExecutionEvents(ctx, connect.NewRequest(&specv1.GetExecutionEventsRequest{
		Slug:  slug,
		Limit: uint32(limit), //nolint:gosec // limit is a bounded API page size from int32
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}
