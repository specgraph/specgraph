// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/spf13/cobra"
)

func specClient() (specgraphv1connect.SpecServiceClient, error) {
	return newClient(specgraphv1connect.NewSpecServiceClient)
}

// --- create ---

var createCmd = &cobra.Command{
	Use:   "create <slug>",
	Short: "Create a new spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runCreate,
}

var (
	createIntent   string
	createPriority string
)

func runCreate(cmd *cobra.Command, args []string) error {
	client, err := specClient()
	if err != nil {
		return err
	}
	resp, err := client.CreateSpec(cmd.Context(), connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:     args[0],
		Intent:   createIntent,
		Priority: createPriority,
	}))
	if err != nil {
		return fmt.Errorf("create spec: %w", err)
	}
	fmt.Printf("Created: %s (%s)\n", resp.Msg.GetSpec().GetSlug(), resp.Msg.GetSpec().GetId())
	return nil
}

// --- update ---

var updateCmd = &cobra.Command{
	Use:   "update <slug>",
	Short: "Update an existing spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runUpdate,
}

var (
	updateIntent     string
	updateStage      string
	updatePriority   string
	updateComplexity string
	updateNotes      string
)

func runUpdate(cmd *cobra.Command, args []string) error {
	client, err := specClient()
	if err != nil {
		return err
	}
	req := &specv1.UpdateSpecRequest{Slug: args[0]}
	if cmd.Flags().Changed("intent") {
		req.Intent = &updateIntent
	}
	if cmd.Flags().Changed("stage") {
		req.Stage = &updateStage
	}
	if cmd.Flags().Changed("priority") {
		req.Priority = &updatePriority
	}
	if cmd.Flags().Changed("complexity") {
		req.Complexity = &updateComplexity
	}
	if cmd.Flags().Changed("notes") {
		req.Notes = &updateNotes
	}

	resp, err := client.UpdateSpec(cmd.Context(), connect.NewRequest(req))
	if err != nil {
		return fmt.Errorf("update spec: %w", err)
	}
	fmt.Printf("Updated: %s (version %d)\n", resp.Msg.GetSpec().GetSlug(), resp.Msg.GetSpec().GetVersion())
	return nil
}

// --- list ---

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List specs",
	RunE:  runList,
}

var (
	listStage    string
	listPriority string
)

func runList(cmd *cobra.Command, _ []string) error {
	client, err := specClient()
	if err != nil {
		return err
	}
	resp, err := client.ListSpecs(cmd.Context(), connect.NewRequest(&specv1.ListSpecsRequest{
		Stage:    listStage,
		Priority: listPriority,
	}))
	if err != nil {
		return fmt.Errorf("list specs: %w", err)
	}
	if listJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.SpecList(resp.Msg.Specs))
	return nil
}

// --- show ---

var showCmd = &cobra.Command{
	Use:   "show <slug>",
	Short: "Show spec details",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

var (
	showJSON bool
	listJSON bool
)

func init() {
	createCmd.Flags().StringVar(&createIntent, "intent", "", "intent for the spec (required)")
	createCmd.Flags().StringVar(&createPriority, "priority", "p2", "priority (p0-p3)")
	cobra.CheckErr(createCmd.MarkFlagRequired("intent"))
	rootCmd.AddCommand(createCmd)

	updateCmd.Flags().StringVar(&updateIntent, "intent", "", "new intent")
	updateCmd.Flags().StringVar(&updateStage, "stage", "", "new stage")
	updateCmd.Flags().StringVar(&updatePriority, "priority", "", "new priority")
	updateCmd.Flags().StringVar(&updateComplexity, "complexity", "", "new complexity")
	updateCmd.Flags().StringVar(&updateNotes, "notes", "", "free-text notes")
	rootCmd.AddCommand(updateCmd)

	listCmd.Flags().StringVar(&listStage, "stage", "", "filter by stage")
	listCmd.Flags().StringVar(&listPriority, "priority", "", "filter by priority")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(listCmd)

	showCmd.Flags().BoolVar(&showJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	client, err := specClient()
	if err != nil {
		return err
	}
	resp, err := client.GetSpec(cmd.Context(), connect.NewRequest(&specv1.GetSpecRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("get spec: %w", err)
	}
	if showJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.Spec(resp.Msg.GetSpec()))
	return nil
}
