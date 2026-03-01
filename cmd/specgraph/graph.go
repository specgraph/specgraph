// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"text/tabwriter"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/spf13/cobra"
)

// --- deps ---

var depsCmd = &cobra.Command{
	Use:   "deps <slug>",
	Short: "Show dependencies of a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runDeps,
}

var depsTransitive bool

func init() {
	depsCmd.Flags().BoolVar(&depsTransitive, "transitive", false, "show transitive dependencies")
	rootCmd.AddCommand(depsCmd)
}

func runDeps(cmd *cobra.Command, args []string) error {
	client, err := graphClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	if depsTransitive {
		resp, tdErr := client.GetTransitiveDeps(ctx, connect.NewRequest(&specv1.GetTransitiveDepsRequest{Slug: args[0]}))
		if tdErr != nil {
			return fmt.Errorf("get transitive deps: %w", tdErr)
		}
		return printNodeRefs(cmd, "DEPENDENCIES (transitive)", resp.Msg.Dependencies)
	}

	resp, err := client.GetDependencies(ctx, connect.NewRequest(&specv1.GetDependenciesRequest{Slug: args[0]}))
	if err != nil {
		return fmt.Errorf("get dependencies: %w", err)
	}
	return printNodeRefs(cmd, "DEPENDENCIES", resp.Msg.Dependencies)
}

// --- ready ---

var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "Show specs ready to work on",
	RunE:  runReady,
}

func init() {
	rootCmd.AddCommand(readyCmd)
}

func runReady(cmd *cobra.Command, _ []string) error {
	client, err := graphClient()
	if err != nil {
		return err
	}
	resp, err := client.GetReady(context.Background(), connect.NewRequest(&specv1.GetReadyRequest{}))
	if err != nil {
		return fmt.Errorf("get ready: %w", err)
	}
	return printNodeRefs(cmd, "READY SPECS", resp.Msg.Ready)
}

// --- critical-path ---

var criticalPathCmd = &cobra.Command{
	Use:   "critical-path <slug>",
	Short: "Show the critical path for a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runCriticalPath,
}

func init() {
	rootCmd.AddCommand(criticalPathCmd)
}

func runCriticalPath(cmd *cobra.Command, args []string) error {
	client, err := graphClient()
	if err != nil {
		return err
	}
	resp, err := client.GetCriticalPath(context.Background(), connect.NewRequest(&specv1.GetCriticalPathRequest{Slug: args[0]}))
	if err != nil {
		return fmt.Errorf("get critical path: %w", err)
	}
	return printNodeRefs(cmd, "CRITICAL PATH", resp.Msg.Path)
}

// --- impact ---

var impactCmd = &cobra.Command{
	Use:   "impact <slug>",
	Short: "Show specs impacted by changes to a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runImpact,
}

func init() {
	rootCmd.AddCommand(impactCmd)
}

func runImpact(cmd *cobra.Command, args []string) error {
	client, err := graphClient()
	if err != nil {
		return err
	}
	resp, err := client.GetImpact(context.Background(), connect.NewRequest(&specv1.GetImpactRequest{Slug: args[0]}))
	if err != nil {
		return fmt.Errorf("get impact: %w", err)
	}
	return printNodeRefs(cmd, "IMPACTED SPECS", resp.Msg.Impacted)
}

// --- helpers ---

func printNodeRefs(cmd *cobra.Command, header string, refs []*specv1.NodeRef) error {
	if len(refs) == 0 {
		fmt.Println("None.")
		return nil
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	tw := &tableWriter{w: w}
	tw.printf("%s:\n", header)
	tw.println("ID\tSLUG\tLABEL\tSTAGE")
	for _, r := range refs {
		tw.printf("%s\t%s\t%s\t%s\n", r.Id, r.Slug, r.Label, r.Stage)
	}
	if tw.err != nil {
		return tw.err
	}
	return w.Flush()
}
