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

var approveCmd = &cobra.Command{
	Use:   "approve <slug>",
	Short: "Mark a spec as approved and ready for execution",
	Args:  cobra.ExactArgs(1),
	RunE:  runApprove,
}

func init() {
	rootCmd.AddCommand(approveCmd)
}

func runApprove(cmd *cobra.Command, args []string) error {
	client, err := authoringClient()
	if err != nil {
		return err
	}
	resp, err := client.Approve(cmd.Context(), connect.NewRequest(&specv1.ApproveRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("approve: %w", err)
	}
	fmt.Printf("Approved: %s at %s\n", resp.Msg.Slug,
		resp.Msg.ApprovedAt.AsTime().Format(time.RFC3339))
	return nil
}
