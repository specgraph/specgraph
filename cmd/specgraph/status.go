// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
)

var statusJSON bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show server and service health",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(statusCmd)
}

type statusOutput struct {
	Server  string `json:"server"`
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

func runStatus(cmd *cobra.Command, _ []string) error {
	baseURL, _, err := resolveBaseURL()
	if err != nil {
		return err
	}

	client, err := newClient(specgraphv1connect.NewServerServiceClient)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
	defer cancel()

	resp, connErr := client.Health(ctx, connect.NewRequest(&specv1.HealthRequest{}))
	if connErr != nil {
		// Only treat connection/timeout errors as "not running".
		// Other errors (auth, server-side) should propagate.
		var netErr net.Error
		isConnErr := errors.Is(connErr, context.DeadlineExceeded) ||
			errors.Is(connErr, context.Canceled) ||
			errors.As(connErr, &netErr)
		if !isConnErr {
			return fmt.Errorf("health check: %w", connErr)
		}
		out := statusOutput{
			Server: baseURL,
			Status: "not running",
		}
		if statusJSON {
			return json.NewEncoder(os.Stdout).Encode(out)
		}
		fmt.Printf("Server:  %s\n", out.Server)
		fmt.Printf("Status:  %s\n", out.Status)
		return nil
	}

	out := statusOutput{
		Server:  baseURL,
		Status:  resp.Msg.GetStatus(),
		Version: resp.Msg.GetVersion(),
	}
	if statusJSON {
		return json.NewEncoder(os.Stdout).Encode(out)
	}
	fmt.Printf("Server:  %s\n", out.Server)
	fmt.Printf("Status:  %s\n", out.Status)
	fmt.Printf("Version: %s\n", out.Version)
	return nil
}
