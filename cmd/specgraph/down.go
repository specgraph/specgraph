// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/docker"
	"github.com/specgraph/specgraph/internal/service"
	"github.com/specgraph/specgraph/internal/xdg"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the SpecGraph server",
	RunE:  runDown,
}

var downRM bool

func init() {
	rootCmd.AddCommand(downCmd)
	downCmd.Flags().BoolVar(&downRM, "rm", false, "uninstall the service definition after stopping")
}

func runDown(_ *cobra.Command, _ []string) error {
	cfg, err := config.LoadGlobal(globalConfigPath())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	var stopErr error

	if cfg.Server.Mode == "service" {
		if downRM {
			// Uninstall handles stopping the service internally; no need for explicit Stop().
			destDir, err := serviceDestDir()
			if err != nil {
				return fmt.Errorf("service dest dir: %w", err)
			}
			defPath := filepath.Join(destDir, serviceDefinitionFilename())
			if err := service.Uninstall(defPath); err != nil {
				fmt.Fprintf(os.Stderr, "warning: uninstall service: %v\n", err)
				stopErr = err
			}
		} else {
			if err := service.Stop(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: stop service: %v\n", err)
				stopErr = err
			}
		}
	}

	if cfg.Server.Docker {
		composeFile := filepath.Join(xdg.DataHome(), "docker-compose.yaml")
		if _, err := os.Stat(composeFile); err == nil {
			if err := docker.ComposeDown(composeFile); err != nil {
				return fmt.Errorf("compose down: %w", err)
			}
		}
	}

	if stopErr != nil {
		return fmt.Errorf("stop service: %w", stopErr)
	}
	fmt.Println("SpecGraph stopped")
	return nil
}
