// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
)

func executionClient() (specgraphv1connect.ExecutionServiceClient, error) {
	return newClient(specgraphv1connect.NewExecutionServiceClient)
}

var bundleCmd = &cobra.Command{
	Use:   "bundle <slug>",
	Short: "Generate an execution bundle for a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runBundle,
}

var (
	bundleOutput   string
	bundleEndpoint string
)

func init() {
	bundleCmd.Flags().StringVar(&bundleOutput, "output", "", "write YAML to file instead of stdout")
	bundleCmd.Flags().StringVar(&bundleEndpoint, "endpoint", "", "override callback endpoint")
	rootCmd.AddCommand(bundleCmd)
}

func runBundle(_ *cobra.Command, args []string) error {
	client, err := executionClient()
	if err != nil {
		return err
	}
	resp, err := client.GenerateBundle(context.Background(), connect.NewRequest(&specv1.GenerateBundleRequest{
		Slug:     args[0],
		Endpoint: bundleEndpoint,
	}))
	if err != nil {
		return fmt.Errorf("generate bundle: %w", err)
	}
	yaml := resp.Msg.GetBundleYaml()
	if bundleOutput != "" {
		if err := os.WriteFile(bundleOutput, []byte(yaml), 0o600); err != nil {
			return fmt.Errorf("write bundle: %w", err)
		}
		fmt.Printf("Bundle written to %s\n", bundleOutput)
		return nil
	}
	fmt.Print(yaml)
	return nil
}
