// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check server health",
	RunE:  runHealth,
}

func init() {
	rootCmd.AddCommand(healthCmd)
}

func healthClient() (specgraphv1connect.ServerServiceClient, error) {
	return newClient(specgraphv1connect.NewServerServiceClient)
}

func runHealth(_ *cobra.Command, _ []string) error {
	client, err := healthClient()
	if err != nil {
		return err
	}
	resp, err := client.Health(context.Background(), connect.NewRequest(&specv1.HealthRequest{}))
	if err != nil {
		return fmt.Errorf("health check: %w", err)
	}
	fmt.Printf("Status:  %s\n", resp.Msg.Status)
	fmt.Printf("Version: %s\n", resp.Msg.Version)
	return nil
}
