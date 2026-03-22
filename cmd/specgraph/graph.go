// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/spf13/cobra"
)

// --- deps ---

var depsCmd = &cobra.Command{
	Use:   "deps <slug>",
	Short: "Show dependencies of a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runDeps,
}

var (
	depsTransitive  bool
	depsJSON        bool
	readyJSON       bool
	criticalPathJSON bool
	impactJSON      bool
)

func init() {
	depsCmd.Flags().BoolVar(&depsTransitive, "transitive", false, "show transitive dependencies")
	depsCmd.Flags().BoolVar(&depsJSON, "json", false, "output as JSON")
	readyCmd.Flags().BoolVar(&readyJSON, "json", false, "output as JSON")
	criticalPathCmd.Flags().BoolVar(&criticalPathJSON, "json", false, "output as JSON")
	impactCmd.Flags().BoolVar(&impactJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(depsCmd)
	rootCmd.AddCommand(readyCmd)
	rootCmd.AddCommand(criticalPathCmd)
	rootCmd.AddCommand(impactCmd)
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
		if depsJSON {
			return printJSON(cmd.OutOrStdout(), resp.Msg)
		}
		fmt.Print(render.NodeRefList("Dependencies (transitive)", resp.Msg.Dependencies))
		return nil
	}

	resp, err := client.GetDependencies(ctx, connect.NewRequest(&specv1.GetDependenciesRequest{Slug: args[0]}))
	if err != nil {
		return fmt.Errorf("get dependencies: %w", err)
	}
	if depsJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.NodeRefList("Dependencies", resp.Msg.Dependencies))
	return nil
}

// --- ready ---

var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "Show specs ready to work on",
	RunE:  runReady,
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
	if readyJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.NodeRefList("Ready Specs", resp.Msg.Ready))
	return nil
}

// --- critical-path ---

var criticalPathCmd = &cobra.Command{
	Use:   "critical-path <slug>",
	Short: "Show the critical path for a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runCriticalPath,
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
	if criticalPathJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.NodeRefList("Critical Path", resp.Msg.Path))
	return nil
}

// --- impact ---

var impactCmd = &cobra.Command{
	Use:   "impact <slug>",
	Short: "Show specs impacted by changes to a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runImpact,
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
	if impactJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.NodeRefList("Impacted Specs", resp.Msg.Impacted))
	return nil
}
