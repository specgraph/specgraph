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

var rootCmd = &cobra.Command{
	Use:           "specgraph",
	Short:         "Live spec-driven development framework",
	SilenceErrors: true,
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
