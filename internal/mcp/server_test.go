// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	sdkmcp "github.com/mark3labs/mcp-go/mcp"
)

func TestNewServer_TierToolCounts(t *testing.T) {
	// NewServer accepts a *Client. The handlers are closures over the
	// client's service stubs, but we only need to verify registration
	// counts here — no actual RPCs are made.
	srv := NewServer(&Client{})

	coreSrv := srv.ForTier(TierCore)
	authoringSrv := srv.ForTier(TierAuthoring)
	executionSrv := srv.ForTier(TierExecution)

	// Each tier should be a distinct MCPServer instance.
	if coreSrv == authoringSrv {
		t.Error("core and authoring servers are the same instance")
	}
	if authoringSrv == executionSrv {
		t.Error("authoring and execution servers are the same instance")
	}
	if coreSrv == executionSrv {
		t.Error("core and execution servers are the same instance")
	}

	coreTools := coreSrv.ListTools()
	authoringTools := authoringSrv.ListTools()
	executionTools := executionSrv.ListTools()

	// Core: spec (2) + graph (2) + core (3) = 7 tools
	// Authoring: core (7) + authoring (3) + lifecycle (4) = 14 tools
	// Execution: authoring (14) + execution (6) = 20 tools
	if len(coreTools) != 7 {
		t.Errorf("core tool count = %d, want 7", len(coreTools))
	}
	if len(authoringTools) != 14 {
		t.Errorf("authoring tool count = %d, want 14", len(authoringTools))
	}
	if len(executionTools) != 20 {
		t.Errorf("execution tool count = %d, want 20", len(executionTools))
	}

	// Higher tiers should be strict supersets of lower tiers.
	for name := range coreTools {
		if _, ok := authoringTools[name]; !ok {
			t.Errorf("authoring tier missing core tool %q", name)
		}
		if _, ok := executionTools[name]; !ok {
			t.Errorf("execution tier missing core tool %q", name)
		}
	}
	for name := range authoringTools {
		if _, ok := executionTools[name]; !ok {
			t.Errorf("execution tier missing authoring tool %q", name)
		}
	}
}

func TestNewServer_FallbackToCore(t *testing.T) {
	srv := NewServer(&Client{})

	// An unknown tier value should fall back to core.
	unknown := srv.ForTier(Tier(99))
	core := srv.ForTier(TierCore)

	if unknown != core {
		t.Error("ForTier(99) did not fall back to core server")
	}
}

func TestWrapToolHandler(t *testing.T) {
	called := false
	handler := func(_ context.Context, params map[string]any) (*ToolResult, error) {
		called = true
		slug := stringParam(params, "slug")
		return textResult("got: " + slug), nil
	}

	wrapped := wrapToolHandler(handler)

	req := callToolRequest("test_tool", map[string]any{"slug": "my-spec"})
	result, err := wrapped(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler was not called")
	}
	if result.IsError {
		t.Error("IsError = true, want false")
	}
}

func TestWrapPromptHandler(t *testing.T) {
	handler := func(_ context.Context, args map[string]string) (*PromptResult, error) {
		return &PromptResult{
			Description: "test",
			Messages: []PromptMessage{
				{Role: "user", Content: "hello " + args["name"]},
			},
		}, nil
	}

	wrapped := wrapPromptHandler(handler)

	req := getPromptRequest("test-prompt", map[string]string{"name": "world"})
	result, err := wrapped(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Description != "test" {
		t.Errorf("Description = %q, want %q", result.Description, "test")
	}
	if len(result.Messages) != 1 {
		t.Fatalf("Messages length = %d, want 1", len(result.Messages))
	}
}

func TestWrapResourceHandler(t *testing.T) {
	handler := func(_ context.Context, uri string) ([]ResourceContent, error) {
		return []ResourceContent{
			{URI: uri, MimeType: "text/plain", Text: "content for " + uri},
		}, nil
	}

	wrapped := wrapResourceHandler(handler)

	req := readResourceRequest("specgraph://test")
	result, err := wrapped(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("result length = %d, want 1", len(result))
	}
}

// --- test helpers for constructing SDK request types ---

func callToolRequest(name string, args map[string]any) sdkmcp.CallToolRequest {
	return sdkmcp.CallToolRequest{
		Params: sdkmcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	}
}

func getPromptRequest(name string, args map[string]string) sdkmcp.GetPromptRequest {
	return sdkmcp.GetPromptRequest{
		Params: sdkmcp.GetPromptParams{
			Name:      name,
			Arguments: args,
		},
	}
}

func readResourceRequest(uri string) sdkmcp.ReadResourceRequest {
	return sdkmcp.ReadResourceRequest{
		Params: sdkmcp.ReadResourceParams{
			URI: uri,
		},
	}
}
