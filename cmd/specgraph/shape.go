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

var shapeCmd = &cobra.Command{
	Use:   "shape <slug>",
	Short: "Bound scope, explore solutions, and surface risks",
	Args:  cobra.ExactArgs(1),
	RunE:  runShape,
}

func init() {
	rootCmd.AddCommand(shapeCmd)
}

func runShape(_ *cobra.Command, args []string) error {
	client, err := authoringClient()
	if err != nil {
		return err
	}
	resp, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug:   args[0],
		Output: &specv1.ShapeOutput{},
	}))
	if err != nil {
		return fmt.Errorf("shape: %w", err)
	}
	fmt.Printf("Shaped: %s\n", args[0])
	for _, f := range resp.Msg.SafetyFlags {
		fmt.Printf("  [%s] %s: %s\n", f.Severity, f.Category, f.Description)
	}
	return nil
}
