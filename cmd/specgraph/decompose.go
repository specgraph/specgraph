// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/spf13/cobra"
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

func runDecompose(cmd *cobra.Command, args []string) error {
	client, err := authoringClient()
	if err != nil {
		return err
	}
	output := &specv1.DecomposeOutput{}
	if decomposeJSONFile != "" {
		if loadErr := loadJSONFile(decomposeJSONFile, output); loadErr != nil {
			return fmt.Errorf("decompose: %w", loadErr)
		}
	}
	resp, err := client.Decompose(cmd.Context(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug:   args[0],
		Output: output,
	}))
	if err != nil {
		return fmt.Errorf("decompose: %w", err)
	}
	fmt.Printf("Decomposed: %s\n", args[0])
	if resp.Msg.Output != nil {
		fmt.Printf("Strategy: %s\n", resp.Msg.Output.Strategy)
	}
	if len(resp.Msg.SliceSlugs) > 0 {
		fmt.Printf("Slices (%d):\n", len(resp.Msg.SliceSlugs))
		for _, slug := range resp.Msg.SliceSlugs {
			fmt.Printf("  - %s\n", slug)
		}
	}
	return nil
}
