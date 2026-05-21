// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"

	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
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

func runHealth(_ *cobra.Command, args []string) error {
	// `specgraph health` is deprecated; it now dispatches to
	// `specgraph doctor server`. The deprecation notice goes to stderr
	// so script consumers reading stdout don't see it. The doctor
	// server runE preserves the original health exit codes.
	fmt.Fprintln(os.Stderr,
		"specgraph health: deprecated, use `specgraph doctor server` (this command will be removed in a future release)")
	return doctorServerCmd.RunE(doctorServerCmd, args)
}
