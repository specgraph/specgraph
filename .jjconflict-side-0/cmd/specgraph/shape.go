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
	Short: "Bound scope, explore solutions, and surface risks (use --json-file to supply output)",
	Args:  cobra.ExactArgs(1),
	RunE:  runShape,
}

var shapeJSONFile string

func init() {
	shapeCmd.Flags().StringVar(&shapeJSONFile, "json-file", "", "path to JSON file containing ShapeOutput")
	rootCmd.AddCommand(shapeCmd)
}

func runShape(_ *cobra.Command, args []string) error {
	client, err := authoringClient()
	if err != nil {
		return err
	}
	output := &specv1.ShapeOutput{}
	if shapeJSONFile != "" {
		if loadErr := loadJSONFile(shapeJSONFile, output); loadErr != nil {
			return fmt.Errorf("shape: %w", loadErr)
		}
	}
	resp, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug:   args[0],
		Output: output,
	}))
	if err != nil {
		return fmt.Errorf("shape: %w", err)
	}
	fmt.Printf("Shaped: %s\n", args[0])
	printSafetyFlags(resp.Msg.SafetyFlags)
	return nil
}
