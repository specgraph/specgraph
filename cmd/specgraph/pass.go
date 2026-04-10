// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render/markdown"
	"github.com/spf13/cobra"
)

var passCmd = &cobra.Command{
	Use:   "pass",
	Short: "Manage analytical passes",
}

var passRunCmd = &cobra.Command{
	Use:   "run <slug>",
	Short: "Run an analytical pass and return the prompt template",
	Long:  "Calls RunAnalyticalPass RPC and returns the prompt template, tool manifest, and instructions. Default output is markdown optimized for LLM consumption.",
	Args:  cobra.ExactArgs(1),
	RunE:  runPassRun,
}

var (
	passRunPassType string
	passRunJSON     bool
)

func init() {
	passRunCmd.Flags().StringVar(&passRunPassType, "pass-type", "", "pass type (constitution-check, red-team, peripheral-vision, consistency, simplicity) (required)")
	passRunCmd.Flags().BoolVar(&passRunJSON, "json", false, "output as JSON")
	passCmd.AddCommand(passRunCmd)
	rootCmd.AddCommand(passCmd)
}

func runPassRun(cmd *cobra.Command, args []string) error {
	if passRunPassType == "" {
		return fmt.Errorf("--pass-type is required")
	}
	pt, ok := passTypeMap[passRunPassType]
	if !ok {
		return fmt.Errorf("unknown pass type %q; valid: constitution-check, red-team, peripheral-vision, consistency, simplicity", passRunPassType)
	}

	client, err := analyticalPassClient()
	if err != nil {
		return err
	}

	slug := args[0]
	resp, err := client.RunAnalyticalPass(cmd.Context(), connect.NewRequest(&specv1.RunAnalyticalPassRequest{
		Slug:     slug,
		PassType: pt,
	}))
	if err != nil {
		return fmt.Errorf("run analytical pass: %w", err)
	}

	if passRunJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	_, err = fmt.Fprint(cmd.OutOrStdout(), markdown.AnalyticalPass(resp.Msg, slug))
	return err
}
