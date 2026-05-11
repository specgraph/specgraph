// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"io/fs"
	"testing"
)

// PR A's manifest is empty, so readSource is exercised with an empty
// embed.FS. We test the not-found path explicitly and rely on PR B+ to
// add tests for actual content.

func TestReadSource_EmptyManifestSourceMissing(t *testing.T) {
	mf := ManagedFile{
		Path:   ".specgraph/agents/opencode/specgraph.ts",
		Source: "opencode/specgraph.ts",
	}
	_, err := readSource(mf)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist, got %v", err)
	}
}

func TestReadSource_EmptySourceField(t *testing.T) {
	// JSON-key-merge files set Source="" because their canonical is built
	// programmatically, not embedded. readSource returns nil bytes + nil error
	// to signal "no embedded content; let the strategy build it."
	mf := ManagedFile{
		Path:     ".mcp.json",
		Strategy: StrategyJSONKeyMerge,
		Source:   "",
	}
	got, err := readSource(mf)
	if err != nil {
		t.Errorf("expected nil error for empty Source, got %v", err)
	}
	if got != nil {
		t.Errorf("expected nil bytes, got %q", got)
	}
}
