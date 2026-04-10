// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// RegisterGraphTools registers the edge and graph_query tools into the registry.
func RegisterGraphTools(r *Registry, c *Client) {
	et := &edgeTool{client: c}
	r.AddTool(et.def())

	gq := &graphQueryTool{client: c}
	r.AddTool(gq.def())
}

// edgeTypeFromString converts a friendly string (e.g. "depends_on") to a specv1.EdgeType enum.
func edgeTypeFromString(s string) specv1.EdgeType {
	key := "EDGE_TYPE_" + strings.ToUpper(strings.ReplaceAll(s, "-", "_"))
	if v, ok := specv1.EdgeType_value[key]; ok {
		return specv1.EdgeType(v)
	}
	return specv1.EdgeType_EDGE_TYPE_UNSPECIFIED
}

// ---------------------------------------------------------------------------
// edgeTool — add, remove, list edges
// ---------------------------------------------------------------------------

type edgeTool struct {
	client *Client
}

func (t *edgeTool) def() ToolDef {
	return ToolDef{
		Name: "edge",
		Description: "Manage edges (relationships) between SpecGraph nodes. " +
			"Actions: add, remove, list.",
		Tier: TierCore,
		Schema: objectSchema(
			props{
				"action": stringProp("Operation to perform", "add", "remove", "list"),
				"slug":   stringProp("Node slug (required for list)"),
				"from_slug": stringProp(
					"Source node slug (required for add/remove)"),
				"to_slug": stringProp(
					"Destination node slug (required for add/remove)"),
				"edge_type": stringProp(
					"Edge type: depends_on, blocks, composes, relates_to, informs, decided_in, supersedes",
					"depends_on", "blocks", "composes", "relates_to", "informs", "decided_in", "supersedes",
				),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *edgeTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "add":
		return t.handleAdd(ctx, params)
	case "remove":
		return t.handleRemove(ctx, params)
	case "list":
		return t.handleList(ctx, params)
	default:
		return errResult(fmt.Sprintf("unknown action %q — valid: add, remove, list", action)), nil
	}
}

func (t *edgeTool) handleAdd(ctx context.Context, params map[string]any) (*ToolResult, error) {
	fromSlug := stringParam(params, "from_slug")
	toSlug := stringParam(params, "to_slug")
	if fromSlug == "" || toSlug == "" {
		return errResult("from_slug and to_slug are required for add"), nil
	}
	resp, err := t.client.Graph.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
		FromSlug: fromSlug,
		ToSlug:   toSlug,
		EdgeType: edgeTypeFromString(stringParam(params, "edge_type")),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *edgeTool) handleRemove(ctx context.Context, params map[string]any) (*ToolResult, error) {
	fromSlug := stringParam(params, "from_slug")
	toSlug := stringParam(params, "to_slug")
	if fromSlug == "" || toSlug == "" {
		return errResult("from_slug and to_slug are required for remove"), nil
	}
	resp, err := t.client.Graph.RemoveEdge(ctx, connect.NewRequest(&specv1.RemoveEdgeRequest{
		FromSlug: fromSlug,
		ToSlug:   toSlug,
		EdgeType: edgeTypeFromString(stringParam(params, "edge_type")),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *edgeTool) handleList(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for list"), nil
	}
	resp, err := t.client.Graph.ListEdges(ctx, connect.NewRequest(&specv1.ListEdgesRequest{
		Slug:     slug,
		EdgeType: edgeTypeFromString(stringParam(params, "edge_type")),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// ---------------------------------------------------------------------------
// graphQueryTool — graph traversal and analytical queries
// ---------------------------------------------------------------------------

type graphQueryTool struct {
	client *Client
}

func (t *graphQueryTool) def() ToolDef {
	return ToolDef{
		Name: "graph_query",
		Description: "Query the SpecGraph dependency graph. " +
			"Actions: dependencies, transitive_deps, impact, ready, critical_path, full.",
		Tier: TierCore,
		Schema: objectSchema(
			props{
				"action": stringProp(
					"Query to run",
					"dependencies", "transitive_deps", "impact", "ready", "critical_path", "full",
				),
				"slug": stringProp(
					"Spec slug (required for dependencies, transitive_deps, impact, critical_path)"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *graphQueryTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "dependencies":
		return t.handleDeps(ctx, params)
	case "transitive_deps":
		return t.handleTransDeps(ctx, params)
	case "impact":
		return t.handleImpact(ctx, params)
	case "ready":
		return t.handleReady(ctx)
	case "critical_path":
		return t.handleCriticalPath(ctx, params)
	case "full":
		return t.handleFull(ctx)
	default:
		return errResult(fmt.Sprintf(
			"unknown action %q — valid: dependencies, transitive_deps, impact, ready, critical_path, full",
			action,
		)), nil
	}
}

func (t *graphQueryTool) requireSlug(params map[string]any) (string, *ToolResult) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return "", errResult("slug is required for this action")
	}
	return slug, nil
}

func (t *graphQueryTool) handleDeps(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug, errRes := t.requireSlug(params)
	if errRes != nil {
		return errRes, nil
	}
	resp, err := t.client.Graph.GetDependencies(ctx, connect.NewRequest(&specv1.GetDependenciesRequest{
		Slug: slug,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *graphQueryTool) handleTransDeps(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug, errRes := t.requireSlug(params)
	if errRes != nil {
		return errRes, nil
	}
	resp, err := t.client.Graph.GetTransitiveDeps(ctx, connect.NewRequest(&specv1.GetTransitiveDepsRequest{
		Slug: slug,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *graphQueryTool) handleImpact(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug, errRes := t.requireSlug(params)
	if errRes != nil {
		return errRes, nil
	}
	resp, err := t.client.Graph.GetImpact(ctx, connect.NewRequest(&specv1.GetImpactRequest{
		Slug: slug,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *graphQueryTool) handleReady(ctx context.Context) (*ToolResult, error) {
	resp, err := t.client.Graph.GetReady(ctx, connect.NewRequest(&specv1.GetReadyRequest{}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *graphQueryTool) handleCriticalPath(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug, errRes := t.requireSlug(params)
	if errRes != nil {
		return errRes, nil
	}
	resp, err := t.client.Graph.GetCriticalPath(ctx, connect.NewRequest(&specv1.GetCriticalPathRequest{
		Slug: slug,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *graphQueryTool) handleFull(ctx context.Context) (*ToolResult, error) {
	resp, err := t.client.Graph.GetFullGraph(ctx, connect.NewRequest(&specv1.GetFullGraphRequest{}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}
