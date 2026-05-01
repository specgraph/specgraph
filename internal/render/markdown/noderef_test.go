// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package markdown

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestNodeRefList(t *testing.T) {
	refs := []*specv1.NodeRef{
		{Slug: "token-storage", Stage: "approved"},
		{Slug: "crypto-utils", Stage: "done"},
	}
	got := NodeRefList("Dependencies", refs)
	if !strings.Contains(got, "## Dependencies") {
		t.Error("missing heading")
	}
	if !strings.Contains(got, "| Slug | Stage |") {
		t.Error("missing header")
	}
	if !strings.Contains(got, "| token-storage | approved |") {
		t.Error("missing first row")
	}
}

func TestNodeRefListEmpty(t *testing.T) {
	got := NodeRefList("Dependencies", nil)
	if !strings.Contains(got, "None.") {
		t.Error("expected empty message")
	}
}
