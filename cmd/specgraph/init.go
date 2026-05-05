// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/config/mcpconfigs"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [project-slug]",
	Short: "Initialize a SpecGraph project in the current directory",
	Long: "Writes .specgraph.yaml and the per-harness MCP config files " +
		"(.cursor/mcp.json, .mcp.json, opencode.json) for the current project. " +
		"Idempotent: safe to re-run on an already-initialized project; managed " +
		"fields are reset to canonical values, user-added fields are preserved.",
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

var initYes bool

func init() {
	initCmd.Flags().BoolVar(&initYes, "yes", false, "non-interactive (accepted for backward compat; init is always non-interactive)")
	rootCmd.AddCommand(initCmd)
}

func runInit(_ *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	var argSlug string
	if len(args) > 0 {
		argSlug = args[0]
	}

	// Resolve project state: load existing .specgraph.yaml if present.
	var existing *config.ProjectConfig
	if root, findErr := config.FindProjectRoot(cwd); findErr == nil {
		loaded, loadErr := config.LoadProject(root)
		if loadErr != nil {
			return fmt.Errorf("load existing project config: %w", loadErr)
		}
		existing = loaded
		cwd = root
	} else if !errors.Is(findErr, config.ErrProjectNotFound) {
		return fmt.Errorf("find project root: %w", findErr)
	}

	// Slug-consistency check: if both an arg and an existing config are
	// present and the slugs differ, refuse. The slug is identity-defining
	// (storage partition key, X-Specgraph-Project header value) and silent
	// mutation would orphan project data.
	if argSlug != "" && existing != nil && argSlug != existing.Slug {
		return fmt.Errorf(
			"cannot change project slug from %q to %q; edit .specgraph.yaml directly or remove it",
			existing.Slug, argSlug,
		)
	}

	// Determine the slug for this run.
	var pc *config.ProjectConfig
	switch {
	case existing != nil:
		pc = existing
	case argSlug != "":
		pc = &config.ProjectConfig{Slug: argSlug}
	default:
		// Derive from git remote / dir name (config.LoadProject already does
		// this when no .specgraph.yaml exists).
		derived, derErr := config.LoadProject(cwd)
		if derErr != nil {
			return fmt.Errorf("derive project slug: %w", derErr)
		}
		pc = &config.ProjectConfig{Slug: derived.Slug}
	}

	// Resolve and validate the server URL BEFORE any writes. A malformed
	// global config or a non-absolute-HTTP(S) resolved URL must fail fast,
	// before .specgraph.yaml is created on a fresh project. ResolveServer
	// itself cannot fail (returns a string), and url.Parse is too lenient
	// on its own: a bare "/api" parses with empty Scheme and Host;
	// "example.com" parses with empty Scheme; "localhost:3000" parses with
	// Scheme="localhost". We require Scheme ∈ {http, https} AND non-empty
	// Host to reject all three.
	globalCfg, err := loadGlobalCfg()
	if err != nil {
		return fmt.Errorf("load global config: %w", err)
	}
	serverURL := globalCfg.ResolveServer(pc.Slug, pc.Server)
	parsed, parseErr := url.Parse(serverURL)
	if parseErr != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		if parseErr != nil {
			return fmt.Errorf("resolved server URL %q must be an absolute http or https URL: %w", serverURL, parseErr)
		}
		return fmt.Errorf("resolved server URL %q must be an absolute http or https URL", serverURL)
	}
	configs := mcpconfigs.ManagedConfigs(pc.Slug, serverURL)

	// Write .specgraph.yaml only if it doesn't exist; idempotent.
	projectCreated := false
	if existing == nil {
		if writeErr := config.WriteProject(cwd, pc); writeErr != nil {
			return fmt.Errorf("write project config: %w", writeErr)
		}
		projectCreated = true
	}

	// Sync the per-harness MCP configs. Per-file actions are printed even
	// on partial failure so the user can see which files made it to disk.
	results, syncErr := mcpconfigs.Sync(cwd, configs)
	for _, r := range results {
		fmt.Printf("%s: %s\n", r.Path, r.Action)
	}
	if syncErr != nil {
		return fmt.Errorf("sync mcp configs: %w", syncErr)
	}

	// Only emit the success banner after Sync succeeds — printing it
	// alongside WriteProject would leave a success-sounding line on
	// stdout ahead of a non-zero exit if a later Sync step fails.
	if projectCreated {
		fmt.Printf("Initialized project %s. Config written to .specgraph.yaml\n", pc.Slug)
	}

	return nil
}
