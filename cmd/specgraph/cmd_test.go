// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestAuthoringCmds_RequireSlug(t *testing.T) {
	cmds := map[string]*cobra.Command{
		"spark":     sparkCmd,
		"shape":     shapeCmd,
		"specify":   specifyCmd,
		"decompose": decomposeCmd,
	}
	for name, cmd := range cmds {
		t.Run(name, func(t *testing.T) {
			err := cmd.Args(cmd, []string{})
			if err == nil {
				t.Fatalf("%s: expected error when slug arg is missing", name)
			}
			if !strings.Contains(err.Error(), "accepts 1 arg") {
				t.Errorf("%s: unexpected error message: %q", name, err.Error())
			}
		})
	}
}

func TestAuthoringCmds_AcceptSlug(t *testing.T) {
	cmds := map[string]*cobra.Command{
		"spark":     sparkCmd,
		"shape":     shapeCmd,
		"specify":   specifyCmd,
		"decompose": decomposeCmd,
	}
	for name, cmd := range cmds {
		t.Run(name, func(t *testing.T) {
			if err := cmd.Args(cmd, []string{"my-spec"}); err != nil {
				t.Fatalf("%s: unexpected error with valid slug: %v", name, err)
			}
		})
	}
}

func TestAuthoringCmds_JSONFileFlag(t *testing.T) {
	cmds := map[string]*cobra.Command{
		"shape":     shapeCmd,
		"specify":   specifyCmd,
		"decompose": decomposeCmd,
	}
	for name, cmd := range cmds {
		t.Run(name, func(t *testing.T) {
			if f := cmd.Flags().Lookup("json-file"); f == nil {
				t.Fatalf("%s: expected --json-file flag to be registered", name)
			}
		})
	}
}

func TestSparkCmd_SeedFlag(t *testing.T) {
	// spark has --seed instead of --json-file.
	sparkCmd.ResetFlags()
	sparkCmd.Flags().StringVar(&sparkSeed, "seed", "", "seed idea (one sentence)")

	if f := sparkCmd.Flags().Lookup("seed"); f == nil {
		t.Fatal("expected --seed flag to be registered on spark command")
	}
}
