// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/specgraph/specgraph/internal/auth"
	mcppkg "github.com/specgraph/specgraph/internal/mcp"
	"github.com/specgraph/specgraph/internal/xdg"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server over stdio (for Claude Code, Cursor, etc.)",
	Long: `Start a Model Context Protocol server that communicates over stdin/stdout.
This lightweight process translates MCP tool calls into ConnectRPC RPCs
against a running specgraph serve instance.

Configure in Claude Code's MCP settings:
  {
    "mcpServers": {
      "specgraph": {
        "command": "specgraph",
        "args": ["mcp", "--profile", "authoring"]
      }
    }
  }`,
	RunE: runMCP,
}

func init() {
	mcpCmd.Flags().String("profile", "core", "Tool profile: core, authoring, or execution")
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(cmd *cobra.Command, _ []string) error {
	profileStr, err := cmd.Flags().GetString("profile")
	if err != nil {
		return fmt.Errorf("profile flag: %w", err)
	}
	switch profileStr {
	case "core", "authoring", "execution":
	default:
		return fmt.Errorf("invalid --profile %q (must be core, authoring, or execution)", profileStr)
	}
	profile := mcppkg.ParseProfile(profileStr)

	baseURL, project, err := resolveBaseURL()
	if err != nil {
		return fmt.Errorf("resolve server: %w", err)
	}

	httpClient := newHTTPClient(project)
	client := mcppkg.NewClient(httpClient, baseURL)
	srv := mcppkg.NewServer(client, mcppkg.WithProfileOverride(profile))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Resolve credential: API key first, then cached OIDC token.
	token := resolveAPIKey()
	if token == "" {
		tokenStore := auth.NewFileTokenStore(xdg.OAuthTokenFile())
		if cached, err := tokenStore.GetToken(ctx); err == nil {
			token = cached.AccessToken
		}
	}
	ctx = auth.WithBearerToken(ctx, token)

	return srv.ServeStdio(ctx, os.Stdin, os.Stdout)
}
