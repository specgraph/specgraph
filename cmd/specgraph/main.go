// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package main implements the specgraph CLI, a live spec-driven development framework.
package main

import (
	"errors"
	"fmt"
	"os"

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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", ".specgraph/config.yaml", "config file path")
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
