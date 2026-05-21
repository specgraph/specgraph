// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package main implements the specgraph CLI, a live spec-driven development framework.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/xdg"
	"github.com/spf13/cobra"
)

// Set by goreleaser ldflags at build time.
var (
	version = "dev"
	commit  = "none"
)

// buildVersion returns a semver-compatible version string.
// Release builds (goreleaser): "0.1.0"
// Dev builds: "0.0.0-dev+none" (valid semver pre-release + build metadata)
func buildVersion() string {
	if version != "dev" {
		return version
	}
	return "0.0.0-dev+" + commit
}

var rootCmd = &cobra.Command{
	Use:           "specgraph",
	Short:         "Live spec-driven development framework",
	Version:       buildVersion(),
	SilenceErrors: true,
	SilenceUsage:  true,
}

var cfgFile string

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file path (default: ~/.config/specgraph/config.yaml, "+
			"or .specgraph/config.yaml when no project config is found)")
	// Wired in init() to avoid an initialization cycle (nudgePreRun
	// closes over rootCmd via the top-level allow-list walk).
	rootCmd.PersistentPreRunE = nudgePreRun
}

// globalConfigPath returns the global config path for server commands,
// honoring --config when set. Server commands must use this instead of
// xdg.ConfigFile() so the persistent root flag is respected.
func globalConfigPath() string {
	if cfgFile != "" {
		return cfgFile
	}
	return xdg.ConfigFile()
}

// legacyConfigPath returns the config path used by resolveBaseURL when no
// project .specgraph.yaml is found: --config if set, else .specgraph/config.yaml.
func legacyConfigPath() string {
	if cfgFile != "" {
		return cfgFile
	}
	return ".specgraph/config.yaml"
}

// loadGlobalCfg dispatches to LoadGlobalExplicit when --config is set so
// typo'd paths fail loudly; the XDG default path retains auto-create.
func loadGlobalCfg() (*config.GlobalConfig, error) {
	if cfgFile != "" {
		return config.LoadGlobalExplicit(cfgFile)
	}
	return config.LoadGlobal(xdg.ConfigFile())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			os.Exit(ee.code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
