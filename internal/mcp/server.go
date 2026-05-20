// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"log/slog"

	sdkmcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/specgraph/specgraph/internal/mcp/skills"
)

const (
	serverName    = "specgraph"
	serverVersion = "0.1.0"
)

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
		slog.Info("mcp: client initialized",
			"client_name", msg.Params.ClientInfo.Name,
			"client_version", msg.Params.ClientInfo.Version,
			"profile", profile,
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
		params := fromSDKParams(&req)
		result, err := h(ctx, params)
		if err != nil {
			return nil, err
		}
		return toSDKResult(result), nil
	}
}

// wrapResourceHandler adapts a SpecGraph ResourceHandler to the mcp-go ResourceHandlerFunc.
func wrapResourceHandler(h ResourceHandler) server.ResourceHandlerFunc {
	return func(ctx context.Context, req sdkmcp.ReadResourceRequest) ([]sdkmcp.ResourceContents, error) {
		contents, err := h(ctx, req.Params.URI)
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
		contents, err := h(ctx, req.Params.URI)
		if err != nil {
			return nil, err
		}
		return toSDKResourceContents(contents), nil
	}
}

// wrapPromptHandler adapts a SpecGraph PromptHandler to the mcp-go PromptHandlerFunc.
func wrapPromptHandler(h PromptHandler) server.PromptHandlerFunc {
	return func(ctx context.Context, req sdkmcp.GetPromptRequest) (*sdkmcp.GetPromptResult, error) {
		result, err := h(ctx, req.Params.Arguments)
		if err != nil {
			return nil, err
		}
		return toSDKPromptResult(result), nil
	}
}
