// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	mcppkg "github.com/specgraph/specgraph/internal/mcp"
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
        "args": ["mcp", "--tier", "authoring"]
      }
    }
  }`,
	RunE: runMCP,
}

func init() {
	mcpCmd.Flags().String("tier", "core", "Tool tier: core, authoring, or execution")
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(cmd *cobra.Command, _ []string) error {
	tierStr, _ := cmd.Flags().GetString("tier")
	tier := mcppkg.ParseTier(tierStr)

	baseURL, project, err := resolveBaseURL()
	if err != nil {
		return fmt.Errorf("resolve server: %w", err)
	}

	httpClient := newHTTPClient(project)
	client := mcppkg.NewClient(httpClient, baseURL)
	srv := mcppkg.NewServer(client)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	return srv.ServeStdio(ctx, tier, os.Stdin, os.Stdout)
}
