// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/seanb4t/specgraph/internal/config"
	"github.com/seanb4t/specgraph/internal/docker"
	"github.com/seanb4t/specgraph/internal/service"
	"github.com/seanb4t/specgraph/internal/xdg"
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
	cfg, err := config.LoadGlobal(xdg.ConfigFile())
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if cfg.Server.Mode == "service" {
		if err := service.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: stop service: %v\n", err)
		}
		if downRM {
			defPath := filepath.Join(serviceDestDir(), serviceDefinitionFilename())
			if err := service.Uninstall(defPath); err != nil {
				fmt.Fprintf(os.Stderr, "warning: uninstall service: %v\n", err)
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

	fmt.Println("SpecGraph stopped")
	return nil
}
