// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/spf13/cobra"
)

var specifyCmd = &cobra.Command{
	Use:   "specify <slug>",
	Short: "Define interface contract, verification criteria, and invariants (use --json-file to supply output)",
	Args:  cobra.ExactArgs(1),
	RunE:  runSpecify,
}

var specifyJSONFile string

func init() {
	specifyCmd.Flags().StringVar(&specifyJSONFile, "json-file", "", "path to JSON file containing SpecifyOutput")
	rootCmd.AddCommand(specifyCmd)
}

func runSpecify(cmd *cobra.Command, args []string) error {
	client, err := authoringClient()
	if err != nil {
		return err
	}
	output := &specv1.SpecifyOutput{}
	if specifyJSONFile != "" {
		if loadErr := loadJSONFile(specifyJSONFile, output); loadErr != nil {
			return fmt.Errorf("specify: %w", loadErr)
		}
	}
	resp, err := client.Specify(cmd.Context(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug:   args[0],
		Output: output,
	}))
	if err != nil {
		return fmt.Errorf("specify: %w", err)
	}
	fmt.Printf("Specified: %s\n", args[0])
	printSafetyFlags(resp.Msg.SafetyFlags)
	return nil
}
