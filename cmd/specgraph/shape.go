// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
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
		data, readErr := os.ReadFile(shapeJSONFile)
		if readErr != nil {
			return fmt.Errorf("shape: read json-file: %w", readErr)
		}
		if unmarshalErr := protojson.Unmarshal(data, output); unmarshalErr != nil {
			return fmt.Errorf("shape: parse json-file: %w", unmarshalErr)
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
	for _, f := range resp.Msg.SafetyFlags {
		fmt.Printf("  [%s] %s: %s\n", f.Severity, f.Category, f.Description)
	}
	return nil
}
