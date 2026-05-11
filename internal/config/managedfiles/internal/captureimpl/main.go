// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package captureimpl is the one-shot golden-fixture generator for
// managedfiles PR B. It imports the deleted-in-this-PR mcpconfigs/
// and pointers/ packages, runs them against empty starting state, and
// writes their byte outputs to testdata/golden/<case>/out/.
//
// Deleted in the same commit that deletes mcpconfigs/ and pointers/.
// To regenerate fixtures after deletion, check out the
// PR-B-pre-cleanup commit and re-run `go run ./internal/config/managedfiles/internal/captureimpl`.

//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/specgraph/specgraph/internal/config/mcpconfigs"
	"github.com/specgraph/specgraph/internal/config/pointers"
)

// Cases captured: one per managed file. Each case has a starting
// state under testdata/golden/<case>/in/ and expected output under
// testdata/golden/<case>/out/.
//
// PR B's parity-test fixtures only need the Missing-file → first-init
// case for byte-equivalence of the JSON merge results and the v=1
// markdown body content. Drifted/edited cases are covered by the
// migration_test.go unit test, not goldens.

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "capture failed:", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := os.MkdirTemp("", "captureimpl-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)

	slug := "captureslug"
	serverURL := "http://localhost:9090"
	configs := mcpconfigs.ManagedConfigs(slug, serverURL)
	if _, err := mcpconfigs.Sync(root, configs); err != nil {
		return fmt.Errorf("mcpconfigs.Sync: %w", err)
	}
	opts, oerr := pointers.NewOptions(serverURL, slug)
	if oerr != nil {
		return oerr
	}
	report := pointers.Sync(root, opts)
	if report.IsErr() {
		return fmt.Errorf("pointers.Sync had errors: %+v", report)
	}

	// Copy outputs into testdata/golden/missing-first-init/out/.
	wd, _ := os.Getwd()
	outDir := filepath.Join(wd, "internal", "config", "managedfiles", "testdata", "golden", "missing-first-init", "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	for _, name := range []string{".mcp.json", ".cursor/mcp.json", "opencode.json", "AGENTS.md", ".cursor/rules/specgraph-bootstrap.md"} {
		src := filepath.Join(root, name)
		data, rerr := os.ReadFile(src)
		if rerr != nil {
			return fmt.Errorf("read %s: %w", src, rerr)
		}
		dst := filepath.Join(outDir, filepath.Base(name))
		if werr := os.WriteFile(dst, data, 0o644); werr != nil {
			return werr
		}
		fmt.Printf("captured: %s (%d bytes)\n", dst, len(data))
	}
	return nil
}
