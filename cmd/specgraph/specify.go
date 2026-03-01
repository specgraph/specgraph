// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/spf13/cobra"
)

var specifyCmd = &cobra.Command{
	Use:   "specify <slug>",
	Short: "Define interface contract, verification criteria, and invariants",
	Args:  cobra.ExactArgs(1),
	RunE:  runSpecify,
}

func init() {
	rootCmd.AddCommand(specifyCmd)
}

func runSpecify(_ *cobra.Command, args []string) error {
	client, err := authoringClient()
	if err != nil {
		return err
	}
	resp, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug:   args[0],
		Output: &specv1.SpecifyOutput{},
	}))
	if err != nil {
		return fmt.Errorf("specify: %w", err)
	}
	fmt.Printf("Specified: %s\n", args[0])
	for _, f := range resp.Msg.SafetyFlags {
		fmt.Printf("  [%s] %s: %s\n", f.Severity, f.Category, f.Description)
	}
	return nil
}
