// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"testing"

	sdkmcp "github.com/mark3labs/mcp-go/mcp"
)

func TestToSDKTool(t *testing.T) {
	def := ToolDef{
		Name:        "spec_get",
		Description: "Get a spec by slug",
		Tier:        TierCore,
		Schema: objectSchema(props{
			"slug": stringProp("The spec slug"),
		}, "slug"),
	}

	tool := toSDKTool(def)

	if tool.Name != "spec_get" {
		t.Errorf("Name = %q, want %q", tool.Name, "spec_get")
	}
	if tool.Description != "Get a spec by slug" {
		t.Errorf("Description = %q, want %q", tool.Description, "Get a spec by slug")
	}
	if tool.InputSchema.Type != "object" {
		t.Errorf("InputSchema.Type = %q, want %q", tool.InputSchema.Type, "object")
	}
	if _, ok := tool.InputSchema.Properties["slug"]; !ok {
		t.Error("InputSchema.Properties missing 'slug'")
	}
	if len(tool.InputSchema.Required) != 1 || tool.InputSchema.Required[0] != "slug" {
		t.Errorf("InputSchema.Required = %v, want [slug]", tool.InputSchema.Required)
	}
}

func TestToSDKTool_NoRequired(t *testing.T) {
	def := ToolDef{
		Name:        "health_check",
		Description: "Check server health",
		Schema: objectSchema(props{
			"verbose": boolProp("Include details"),
		}),
	}

	tool := toSDKTool(def)

	if len(tool.InputSchema.Required) != 0 {
		t.Errorf("InputSchema.Required = %v, want empty", tool.InputSchema.Required)
	}
}

func TestToSDKResult_Success(t *testing.T) {
	r := textResult("hello world")

	got := toSDKResult(r)

	if got.IsError {
		t.Error("IsError = true, want false")
	}
	if len(got.Content) != 1 {
		t.Fatalf("Content length = %d, want 1", len(got.Content))
	}
	tc, ok := sdkmcp.AsTextContent(got.Content[0])
	if !ok {
		t.Fatal("Content[0] is not TextContent")
	}
	if tc.Text != "hello world" {
		t.Errorf("Text = %q, want %q", tc.Text, "hello world")
	}
}

func TestToSDKResult_Error(t *testing.T) {
	r := errResult("something went wrong")

	got := toSDKResult(r)

	if !got.IsError {
		t.Error("IsError = false, want true")
	}
	if len(got.Content) != 1 {
		t.Fatalf("Content length = %d, want 1", len(got.Content))
	}
	tc, ok := sdkmcp.AsTextContent(got.Content[0])
	if !ok {
		t.Fatal("Content[0] is not TextContent")
	}
	if tc.Text != "something went wrong" {
		t.Errorf("Text = %q, want %q", tc.Text, "something went wrong")
	}
}

func TestToSDKResult_MultipleContent(t *testing.T) {
	r := &ToolResult{
		Content: []Content{
			{Type: "text", Text: "line 1"},
			{Type: "text", Text: "line 2"},
		},
	}

	got := toSDKResult(r)

	if len(got.Content) != 2 {
		t.Fatalf("Content length = %d, want 2", len(got.Content))
	}
}

func TestFromSDKParams(t *testing.T) {
	req := sdkmcp.CallToolRequest{
		Params: sdkmcp.CallToolParams{
			Name: "spec_get",
			Arguments: map[string]any{
				"slug":    "auth-service",
				"version": float64(3),
			},
		},
	}

	params := fromSDKParams(req)

	if params["slug"] != "auth-service" {
		t.Errorf("slug = %v, want %q", params["slug"], "auth-service")
	}
	if params["version"] != float64(3) {
		t.Errorf("version = %v, want 3.0", params["version"])
	}
}

func TestFromSDKParams_Empty(t *testing.T) {
	req := sdkmcp.CallToolRequest{
		Params: sdkmcp.CallToolParams{
			Name:      "health_check",
			Arguments: map[string]any{},
		},
	}

	params := fromSDKParams(req)

	if len(params) != 0 {
		t.Errorf("params length = %d, want 0", len(params))
	}
}

func TestToSDKResource(t *testing.T) {
	def := ResourceDef{
		URI:         "specgraph://constitution",
		Name:        "constitution",
		Description: "Project constitution",
		MimeType:    "application/json",
	}

	res := toSDKResource(def)

	if res.URI != "specgraph://constitution" {
		t.Errorf("URI = %q, want %q", res.URI, "specgraph://constitution")
	}
	if res.Name != "constitution" {
		t.Errorf("Name = %q, want %q", res.Name, "constitution")
	}
	if res.Description != "Project constitution" {
		t.Errorf("Description = %q, want %q", res.Description, "Project constitution")
	}
	if res.MIMEType != "application/json" {
		t.Errorf("MIMEType = %q, want %q", res.MIMEType, "application/json")
	}
}

func TestToSDKResourceTemplate(t *testing.T) {
	def := ResourceDef{
		URI:         "specgraph://specs/{slug}",
		Name:        "spec",
		Description: "A spec by slug",
		MimeType:    "application/json",
		IsTemplate:  true,
	}

	tmpl := toSDKResourceTemplate(def)

	if tmpl.Name != "spec" {
		t.Errorf("Name = %q, want %q", tmpl.Name, "spec")
	}
	if tmpl.Description != "A spec by slug" {
		t.Errorf("Description = %q, want %q", tmpl.Description, "A spec by slug")
	}
	if tmpl.MIMEType != "application/json" {
		t.Errorf("MIMEType = %q, want %q", tmpl.MIMEType, "application/json")
	}
}

func TestToSDKResourceContents(t *testing.T) {
	rcs := []ResourceContent{
		{URI: "specgraph://constitution", MimeType: "application/json", Text: `{"layers":[]}`},
	}

	got := toSDKResourceContents(rcs)

	if len(got) != 1 {
		t.Fatalf("length = %d, want 1", len(got))
	}
	trc, ok := sdkmcp.AsTextResourceContents(got[0])
	if !ok {
		t.Fatal("got[0] is not TextResourceContents")
	}
	if trc.URI != "specgraph://constitution" {
		t.Errorf("URI = %q, want %q", trc.URI, "specgraph://constitution")
	}
	if trc.Text != `{"layers":[]}` {
		t.Errorf("Text = %q, want %q", trc.Text, `{"layers":[]}`)
	}
}

func TestToSDKPrompt(t *testing.T) {
	def := PromptDef{
		Name:        "spec-review",
		Description: "Review a spec for completeness",
		Arguments: []PromptArgument{
			{Name: "slug", Description: "Spec slug", Required: true},
			{Name: "focus", Description: "Focus area", Required: false},
		},
	}

	prompt := toSDKPrompt(def)

	if prompt.Name != "spec-review" {
		t.Errorf("Name = %q, want %q", prompt.Name, "spec-review")
	}
	if prompt.Description != "Review a spec for completeness" {
		t.Errorf("Description = %q, want %q", prompt.Description, "Review a spec for completeness")
	}
	if len(prompt.Arguments) != 2 {
		t.Fatalf("Arguments length = %d, want 2", len(prompt.Arguments))
	}
	if prompt.Arguments[0].Name != "slug" {
		t.Errorf("Arguments[0].Name = %q, want %q", prompt.Arguments[0].Name, "slug")
	}
	if !prompt.Arguments[0].Required {
		t.Error("Arguments[0].Required = false, want true")
	}
	if prompt.Arguments[1].Required {
		t.Error("Arguments[1].Required = true, want false")
	}
}

func TestToSDKPromptResult(t *testing.T) {
	r := &PromptResult{
		Description: "Review results",
		Messages: []PromptMessage{
			{Role: "user", Content: "Please review spec auth-service"},
			{Role: "assistant", Content: "The spec looks good."},
		},
	}

	got := toSDKPromptResult(r)

	if got.Description != "Review results" {
		t.Errorf("Description = %q, want %q", got.Description, "Review results")
	}
	if len(got.Messages) != 2 {
		t.Fatalf("Messages length = %d, want 2", len(got.Messages))
	}
	if got.Messages[0].Role != sdkmcp.RoleUser {
		t.Errorf("Messages[0].Role = %q, want %q", got.Messages[0].Role, sdkmcp.RoleUser)
	}
	if got.Messages[1].Role != sdkmcp.RoleAssistant {
		t.Errorf("Messages[1].Role = %q, want %q", got.Messages[1].Role, sdkmcp.RoleAssistant)
	}
	tc, ok := sdkmcp.AsTextContent(got.Messages[0].Content)
	if !ok {
		t.Fatal("Messages[0].Content is not TextContent")
	}
	if tc.Text != "Please review spec auth-service" {
		t.Errorf("Messages[0] text = %q, want %q", tc.Text, "Please review spec auth-service")
	}
}
