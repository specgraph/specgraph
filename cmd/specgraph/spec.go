// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
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

func runCreate(_ *cobra.Command, args []string) error {
	client, err := specClient()
	if err != nil {
		return err
	}
	resp, err := client.CreateSpec(context.Background(), connect.NewRequest(&specv1.CreateSpecRequest{
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

	resp, err := client.UpdateSpec(context.Background(), connect.NewRequest(req))
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
	resp, err := client.ListSpecs(context.Background(), connect.NewRequest(&specv1.ListSpecsRequest{
		Stage:    listStage,
		Priority: listPriority,
	}))
	if err != nil {
		return fmt.Errorf("list specs: %w", err)
	}
	specs := resp.Msg.Specs
	if len(specs) == 0 {
		fmt.Println("No specs found.")
		return nil
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	tw := &tableWriter{w: w}
	tw.println("ID\tPRIORITY\tSTAGE\tSLUG")
	for _, s := range specs {
		tw.printf("%s\t%s\t%s\t%s\n", s.Id, s.Priority, s.Stage, s.Slug)
	}
	if tw.err != nil {
		return tw.err
	}
	return w.Flush()
}

// --- show ---

var showCmd = &cobra.Command{
	Use:   "show <slug>",
	Short: "Show spec details",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

var showFormat string

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
	rootCmd.AddCommand(listCmd)

	showCmd.Flags().StringVar(&showFormat, "format", "text", "output format (text, json)")
	rootCmd.AddCommand(showCmd)
}

func runShow(_ *cobra.Command, args []string) error {
	client, err := specClient()
	if err != nil {
		return err
	}
	resp, err := client.GetSpec(context.Background(), connect.NewRequest(&specv1.GetSpecRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("get spec: %w", err)
	}

	switch showFormat {
	case "text":
		// fall through to text output below
	case "json":
		// handled below
	default:
		return fmt.Errorf("unsupported format %q; valid values: text, json", showFormat)
	}

	if showFormat == "json" {
		marshaler := protojson.MarshalOptions{Multiline: true}
		data, mErr := marshaler.Marshal(resp.Msg)
		if mErr != nil {
			return fmt.Errorf("marshal json: %w", mErr)
		}
		_, err = os.Stdout.Write(data)
		if err != nil {
			return err
		}
		fmt.Println()
		return nil
	}

	s := resp.Msg.GetSpec()
	fmt.Printf("ID:         %s\n", s.Id)
	fmt.Printf("Slug:       %s\n", s.Slug)
	fmt.Printf("Intent:     %s\n", s.Intent)
	fmt.Printf("Stage:      %s\n", s.Stage)
	fmt.Printf("Priority:   %s\n", s.Priority)
	fmt.Printf("Complexity: %s\n", s.Complexity)
	fmt.Printf("Version:    %d\n", s.Version)
	if s.Notes != "" {
		fmt.Printf("Notes:      %s\n", s.Notes)
	}
	return nil
}
