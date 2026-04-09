// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/spf13/cobra"
)

const reportTimeout = 30 * time.Second

// --- report-progress ---

var reportProgressCmd = &cobra.Command{
	Use:   "report-progress <slug>",
	Short: "Report execution progress for a claimed spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runReportProgress,
}

var (
	reportProgressAgent   string
	reportProgressMessage string
)

// --- report-blocker ---

var reportBlockerCmd = &cobra.Command{
	Use:   "report-blocker <slug>",
	Short: "Report a blocker on a claimed spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runReportBlocker,
}

var (
	reportBlockerAgent       string
	reportBlockerDescription string
)

// --- report-completion ---

var reportCompletionCmd = &cobra.Command{
	Use:   "report-completion <slug>",
	Short: "Report completion of a claimed spec (transitions to done)",
	Args:  cobra.ExactArgs(1),
	RunE:  runReportCompletion,
}

var reportCompletionAgent string

func init() {
	reportProgressCmd.Flags().StringVar(&reportProgressAgent, "agent", "", "agent identifier (required)")
	reportProgressCmd.Flags().StringVar(&reportProgressMessage, "message", "", "progress message (required)")
	cobra.CheckErr(reportProgressCmd.MarkFlagRequired("agent"))
	cobra.CheckErr(reportProgressCmd.MarkFlagRequired("message"))
	rootCmd.AddCommand(reportProgressCmd)

	reportBlockerCmd.Flags().StringVar(&reportBlockerAgent, "agent", "", "agent identifier (required)")
	reportBlockerCmd.Flags().StringVar(&reportBlockerDescription, "description", "", "blocker description (required)")
	cobra.CheckErr(reportBlockerCmd.MarkFlagRequired("agent"))
	cobra.CheckErr(reportBlockerCmd.MarkFlagRequired("description"))
	rootCmd.AddCommand(reportBlockerCmd)

	reportCompletionCmd.Flags().StringVar(&reportCompletionAgent, "agent", "", "agent identifier (required)")
	cobra.CheckErr(reportCompletionCmd.MarkFlagRequired("agent"))
	rootCmd.AddCommand(reportCompletionCmd)
}

func runReportProgress(cmd *cobra.Command, args []string) error {
	client, err := executionClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), reportTimeout)
	defer cancel()
	_, err = client.ReportProgress(ctx, connect.NewRequest(&specv1.ReportProgressRequest{
		Slug:    args[0],
		Agent:   reportProgressAgent,
		Message: reportProgressMessage,
	}))
	if err != nil {
		return fmt.Errorf("report progress: %w", err)
	}
	fmt.Printf("Progress reported: %s\n", args[0])
	return nil
}

func runReportBlocker(cmd *cobra.Command, args []string) error {
	client, err := executionClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), reportTimeout)
	defer cancel()
	_, err = client.ReportBlocker(ctx, connect.NewRequest(&specv1.ReportBlockerRequest{
		Slug:        args[0],
		Agent:       reportBlockerAgent,
		Description: reportBlockerDescription,
	}))
	if err != nil {
		return fmt.Errorf("report blocker: %w", err)
	}
	fmt.Printf("Blocker reported: %s\n", args[0])
	return nil
}

func runReportCompletion(cmd *cobra.Command, args []string) error {
	client, err := executionClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), reportTimeout)
	defer cancel()
	resp, err := client.ReportCompletion(ctx, connect.NewRequest(&specv1.ReportCompletionRequest{
		Slug:  args[0],
		Agent: reportCompletionAgent,
	}))
	if err != nil {
		return fmt.Errorf("report completion: %w", err)
	}
	fmt.Printf("Completion reported: %s → %s\n", args[0], resp.Msg.NewStage)
	return nil
}
