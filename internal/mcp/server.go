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

// Server wraps one MCPServer per tier. Each tier's MCPServer is pre-populated
// with the tools, resources, and prompts appropriate for that tier.
type Server struct {
	registry *Registry
	servers  map[Tier]*server.MCPServer
}

// NewServer creates a fully wired MCP server backed by the given ConnectRPC client.
// It registers all tools, resources, and prompts, then builds one MCPServer
// per tier with the appropriate subset of tools (all tiers get all resources
// and prompts).
func NewServer(client *Client) *Server {
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

	tiers := []Tier{TierCore, TierAuthoring, TierExecution}
	servers := make(map[Tier]*server.MCPServer, len(tiers))

	for _, tier := range tiers {
		srv := server.NewMCPServer(
			serverName,
			serverVersion,
			server.WithToolCapabilities(false),
			server.WithResourceCapabilities(false, false),
			server.WithPromptCapabilities(false),
		)

		// Add tier-appropriate tools.
		for _, td := range reg.ToolsForTier(tier) {
			srv.AddTool(toSDKTool(td), wrapToolHandler(td.Handler))
		}

		// All tiers get all resources.
		for i := range reg.Resources() {
			rd := &reg.resources[i]
			if rd.IsTemplate {
				srv.AddResourceTemplate(toSDKResourceTemplate(rd), wrapResourceTemplateHandler(rd.Handler))
			} else {
				srv.AddResource(toSDKResource(rd), wrapResourceHandler(rd.Handler))
			}
		}

		// All tiers get all prompts.
		for _, pd := range reg.Prompts() {
			srv.AddPrompt(toSDKPrompt(pd), wrapPromptHandler(pd.Handler))
		}

		servers[tier] = srv
	}

	return &Server{
		registry: reg,
		servers:  servers,
	}
}

// ForTier returns the MCPServer for the given tier. Unknown tiers fall back to TierCore.
func (s *Server) ForTier(tier Tier) *server.MCPServer {
	srv, ok := s.servers[tier]
	if !ok {
		return s.servers[TierCore]
	}
	return srv
}

// ServeStdio runs a stdio transport for the given tier, reading JSON-RPC
// messages from stdin and writing responses to stdout. It blocks until ctx
// is cancelled or an error occurs.
func (s *Server) ServeStdio(ctx context.Context, tier Tier, stdin io.Reader, stdout io.Writer) error {
	stdio := server.NewStdioServer(s.ForTier(tier))
	if err := stdio.Listen(ctx, stdin, stdout); err != nil {
		return fmt.Errorf("mcp stdio: %w", err)
	}
	return nil
}

// HTTPHandler returns a StreamableHTTPServer for the given tier, suitable
// for use as an http.Handler.
func (s *Server) HTTPHandler(tier Tier) *server.StreamableHTTPServer {
	return server.NewStreamableHTTPServer(s.ForTier(tier))
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
