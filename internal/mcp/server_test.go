// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	sdkmcp "github.com/mark3labs/mcp-go/mcp"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestNewServer_AllToolsRegistered(t *testing.T) {
	srv := NewServer(&Client{})
	tools := srv.MCPServer().ListTools()
	if len(tools) != 24 {
		t.Errorf("total tool count = %d, want 24", len(tools))
	}
}

func TestNewServer_ProfileToolSets(t *testing.T) {
	srv := NewServer(&Client{})

	tests := []struct {
		profile Profile
		want    int
	}{
		{ProfileCore, 10},
		{ProfileAuthoring, 18},
		{ProfileExecution, 24},
	}

	for _, tt := range tests {
		t.Run(tt.profile.String(), func(t *testing.T) {
			tools := srv.ToolsForProfile(tt.profile)
			if len(tools) != tt.want {
				t.Errorf("ToolsForProfile(%s) count = %d, want %d", tt.profile, len(tools), tt.want)
			}
		})
	}

	// Higher profiles should be strict supersets.
	core := srv.ToolsForProfile(ProfileCore)
	authoring := srv.ToolsForProfile(ProfileAuthoring)
	execution := srv.ToolsForProfile(ProfileExecution)

	for name := range core {
		if _, ok := authoring[name]; !ok {
			t.Errorf("authoring profile missing core tool %q", name)
		}
		if _, ok := execution[name]; !ok {
			t.Errorf("execution profile missing core tool %q", name)
		}
	}
	for name := range authoring {
		if _, ok := execution[name]; !ok {
			t.Errorf("execution profile missing authoring tool %q", name)
		}
	}
}

func TestNewServer_FallbackToCore(t *testing.T) {
	srv := NewServer(&Client{})
	unknown := srv.ToolsForProfile(Profile(99))
	core := srv.ToolsForProfile(ProfileCore)
	if len(unknown) != len(core) {
		t.Errorf("ToolsForProfile(99) returned %d tools, want %d (core)", len(unknown), len(core))
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

// recordSpans installs an in-memory SpanRecorder as the global tracer provider
// for the duration of a test, restoring the previous provider on cleanup. The
// MCP span helper resolves the global tracer, so this captures the spans it
// emits.
func recordSpans(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return rec
}

// TestMCPSpan_ResourceURIBoundedName asserts that resource span names stay
// low-cardinality (the concrete URI is a span attribute, not part of the span
// name), so an unbounded set of resource URIs can never explode span-name
// cardinality in trace backends.
func TestMCPSpan_ResourceURIBoundedName(t *testing.T) {
	rec := recordSpans(t)

	const uri = "specgraph://spec/oauth-refresh"
	handler := func(_ context.Context, u string) ([]ResourceContent, error) {
		return []ResourceContent{{URI: u, MimeType: "text/plain", Text: "x"}}, nil
	}
	if _, err := wrapResourceHandler(handler)(context.Background(), readResourceRequest(uri)); err != nil {
		t.Fatalf("handler: %v", err)
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	span := spans[0]
	if span.Name() != "mcp.resource" {
		t.Errorf("span name = %q, want %q (the URI must NOT be in the span name)", span.Name(), "mcp.resource")
	}
	var gotURI string
	var found bool
	for _, kv := range span.Attributes() {
		if string(kv.Key) == "mcp.resource.uri" {
			found, gotURI = true, kv.Value.AsString()
		}
	}
	if !found {
		t.Fatalf("span missing mcp.resource.uri attribute; attrs=%v", span.Attributes())
	}
	if gotURI != uri {
		t.Errorf("mcp.resource.uri = %q, want %q", gotURI, uri)
	}
}

// TestMCPSpan_ToolNameInBoundedName asserts that tool span names keep the tool
// name in the span name (tool names come from a fixed compiled-in registry, so
// they are bounded — unlike resource URIs).
func TestMCPSpan_ToolNameInBoundedName(t *testing.T) {
	rec := recordSpans(t)

	handler := func(_ context.Context, _ map[string]any) (*ToolResult, error) {
		return textResult("ok"), nil
	}
	if _, err := wrapToolHandler(handler)(context.Background(), callToolRequest("author", nil)); err != nil {
		t.Fatalf("handler: %v", err)
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	if got := spans[0].Name(); got != "mcp.tool/author" {
		t.Errorf("span name = %q, want %q", got, "mcp.tool/author")
	}
}
