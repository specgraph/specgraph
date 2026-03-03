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

var decomposeCmd = &cobra.Command{
	Use:   "decompose <slug>",
	Short: "Break a spec into work units (use --json-file to supply output)",
	Args:  cobra.ExactArgs(1),
	RunE:  runDecompose,
}

var decomposeJSONFile string

func init() {
	decomposeCmd.Flags().StringVar(&decomposeJSONFile, "json-file", "", "path to JSON file containing DecomposeOutput")
	rootCmd.AddCommand(decomposeCmd)
}

func runDecompose(_ *cobra.Command, args []string) error {
	client, err := authoringClient()
	if err != nil {
		return err
	}
	output := &specv1.DecomposeOutput{}
	if decomposeJSONFile != "" {
		data, readErr := os.ReadFile(decomposeJSONFile)
		if readErr != nil {
			return fmt.Errorf("decompose: read json-file: %w", readErr)
		}
		if unmarshalErr := protojson.Unmarshal(data, output); unmarshalErr != nil {
			return fmt.Errorf("decompose: parse json-file: %w", unmarshalErr)
		}
	}
	resp, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug:   args[0],
		Output: output,
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
