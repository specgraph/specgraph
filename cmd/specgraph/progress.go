// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/spf13/cobra"
)

var progressCmd = &cobra.Command{
	Use:   "progress <slug>",
	Short: "Show execution events for a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runProgress,
}

var progressLimit uint32

func init() {
	progressCmd.Flags().Uint32Var(&progressLimit, "limit", 20, "max events to show")
	rootCmd.AddCommand(progressCmd)
}

func runProgress(cmd *cobra.Command, args []string) error {
	client, err := executionClient()
	if err != nil {
		return err
	}
	resp, err := client.GetExecutionEvents(cmd.Context(), connect.NewRequest(&specv1.GetExecutionEventsRequest{
		Slug:  args[0],
		Limit: progressLimit,
	}))
	if err != nil {
		return fmt.Errorf("get execution events: %w", err)
	}
	events := resp.Msg.GetEvents()
	if len(events) == 0 {
		fmt.Println("No execution events found.")
		return nil
	}
	for _, evt := range events {
		fmt.Printf("[%s] %s  %s  %s\n",
			evt.GetType().String(),
			evt.GetSpecSlug(),
			evt.GetAgent(),
			evt.GetCreatedAt().AsTime().Format(time.RFC3339))
		if msg := evt.GetMessage(); msg != "" {
			fmt.Printf("  %s\n", msg)
		}
	}
	return nil
}
