// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/seanb4t/specgraph/internal/config"
	"github.com/seanb4t/specgraph/internal/scanner"
	"github.com/spf13/cobra"
)

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return strings.TrimSpace(line), fmt.Errorf("reading input: %w", err)
	}
	return strings.TrimSpace(line), nil
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a SpecGraph project",
	RunE:  runInit,
}

var initYes bool
var initScan bool

func init() {
	initCmd.Flags().BoolVar(&initYes, "yes", false, "non-interactive mode with defaults")
	initCmd.Flags().BoolVar(&initScan, "scan", false, "scan codebase and generate constitution draft")
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
		backend, err := readLine(reader)
		if err != nil {
			return err
		}
		if backend == "" {
			backend = "memgraph"
		}
		cfg.Storage.Backend = backend

		// Mode
		fmt.Print("Deployment mode (docker/external) [docker]: ")
		mode, err := readLine(reader)
		if err != nil {
			return err
		}
		if mode == "" {
			mode = "docker"
		}
		cfg.Server.Mode = mode

		// If external, ask for connection details
		if mode == "external" {
			switch backend {
			case "memgraph":
				fmt.Print("Memgraph bolt URI [bolt://localhost:7687]: ")
				uri, err := readLine(reader)
				if err != nil {
					return err
				}
				if uri != "" {
					cfg.Storage.Memgraph.BoltURI = uri
				}
			case "postgres":
				fmt.Print("Postgres URL: ")
				url, err := readLine(reader)
				if err != nil {
					return err
				}
				cfg.Storage.Postgres.URL = url
			}
		}
	}

	// Ensure parent directory exists
	if dir := filepath.Dir(configPath); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("creating config directory: %w", err)
		}
	}

	if err := cfg.Write(configPath); err != nil {
		return err
	}
	fmt.Printf("Initialized SpecGraph project at %s\n", configPath)

	doScan := initScan || initYes
	if doScan {
		if err := runConstitutionScan(configPath); err != nil {
			return err
		}
	} else {
		fmt.Println("Hint: run 'specgraph init --scan' to auto-generate a constitution draft.")
	}

	return nil
}

func runConstitutionScan(configPath string) error {
	fmt.Println("Scanning codebase for constitution draft...")
	proto, err := scanner.Scan(".")
	if err != nil {
		return fmt.Errorf("scanning codebase: %w", err)
	}

	c := &config.ConstitutionConfig{
		Name:  "project",
		Layer: "project",
	}

	if proto.Tech != nil {
		if proto.Tech.Languages != nil {
			c.Tech.Languages.Primary = proto.Tech.Languages.Primary
		}
		if len(proto.Tech.Frameworks) > 0 {
			c.Tech.Frameworks = proto.Tech.Frameworks
		}
		if len(proto.Tech.Infrastructure) > 0 {
			c.Tech.Infrastructure = proto.Tech.Infrastructure
		}
	}

	constitutionPath := filepath.Join(filepath.Dir(configPath), "constitution.yaml")

	fmt.Printf("Detected language: %s\n", c.Tech.Languages.Primary)
	if len(c.Tech.Frameworks) > 0 {
		fmt.Printf("Detected frameworks: %v\n", c.Tech.Frameworks)
	}
	if len(c.Tech.Infrastructure) > 0 {
		fmt.Printf("Detected infrastructure: %v\n", c.Tech.Infrastructure)
	}

	if err := config.WriteConstitutionYAML(constitutionPath, c); err != nil {
		return err
	}
	fmt.Printf("Constitution draft written to %s\n", constitutionPath)
	return nil
}
