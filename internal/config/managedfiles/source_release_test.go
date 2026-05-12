// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build !dev

package managedfiles

import (
	"bytes"
	"testing"
)

func TestCanonicalSourcesEmbedded(t *testing.T) {
	data, err := canonicalSources.ReadFile("embedded/opencode/specgraph.ts")
	if err != nil {
		t.Fatalf("read embedded specgraph.ts: %v", err)
	}
	if len(data) == 0 {
		t.Error("embedded specgraph.ts is empty")
	}
	if !bytes.Contains(data, []byte("specgraph")) {
		t.Error("embedded specgraph.ts doesn't look like the right file")
	}
}
