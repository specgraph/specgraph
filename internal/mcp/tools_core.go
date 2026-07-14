// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	constload "github.com/specgraph/specgraph/internal/constitution/load"
)

// RegisterCoreTools registers the constitution, findings, and health tools into the registry.
func RegisterCoreTools(r *Registry, c *Client) {
	ct := &constitutionTool{client: c}
	r.AddTool(ct.def())

	ft := &findingsTool{client: c}
	r.AddTool(ft.def())

	ht := &healthTool{client: c}
	r.AddTool(ht.def())
}

// constitutionLayerFromString converts a friendly string (e.g. "project") to a ConstitutionLayer enum.
func constitutionLayerFromString(s string) specv1.ConstitutionLayer {
	key := "CONSTITUTION_LAYER_" + strings.ToUpper(s)
	if v, ok := specv1.ConstitutionLayer_value[key]; ok {
		return specv1.ConstitutionLayer(v)
	}
	return specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED
}

// passTypeFromString converts a friendly string (e.g. "constitution-check") to a PassType enum.
func passTypeFromString(s string) specv1.PassType {
	key := "PASS_TYPE_" + strings.ToUpper(strings.ReplaceAll(s, "-", "_"))
	if v, ok := specv1.PassType_value[key]; ok {
		return specv1.PassType(v)
	}
	return specv1.PassType_PASS_TYPE_UNSPECIFIED
}

// ---------------------------------------------------------------------------
// constitutionTool — get and update the project constitution
// ---------------------------------------------------------------------------

type constitutionTool struct {
	client *Client
}

func (t *constitutionTool) def() ToolDef {
	return ToolDef{
		Name: "constitution",
		Description: "Read and write the SpecGraph project constitution (layered ground truth). " +
			"Actions: get, update. For update, pass friendly YAML in the `constitution` " +
			"param — e.g. `layer: project`, `name: my-project`, plus optional `tech`, " +
			"`principles`, `constraints`, `antipatterns`, `process`, and `references` " +
			"(reference `type: adr`). No protojson/enum names required.",
		Profile: ProfileCore,
		Schema: objectSchema(
			props{
				"action": stringProp("Operation to perform", "get", "update"),
				"layer": stringProp(
					"Constitution layer filter for get: user, org, project, domain",
					"user", "org", "project", "domain",
				),
				"constitution": stringProp(
					"Constitution as friendly YAML for update. Set an explicit `layer:` " +
						"(user|org|project|domain — required) and `name:`. Optional sections: " +
						"`tech` (languages/frameworks/infrastructure/api_standards/data), " +
						"`principles[]` (id/statement/rationale/exceptions), `constraints[]`, " +
						"`antipatterns[]` (pattern/why/instead), `process`, `references[]` " +
						"(type: adr|spec|doc|url, path). Example: `layer: project` + " +
						"`name: my-project` + `constraints: [\"no vendor lock-in\"]`.",
				),
			},
			"action",
		),
		Handler: t.handle,
	}
}

func (t *constitutionTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "get":
		return t.handleGet(ctx, params)
	case "update":
		return t.handleUpdate(ctx, params)
	default:
		return errResult(fmt.Sprintf("unknown action %q — valid: get, update", action)), nil
	}
}

func (t *constitutionTool) handleGet(ctx context.Context, params map[string]any) (*ToolResult, error) {
	layerStr := stringParam(params, "layer")
	layer := specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED
	if layerStr != "" {
		layer = constitutionLayerFromString(layerStr)
		if layer == specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED {
			return errResult("invalid layer (valid: user, org, project, domain)"), nil
		}
	}
	resp, err := t.client.Constitution.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{
		Layer: layer,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg.GetConstitution()), nil
}

func (t *constitutionTool) handleUpdate(ctx context.Context, params map[string]any) (*ToolResult, error) {
	raw := stringParam(params, "constitution")
	if raw == "" {
		return errResult("constitution is required for update (pass friendly YAML with an explicit layer, e.g. `layer: project`)"), nil
	}
	con, err := constload.FromYAML([]byte(raw))
	if err != nil {
		// Sanitized: never surface raw yaml/protojson parser internals (T-06-03).
		return errResult("invalid constitution input (expected friendly YAML with fields like `layer: project`, `name: ...`)"), nil
	}
	// Explicit-layer guard: FromYAML permits an empty layer, but a project
	// bootstrap update needs a real one — reject rather than persist an
	// empty-layer constitution.
	if con.Layer == "" {
		return errResult("constitution layer is required (set `layer:` to one of user, org, project, domain)"), nil
	}
	resp, err := t.client.Constitution.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
		Constitution: constload.ToProto(con),
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// ---------------------------------------------------------------------------
// findingsTool — list analytical pass findings for a spec
// ---------------------------------------------------------------------------

type findingsTool struct {
	client *Client
}

func (t *findingsTool) def() ToolDef {
	return ToolDef{
		Name: "findings",
		Description: "List analytical pass findings for a spec. " +
			"Actions: list.",
		Profile: ProfileCore,
		Schema: objectSchema(
			props{
				"action": stringProp("Operation to perform", "list"),
				"slug":   stringProp("Spec slug (required for list)"),
				"pass_type": stringProp(
					"Filter by pass type: constitution-check, red-team, peripheral-vision, consistency, simplicity",
					"constitution-check", "red-team", "peripheral-vision", "consistency", "simplicity",
				),
			},
			"action", "slug",
		),
		Handler: t.handle,
	}
}

func (t *findingsTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
	action := stringParam(params, "action")
	switch action {
	case "list":
		return t.handleList(ctx, params)
	default:
		return errResult(fmt.Sprintf("unknown action %q — valid: list", action)), nil
	}
}

func (t *findingsTool) handleList(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for list"), nil
	}
	passTypeStr := stringParam(params, "pass_type")
	passType := specv1.PassType_PASS_TYPE_UNSPECIFIED
	if passTypeStr != "" {
		passType = passTypeFromString(passTypeStr)
		if passType == specv1.PassType_PASS_TYPE_UNSPECIFIED {
			return errResult("invalid pass_type for list"), nil
		}
	}
	resp, err := t.client.AnalyticalPass.ListFindings(ctx, connect.NewRequest(&specv1.ListFindingsRequest{
		Slug:     slug,
		PassType: passType,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}

// ---------------------------------------------------------------------------
// healthTool — check server health
// ---------------------------------------------------------------------------

type healthTool struct {
	client *Client
}

func (t *healthTool) def() ToolDef {
	return ToolDef{
		Name:        "health",
		Description: "Check SpecGraph server health and version.",
		Profile:     ProfileCore,
		Schema:      objectSchema(props{}),
		Handler:     t.handle,
	}
}

func (t *healthTool) handle(ctx context.Context, _ map[string]any) (*ToolResult, error) {
	resp, err := t.client.Health.Health(ctx, connect.NewRequest(&specv1.HealthRequest{}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}
