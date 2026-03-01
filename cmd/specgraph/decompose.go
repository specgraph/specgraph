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

var decomposeCmd = &cobra.Command{
	Use:   "decompose <slug>",
	Short: "Break a spec into work units",
	Args:  cobra.ExactArgs(1),
	RunE:  runDecompose,
}

func init() {
	rootCmd.AddCommand(decomposeCmd)
}

func runDecompose(_ *cobra.Command, args []string) error {
	client, err := authoringClient()
	if err != nil {
		return err
	}
	resp, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug:   args[0],
		Output: &specv1.DecomposeOutput{},
	}))
	if err != nil {
		return fmt.Errorf("decompose: %w", err)
	}
	fmt.Printf("Decomposed: %s\n", args[0])
	if resp.Msg.Output != nil {
		fmt.Printf("Strategy: %s\n", resp.Msg.Output.Strategy)
		for _, s := range resp.Msg.Output.Slices {
			fmt.Printf("  - %s: %s\n", s.Id, s.Intent)
		}
	}
	return nil
}
