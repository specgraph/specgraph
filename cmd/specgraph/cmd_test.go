// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"strings"
	"testing"
)

// TestSparkCmd_RequiresSlug verifies that the spark command requires exactly one positional argument.
func TestSparkCmd_RequiresSlug(t *testing.T) {
	// cobra validates Args: cobra.ExactArgs(1) before RunE is called.
	// Passing no args should produce an argument validation error.
	sparkCmd.ResetFlags()
	sparkCmd.Flags().StringVar(&sparkSeed, "seed", "", "seed idea (one sentence)")

	err := sparkCmd.Args(sparkCmd, []string{})
	if err == nil {
		t.Fatal("expected error when slug arg is missing")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

// TestSparkCmd_AcceptsSlug verifies that the spark command accepts exactly one positional argument.
func TestSparkCmd_AcceptsSlug(t *testing.T) {
	err := sparkCmd.Args(sparkCmd, []string{"my-spec"})
	if err != nil {
		t.Fatalf("unexpected error with valid slug: %v", err)
	}
}

// TestShapeCmd_RequiresSlug verifies that the shape command requires exactly one positional argument.
func TestShapeCmd_RequiresSlug(t *testing.T) {
	err := shapeCmd.Args(shapeCmd, []string{})
	if err == nil {
		t.Fatal("expected error when slug arg is missing")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

// TestShapeCmd_AcceptsSlug verifies that the shape command accepts exactly one positional argument.
func TestShapeCmd_AcceptsSlug(t *testing.T) {
	err := shapeCmd.Args(shapeCmd, []string{"my-spec"})
	if err != nil {
		t.Fatalf("unexpected error with valid slug: %v", err)
	}
}

// TestSpecifyCmd_RequiresSlug verifies that the specify command requires exactly one positional argument.
func TestSpecifyCmd_RequiresSlug(t *testing.T) {
	err := specifyCmd.Args(specifyCmd, []string{})
	if err == nil {
		t.Fatal("expected error when slug arg is missing")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

// TestDecomposeCmd_RequiresSlug verifies that the decompose command requires exactly one positional argument.
func TestDecomposeCmd_RequiresSlug(t *testing.T) {
	err := decomposeCmd.Args(decomposeCmd, []string{})
	if err == nil {
		t.Fatal("expected error when slug arg is missing")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

// TestShapeCmd_JSONFileFlag verifies that the --json-file flag is registered on the shape command.
func TestShapeCmd_JSONFileFlag(t *testing.T) {
	f := shapeCmd.Flags().Lookup("json-file")
	if f == nil {
		t.Fatal("expected --json-file flag to be registered on shape command")
	}
}

// TestSpecifyCmd_JSONFileFlag verifies that the --json-file flag is registered on the specify command.
func TestSpecifyCmd_JSONFileFlag(t *testing.T) {
	f := specifyCmd.Flags().Lookup("json-file")
	if f == nil {
		t.Fatal("expected --json-file flag to be registered on specify command")
	}
}

// TestDecomposeCmd_JSONFileFlag verifies that the --json-file flag is registered on the decompose command.
func TestDecomposeCmd_JSONFileFlag(t *testing.T) {
	f := decomposeCmd.Flags().Lookup("json-file")
	if f == nil {
		t.Fatal("expected --json-file flag to be registered on decompose command")
	}
}

// TestSparkCmd_SeedFlag verifies that the --seed flag is registered on the spark command.
func TestSparkCmd_SeedFlag(t *testing.T) {
	f := sparkCmd.Flags().Lookup("seed")
	if f == nil {
		t.Fatal("expected --seed flag to be registered on spark command")
	}
}
