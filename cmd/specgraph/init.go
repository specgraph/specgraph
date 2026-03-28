// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"os"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [project-slug]",
	Short: "Initialize a SpecGraph project in the current directory",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runInit,
}

var initYes bool

func init() {
	initCmd.Flags().BoolVar(&initYes, "yes", false, "non-interactive (accepted for backward compat; init is always non-interactive)")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	if err := runUp(cmd, nil); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "warning: server not started: %v\n", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Reject re-init if .specgraph.yaml already exists.
	if _, findErr := config.FindProjectRoot(cwd); findErr == nil {
		return fmt.Errorf("project already initialized (found .specgraph.yaml)")
	}

	var slug string
	if len(args) > 0 && args[0] != "" {
		slug = args[0]
	} else {
		// LoadProject derives slug from git remote origin or dir name when no
		// .specgraph.yaml exists yet.
		derived, err := config.LoadProject(cwd)
		if err != nil {
			return fmt.Errorf("derive project slug: %w", err)
		}
		slug = derived.Slug
	}

	pc := &config.ProjectConfig{Slug: slug}
	if err := config.WriteProject(cwd, pc); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}

	fmt.Printf("Initialized project %s. Config written to .specgraph.yaml\n", slug)
	return nil
}
