// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"log/slog"

	sdkmcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/specgraph/specgraph/internal/mcp/skills"
)

const (
	serverName    = "specgraph"
	serverVersion = "0.1.0"
)

// mcpSpan starts a span with the given low-cardinality name and optional
// attributes, returning the child ctx plus an end func that records the
// outcome. High-cardinality identifiers (e.g. resource URIs) MUST be passed as
// attributes, not embedded in the span name, to keep span-name cardinality
// bounded. It uses the OTel global tracer, which is the built-in no-op when
// telemetry is disabled, so this is always safe to call regardless of
// telemetry configuration.
func mcpSpan(ctx context.Context, spanName string, attrs ...attribute.KeyValue) (spanCtx context.Context, end func(err error)) {
	ctx, span := otel.Tracer("specgraph.mcp").Start(ctx, spanName, trace.WithAttributes(attrs...))
	return ctx, func(err error) {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
}

// Server wraps a single MCPServer pre-populated with all tools, resources,
// and prompts. Profile-based tool filtering happens per-session via the
// OnAfterInitialize hook based on the client's reported clientInfo.
type Server struct {
	registry        *Registry
	mcpServer       *server.MCPServer
	profileToolSets map[Profile]map[string]server.ServerTool
}

// NewServer creates a fully wired MCP server backed by the given ConnectRPC client.
// It registers all tools, resources, and prompts onto a single MCPServer
// (the execution-profile superset). An OnAfterInitialize hook narrows
// session-visible tools based on the client's profile.
func NewServer(client *Client) *Server {
	reg := NewRegistry()

	skillsSrc, err := skills.NewEmbedded()
	if err != nil {
		// The embedded catalog is compiled into the binary; a parse failure
		// means the binary itself is broken. Panic immediately so CI catches it.
		panic(fmt.Sprintf("load embedded skills: %v", err))
	}

	RegisterSpecTools(reg, client)
	RegisterGraphTools(reg, client)
	RegisterCoreTools(reg, client)
	RegisterAuthoringTools(reg, client)
	RegisterLifecycleTools(reg, client)
	RegisterExecutionTools(reg, client)
	RegisterResources(reg, client, skillsSrc)
	RegisterPrompts(reg, client)
	RegisterSkillTools(reg, skillsSrc)

	hooks := &server.Hooks{}

	srv := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(false, false),
		server.WithPromptCapabilities(false),
		server.WithHooks(hooks),
	)

	profileToolSets := make(map[Profile]map[string]server.ServerTool, 3)
	for _, profile := range []Profile{ProfileCore, ProfileAuthoring, ProfileExecution} {
		toolMap := make(map[string]server.ServerTool)
		for _, td := range reg.ToolsForProfile(profile) {
			toolMap[td.Name] = server.ServerTool{
				Tool:    toSDKTool(td),
				Handler: wrapToolHandler(td.Handler),
			}
		}
		profileToolSets[profile] = toolMap
	}

	for _, td := range reg.ToolsForProfile(ProfileExecution) {
		srv.AddTool(toSDKTool(td), wrapToolHandler(td.Handler))
	}

	for i := range reg.Resources() {
		rd := &reg.resources[i]
		if rd.IsTemplate {
			srv.AddResourceTemplate(toSDKResourceTemplate(rd), wrapResourceTemplateHandler(rd.Handler))
		} else {
			srv.AddResource(toSDKResource(rd), wrapResourceHandler(rd.Handler))
		}
	}
	for _, pd := range reg.Prompts() {
		srv.AddPrompt(toSDKPrompt(pd), wrapPromptHandler(pd.Handler))
	}

	s := &Server{
		registry:        reg,
		mcpServer:       srv,
		profileToolSets: profileToolSets,
	}

	hooks.AddAfterInitialize(func(ctx context.Context, _ any, msg *sdkmcp.InitializeRequest, _ *sdkmcp.InitializeResult) {
		profile := ProfileFromClientInfo(&msg.Params.ClientInfo)
		slog.LogAttrs(ctx, slog.LevelInfo, "mcp: client initialized",
			slog.String("client_name", msg.Params.ClientInfo.Name),
			slog.String("client_version", msg.Params.ClientInfo.Version),
			slog.Any("profile", profile),
		)
		if profile == ProfileExecution {
			return
		}
		session := server.ClientSessionFromContext(ctx)
		if swt, ok := session.(server.SessionWithTools); ok {
			swt.SetSessionTools(s.profileToolSets[profile])
		}
	})

	return s
}

// MCPServer returns the underlying mcp-go server.
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcpServer
}

// ToolsForProfile returns the pre-built ServerTool map for the given profile.
// Unknown profiles fall back to ProfileCore.
func (s *Server) ToolsForProfile(profile Profile) map[string]server.ServerTool {
	tools, ok := s.profileToolSets[profile]
	if !ok {
		return s.profileToolSets[ProfileCore]
	}
	return tools
}

// HTTPHandler returns a StreamableHTTPServer suitable for use as an http.Handler.
func (s *Server) HTTPHandler(opts ...server.StreamableHTTPOption) *server.StreamableHTTPServer {
	return server.NewStreamableHTTPServer(s.mcpServer, opts...)
}

// wrapToolHandler adapts a SpecGraph ToolHandler to the mcp-go ToolHandlerFunc signature.
func wrapToolHandler(h ToolHandler) server.ToolHandlerFunc {
	return func(ctx context.Context, req sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
		ctx, end := mcpSpan(ctx, "mcp.tool/"+req.Params.Name)
		params := fromSDKParams(&req)
		result, err := h(ctx, params)
		end(err)
		if err != nil {
			return nil, err
		}
		return toSDKResult(result), nil
	}
}

// wrapResourceHandler adapts a SpecGraph ResourceHandler to the mcp-go ResourceHandlerFunc.
func wrapResourceHandler(h ResourceHandler) server.ResourceHandlerFunc {
	return func(ctx context.Context, req sdkmcp.ReadResourceRequest) ([]sdkmcp.ResourceContents, error) {
		ctx, end := mcpSpan(ctx, "mcp.resource", attribute.String("mcp.resource.uri", req.Params.URI))
		contents, err := h(ctx, req.Params.URI)
		end(err)
		if err != nil {
			return nil, err
		}
		return toSDKResourceContents(contents), nil
	}
}

// wrapResourceTemplateHandler adapts a SpecGraph ResourceHandler to the mcp-go
// ResourceTemplateHandlerFunc (same signature as ResourceHandlerFunc).
func wrapResourceTemplateHandler(h ResourceHandler) server.ResourceTemplateHandlerFunc {
	return func(ctx context.Context, req sdkmcp.ReadResourceRequest) ([]sdkmcp.ResourceContents, error) {
		ctx, end := mcpSpan(ctx, "mcp.resource_template", attribute.String("mcp.resource_template.uri", req.Params.URI))
		contents, err := h(ctx, req.Params.URI)
		end(err)
		if err != nil {
			return nil, err
		}
		return toSDKResourceContents(contents), nil
	}
}

// wrapPromptHandler adapts a SpecGraph PromptHandler to the mcp-go PromptHandlerFunc.
func wrapPromptHandler(h PromptHandler) server.PromptHandlerFunc {
	return func(ctx context.Context, req sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		ctx, end := mcpSpan(ctx, "mcp.prompt/"+req.Params.Name)
		result, err := h(ctx, req.Params.Arguments)
		end(err)
		if err != nil {
			return nil, err
		}
		return toSDKPromptResult(result), nil
	}
}
