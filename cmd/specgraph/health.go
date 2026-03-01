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
	baseURL, err := resolveBaseURL()
	if err != nil {
		return nil, err
	}
	return specgraphv1connect.NewServerServiceClient(newHTTPClient(), baseURL), nil
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
