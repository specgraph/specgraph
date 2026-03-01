// Copyright (c) 2026 Sean Bartell. All rights reserved.
// Use of this source code is governed by the MIT license
// that can be found in the LICENSE file.

// Package main implements the specgraph CLI, a live spec-driven development framework.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "specgraph",
	Short: "Live spec-driven development framework",
}

var cfgFile string

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", ".specgraph/config.yaml", "config file path")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
