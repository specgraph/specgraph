// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/seanb4t/specgraph/internal/config"
	"github.com/spf13/cobra"
)

func readLine(r *bufio.Reader) string {
	line, err := r.ReadString('\n')
	if err != nil {
		return strings.TrimSpace(line) // return partial on EOF
	}
	return strings.TrimSpace(line)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a SpecGraph project",
	RunE:  runInit,
}

var initYes bool

func init() {
	initCmd.Flags().BoolVar(&initYes, "yes", false, "non-interactive mode with defaults")
	rootCmd.AddCommand(initCmd)
}

func runInit(_ *cobra.Command, _ []string) error {
	configPath := cfgFile

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config already exists at %s", configPath)
	}

	cfg := &config.Config{}

	if initYes {
		// Non-interactive: use defaults
		cfg.Storage.Backend = "memgraph"
		cfg.Server.Mode = "docker"
	} else {
		reader := bufio.NewReader(os.Stdin)

		// Backend
		fmt.Print("Storage backend (memgraph/postgres) [memgraph]: ")
		backend := readLine(reader)
		if backend == "" {
			backend = "memgraph"
		}
		cfg.Storage.Backend = backend

		// Mode
		fmt.Print("Deployment mode (docker/external) [docker]: ")
		mode := readLine(reader)
		if mode == "" {
			mode = "docker"
		}
		cfg.Server.Mode = mode

		// If external, ask for connection details
		if mode == "external" {
			switch backend {
			case "memgraph":
				fmt.Print("Memgraph bolt URI [bolt://localhost:7687]: ")
				if uri := readLine(reader); uri != "" {
					cfg.Storage.Memgraph.BoltURI = uri
				}
			case "postgres":
				fmt.Print("Postgres URL: ")
				cfg.Storage.Postgres.URL = readLine(reader)
			}
		}
	}

	if err := cfg.Write(configPath); err != nil {
		return err
	}
	fmt.Printf("Initialized SpecGraph project at %s\n", configPath)
	return nil
}
