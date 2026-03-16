// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"text/tabwriter"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/config"
	"github.com/seanb4t/specgraph/internal/xdg"
	"github.com/spf13/cobra"
)

var primeCmd = &cobra.Command{
	Use:   "prime",
	Short: "Orient Claude Code to the current project",
	Long:  "Ensure the server is running, then print project context and active specs for use by Claude Code's SessionStart hook.",
	RunE:  runPrime,
}

func init() {
	rootCmd.AddCommand(primeCmd)
}

func runPrime(cmd *cobra.Command, args []string) error {
	// 1. Ensure server is running (idempotent).
	if err := runUp(cmd, args); err != nil {
		// Non-fatal: server may already be running via manual mode.
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: up: %v\n", err)
	}

	// 2. Load project config from CWD.
	project, err := config.LoadProject(".")
	if err != nil {
		return fmt.Errorf("load project config: %w", err)
	}

	// 3. Load global config.
	cfg, err := config.LoadGlobal(xdg.ConfigFile())
	if err != nil {
		return fmt.Errorf("load global config: %w", err)
	}

	// 4. Resolve server URL.
	serverURL := cfg.ResolveServer(project.Slug, project.Server)

	// 5. Print orientation header.
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Project: %s\n", project.Slug)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Server:  %s\n", serverURL)

	// 6. List non-terminal specs.
	client, err := specClient()
	if err != nil {
		return fmt.Errorf("create spec client: %w", err)
	}
	resp, err := client.ListSpecs(context.Background(), connect.NewRequest(&specv1.ListSpecsRequest{}))
	if err != nil {
		return fmt.Errorf("list specs: %w", err)
	}

	var active []*specv1.Spec
	for _, s := range resp.Msg.Specs {
		if s.Stage != "done" {
			active = append(active, s)
		}
	}

	if len(active) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nNo active specs.")
		return nil
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout())
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	tw := &tableWriter{w: w}
	tw.println("SLUG\tSTAGE\tPRIORITY")
	for _, s := range active {
		tw.printf("%s\t%s\t%s\n", s.Slug, s.Stage, s.Priority)
	}
	if tw.err != nil {
		return tw.err
	}
	return w.Flush()
}
