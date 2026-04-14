// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"io"

	sdkmcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	serverName    = "specgraph"
	serverVersion = "0.1.0"
)

// ServerOption configures a Server.
type ServerOption func(*serverConfig)

type serverConfig struct {
	profileOverride *Profile
}

// WithProfileOverride sets a fixed profile, overriding clientInfo-based
// derivation in the OnAfterInitialize hook. Used by the stdio transport
// where the operator selects the profile via --profile flag.
func WithProfileOverride(p Profile) ServerOption {
	return func(cfg *serverConfig) {
		cfg.profileOverride = &p
	}
}

// Server wraps a single MCPServer pre-populated with all tools, resources,
// and prompts. Profile-based tool filtering happens per-session via the
// OnAfterInitialize hook.
type Server struct {
	registry        *Registry
	mcpServer       *server.MCPServer
	profileToolSets map[Profile]map[string]server.ServerTool
}

// NewServer creates a fully wired MCP server backed by the given ConnectRPC client.
// It registers all tools, resources, and prompts onto a single MCPServer
// (the execution-profile superset). An OnAfterInitialize hook narrows
// session-visible tools based on the client's profile.
func NewServer(client *Client, opts ...ServerOption) *Server {
	var cfg serverConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	reg := NewRegistry()

	// Register all handlers into the registry.
	RegisterSpecTools(reg, client)
	RegisterGraphTools(reg, client)
	RegisterCoreTools(reg, client)
	RegisterAuthoringTools(reg, client)
	RegisterLifecycleTools(reg, client)
	RegisterExecutionTools(reg, client)
	RegisterResources(reg, client)
	RegisterPrompts(reg, client)

	hooks := &server.Hooks{}

	srv := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(false, false),
		server.WithPromptCapabilities(false),
		server.WithHooks(hooks),
	)

	// Build per-profile tool sets for session filtering.
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

	// Register tools on the MCPServer. When a profile override is set
	// (stdio transport), register only that profile's tools because
	// stdioSession does not implement SessionWithTools — the
	// OnAfterInitialize hook cannot narrow the tool set per session.
	// For HTTP (no override), register all tools and let the hook filter.
	registerProfile := ProfileExecution
	if cfg.profileOverride != nil {
		registerProfile = *cfg.profileOverride
	}
	for _, td := range reg.ToolsForProfile(registerProfile) {
		srv.AddTool(toSDKTool(td), wrapToolHandler(td.Handler))
	}

	// All resources and prompts (profile-independent).
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

	// Hook: after MCP initialize, narrow tools to the client's profile.
	hooks.AddAfterInitialize(func(ctx context.Context, _ any, msg *sdkmcp.InitializeRequest, _ *sdkmcp.InitializeResult) {
		var profile Profile
		if cfg.profileOverride != nil {
			profile = *cfg.profileOverride
		} else {
			profile = ProfileFromClientInfo(&msg.Params.ClientInfo)
		}
		if profile == ProfileExecution {
			return // full access, no filtering needed
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

// ServeStdio runs a stdio transport. It blocks until ctx is cancelled.
func (s *Server) ServeStdio(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	stdio := server.NewStdioServer(s.mcpServer)
	if err := stdio.Listen(ctx, stdin, stdout); err != nil {
		return fmt.Errorf("mcp stdio: %w", err)
	}
	return nil
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
