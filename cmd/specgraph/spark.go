// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/spf13/cobra"
)

var sparkCmd = &cobra.Command{
	Use:   "spark <slug>",
	Short: "Capture an idea and enter the authoring funnel",
	Args:  cobra.ExactArgs(1),
	RunE:  runSpark,
}

var sparkSeed string

func init() {
	sparkCmd.Flags().StringVar(&sparkSeed, "seed", "", "seed idea (one sentence)")
	rootCmd.AddCommand(sparkCmd)
}

func runSpark(cmd *cobra.Command, args []string) error {
	client, err := authoringClient()
	if err != nil {
		return err
	}
	resp, err := client.Spark(cmd.Context(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   args[0],
		Output: &specv1.SparkOutput{Seed: sparkSeed},
	}))
	if err != nil {
		return fmt.Errorf("spark: %w", err)
	}
	fmt.Printf("Sparked: %s\n", args[0])
	if resp.Msg.Output != nil && resp.Msg.Output.Seed != "" {
		fmt.Printf("Seed: %s\n", resp.Msg.Output.Seed)
	}
	printSafetyFlags(resp.Msg.SafetyFlags)
	if len(resp.Msg.NextPrompts) > 0 {
		fmt.Println("\nNext stage prompts (shape):")
		for _, p := range resp.Msg.NextPrompts {
			fmt.Printf("  - %s: %s\n", p.Name, p.Template)
		}
	}
	return nil
}
