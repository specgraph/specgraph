// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
)

func injectClient() (specgraphv1connect.SyncServiceClient, error) {
	return newClient(specgraphv1connect.NewSyncServiceClient)
}

var injectCmd = &cobra.Command{
	Use:   "inject <slug>",
	Short: "Write spec context into workspace for a coding tool",
	Long:  "Inject spec execution context (bundle + constitution subset) into tool-specific workspace files.",
	Args:  cobra.ExactArgs(1),
	RunE:  runInject,
}

var (
	injectTool   string
	injectOutput string
)

func runInject(_ *cobra.Command, args []string) error {
	client, err := injectClient()
	if err != nil {
		return err
	}

	var tool specv1.InjectTool
	switch strings.ToLower(injectTool) {
	case "claude-code", "claude":
		tool = specv1.InjectTool_INJECT_TOOL_CLAUDE_CODE
	case "cursor":
		tool = specv1.InjectTool_INJECT_TOOL_CURSOR
	case "agents-md", "agents":
		tool = specv1.InjectTool_INJECT_TOOL_AGENTS_MD
	default:
		return fmt.Errorf("unsupported tool: %s (supported: claude-code, cursor, agents-md)", injectTool)
	}

	resp, err := client.Inject(context.Background(), connect.NewRequest(&specv1.InjectRequest{
		SpecSlug:  args[0],
		Tool:      tool,
		OutputDir: injectOutput,
	}))
	if err != nil {
		return fmt.Errorf("inject: %w", err)
	}

	fmt.Println(resp.Msg.Summary)
	for _, f := range resp.Msg.FilesWritten {
		fmt.Printf("  -> %s\n", f)
	}
	return nil
}

func init() {
	injectCmd.Flags().StringVar(&injectTool, "tool", "claude-code", "target tool (claude-code, cursor, agents-md)")
	injectCmd.Flags().StringVarP(&injectOutput, "output", "o", "", "output directory (default: current directory)")
	rootCmd.AddCommand(injectCmd)
}
