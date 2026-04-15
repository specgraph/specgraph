// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// RegisterLifecycleTools registers drift, lint, sync, and export tools.
func RegisterLifecycleTools(r *Registry, c *Client) {
	dt := &driftTool{client: c}
	r.AddTool(dt.def())

	lt := &lintTool{client: c}
	r.AddTool(lt.def())

	st := &syncTool{client: c}
	r.AddTool(st.def())

	et := &exportTool{client: c}
	r.AddTool(et.def())
}

// driftScopeFromString converts a friendly string (e.g. "deps") to a DriftScope enum.
func driftScopeFromString(s string) specv1.DriftScope {
	key := "DRIFT_SCOPE_" + strings.ToUpper(s)
	if v, ok := specv1.DriftScope_value[key]; ok {
		return specv1.DriftScope(v)
	}
	return specv1.DriftScope_DRIFT_SCOPE_UNSPECIFIED
}

// ---------------------------------------------------------------------------
// driftTool — check and acknowledge spec drift
// ---------------------------------------------------------------------------

type driftTool struct {
	client *Client
}

func (t *driftTool) def() ToolDef {
	return ToolDef{
		Name: "drift",
		Description: "Check for and acknowledge spec drift against upstream dependencies. " +
			"Actions: check, acknowledge.",
		Profile: ProfileAuthoring,
		Schema: objectSchema(
			props{
				"action":        stringProp("Operation to perform", "check", "acknowledge"),
				"slug":          stringProp("Spec slug (empty = check all eligible specs for check)"),
				"scope":         stringProp("Drift scope filter: deps, interfaces, verify"),
				"note":          stringProp("Acknowledgment note explaining why drift is accepted (required for acknowledge)"),
				"upstream_slug": stringProp("Specific upstream slug to acknowledge (use with acknowledge)"),
				"all":           boolProp("Acknowledge all upstream dependencies at once"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *driftTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "check":
		return t.handleCheck(ctx, params)
	case "acknowledge":
		return t.handleAcknowledge(ctx, params)
	default:
		return errResult(fmt.Sprintf("unknown action %q — valid: check, acknowledge", action)), nil
	}
}

func (t *driftTool) handleCheck(ctx context.Context, params map[string]any) (*ToolResult, error) {
	resp, err := t.client.Lifecycle.CheckDrift(ctx, connect.NewRequest(&specv1.DriftCheckRequest{
		Slug:  stringParam(params, "slug"),
		Scope: driftScopeFromString(stringParam(params, "scope")),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *driftTool) handleAcknowledge(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for acknowledge"), nil
	}
	note := stringParam(params, "note")
	if note == "" {
		return errResult("note is required for acknowledge"), nil
	}
	resp, err := t.client.Lifecycle.AcknowledgeDrift(ctx, connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug:         slug,
		Note:         note,
		UpstreamSlug: stringParam(params, "upstream_slug"),
		All:          boolParam(params, "all"),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// ---------------------------------------------------------------------------
// lintTool — lint specs
// ---------------------------------------------------------------------------

type lintTool struct {
	client *Client
}

func (t *lintTool) def() ToolDef {
	return ToolDef{
		Name:        "lint",
		Description: "Lint one or all specs for structural and rule violations. Leave slug empty to lint all specs.",
		Profile:     ProfileAuthoring,
		Schema: objectSchema(
			props{
				"slug": stringProp("Spec slug to lint (empty = lint all specs)"),
			},
		),
		Handler: t.handle,
	}
}

func (t *lintTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	resp, err := t.client.Lifecycle.Lint(ctx, connect.NewRequest(&specv1.LintRequest{
		Slug: stringParam(params, "slug"),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// ---------------------------------------------------------------------------
// syncTool — sync specs with external issue trackers and VCS
// ---------------------------------------------------------------------------

type syncTool struct {
	client *Client
}

func (t *syncTool) def() ToolDef {
	return ToolDef{
		Name: "sync",
		Description: "Sync specs with external issue trackers and VCS. " +
			"Actions: beads, github, status.",
		Profile: ProfileAuthoring,
		Schema: objectSchema(
			props{
				"action": stringProp("Operation to perform", "beads", "github", "status"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *syncTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "beads":
		return t.handleBeads(ctx)
	case "github":
		return t.handleGitHub(ctx)
	case "status":
		return t.handleStatus(ctx)
	default:
		return errResult(fmt.Sprintf("unknown action %q — valid: beads, github, status", action)), nil
	}
}

func (t *syncTool) handleBeads(ctx context.Context) (*ToolResult, error) {
	resp, err := t.client.Sync.SyncBeads(ctx, connect.NewRequest(&specv1.SyncBeadsRequest{}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *syncTool) handleGitHub(ctx context.Context) (*ToolResult, error) {
	resp, err := t.client.Sync.SyncGitHub(ctx, connect.NewRequest(&specv1.SyncGitHubRequest{}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *syncTool) handleStatus(ctx context.Context) (*ToolResult, error) {
	resp, err := t.client.Sync.GetSyncStatus(ctx, connect.NewRequest(&specv1.SyncStatusRequest{}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// ---------------------------------------------------------------------------
// exportTool — export, import, and verify project bundles
// ---------------------------------------------------------------------------

type exportTool struct {
	client *Client
}

func (t *exportTool) def() ToolDef {
	return ToolDef{
		Name: "export",
		Description: "Export, import, and verify SpecGraph project bundles. " +
			"Actions: export, import, verify. " +
			"For import and verify, pass the export data as a base64-encoded string.",
		Profile: ProfileAuthoring,
		Schema: objectSchema(
			props{
				"action":            stringProp("Operation to perform", "export", "import", "verify"),
				"project_slug":      stringProp("Project slug (required for export; optional hint for verify)"),
				"data":              stringProp("Base64-encoded export bundle data (required for import and verify)"),
				"force":             boolProp("Overwrite existing data on import"),
				"require_signature": boolProp("Require a valid signature on import"),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *exportTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "export":
		return t.handleExport(ctx, params)
	case "import":
		return t.handleImport(ctx, params)
	case "verify":
		return t.handleVerify(ctx, params)
	default:
		return errResult(fmt.Sprintf("unknown action %q — valid: export, import, verify", action)), nil
	}
}

func (t *exportTool) handleExport(ctx context.Context, params map[string]any) (*ToolResult, error) {
	projectSlug := stringParam(params, "project_slug")
	if projectSlug == "" {
		return errResult("project_slug is required for export"), nil
	}
	resp, err := t.client.Export.ExportProject(ctx, connect.NewRequest(&specv1.ExportProjectRequest{
		ProjectSlug: projectSlug,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	// Return base64-encoded export data for easy CLI consumption.
	encoded := base64.StdEncoding.EncodeToString(resp.Msg.GetData())
	return textResult(encoded), nil
}

func (t *exportTool) handleImport(ctx context.Context, params map[string]any) (*ToolResult, error) {
	dataStr := stringParam(params, "data")
	if dataStr == "" {
		return errResult("data is required for import (base64-encoded export bundle)"), nil
	}
	data, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		return errResult(fmt.Sprintf("invalid base64 data: %v", err)), nil
	}
	resp, err := t.client.Export.ImportProject(ctx, connect.NewRequest(&specv1.ImportProjectRequest{
		Data:             data,
		Force:            boolParam(params, "force"),
		RequireSignature: boolParam(params, "require_signature"),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

func (t *exportTool) handleVerify(ctx context.Context, params map[string]any) (*ToolResult, error) {
	dataStr := stringParam(params, "data")
	if dataStr == "" {
		return errResult("data is required for verify (base64-encoded export bundle)"), nil
	}
	data, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		return errResult(fmt.Sprintf("invalid base64 data: %v", err)), nil
	}
	resp, err := t.client.Export.VerifyExport(ctx, connect.NewRequest(&specv1.VerifyExportRequest{
		Data:        data,
		ProjectSlug: stringParam(params, "project_slug"),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}
