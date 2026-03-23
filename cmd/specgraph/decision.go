// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/spf13/cobra"
)

func decisionClient() (specgraphv1connect.DecisionServiceClient, error) {
	return newClient(specgraphv1connect.NewDecisionServiceClient)
}

// --- decision parent command ---

var decisionCmd = &cobra.Command{
	Use:   "decision",
	Short: "Manage decisions",
}

// --- decision create ---

var decisionCreateCmd = &cobra.Command{
	Use:   "create <slug>",
	Short: "Create a new decision",
	Args:  cobra.ExactArgs(1),
	RunE:  runDecisionCreate,
}

var (
	decisionTitle     string
	decisionText      string
	decisionRationale string
)

func runDecisionCreate(_ *cobra.Command, args []string) error {
	client, err := decisionClient()
	if err != nil {
		return err
	}
	resp, err := client.CreateDecision(context.Background(), connect.NewRequest(&specv1.CreateDecisionRequest{
		Slug:      args[0],
		Title:     decisionTitle,
		Decision:  decisionText,
		Rationale: decisionRationale,
	}))
	if err != nil {
		return fmt.Errorf("create decision: %w", err)
	}
	fmt.Printf("Created: %s (%s)\n", resp.Msg.GetDecision().GetSlug(), resp.Msg.GetDecision().GetId())
	return nil
}

// --- decision list ---

var decisionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List decisions",
	RunE:  runDecisionList,
}

var (
	decisionListStatus string
	decisionListJSON   bool
	decisionShowJSON   bool
)

func runDecisionList(cmd *cobra.Command, _ []string) error {
	client, err := decisionClient()
	if err != nil {
		return err
	}

	var statusFilter specv1.DecisionStatus
	if decisionListStatus != "" {
		val, ok := specv1.DecisionStatus_value[decisionListStatus]
		if !ok {
			return fmt.Errorf("unknown status %q; valid values: proposed, accepted, deprecated, superseded", decisionListStatus)
		}
		statusFilter = specv1.DecisionStatus(val)
	}

	resp, err := client.ListDecisions(context.Background(), connect.NewRequest(&specv1.ListDecisionsRequest{
		Status: statusFilter,
	}))
	if err != nil {
		return fmt.Errorf("list decisions: %w", err)
	}
	if decisionListJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.DecisionList(resp.Msg.Decisions))
	return nil
}

// --- decision show ---

var decisionShowCmd = &cobra.Command{
	Use:   "show <slug>",
	Short: "Show decision details",
	Args:  cobra.ExactArgs(1),
	RunE:  runDecisionShow,
}

func init() {
	rootCmd.AddCommand(decisionCmd)

	decisionCreateCmd.Flags().StringVar(&decisionTitle, "title", "", "decision title (required)")
	decisionCreateCmd.Flags().StringVar(&decisionText, "decision", "", "decision text")
	decisionCreateCmd.Flags().StringVar(&decisionRationale, "rationale", "", "rationale")
	cobra.CheckErr(decisionCreateCmd.MarkFlagRequired("title"))
	decisionCmd.AddCommand(decisionCreateCmd)

	decisionListCmd.Flags().StringVar(&decisionListStatus, "status", "", "filter by status")
	decisionListCmd.Flags().BoolVar(&decisionListJSON, "json", false, "output as JSON")
	decisionCmd.AddCommand(decisionListCmd)

	decisionShowCmd.Flags().BoolVar(&decisionShowJSON, "json", false, "output as JSON")
	decisionCmd.AddCommand(decisionShowCmd)
}

func runDecisionShow(cmd *cobra.Command, args []string) error {
	client, err := decisionClient()
	if err != nil {
		return err
	}
	resp, err := client.GetDecision(context.Background(), connect.NewRequest(&specv1.GetDecisionRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("get decision: %w", err)
	}
	if decisionShowJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Print(render.Decision(resp.Msg.GetDecision()))
	return nil
}
