// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestEdgeList(t *testing.T) {
	edges := []*specv1.Edge{
		{FromId: "login-api", ToId: "token-storage", EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON},
		{FromId: "api-gateway", ToId: "login-api", EdgeType: specv1.EdgeType_EDGE_TYPE_BLOCKS},
	}
	got := EdgeList("login-api", edges)
	if !strings.Contains(got, "## Edges for login-api") {
		t.Error("missing heading")
	}
	if !strings.Contains(got, "| Type | Direction | Target |") {
		t.Error("missing header")
	}
	if !strings.Contains(got, "| DEPENDS_ON | outgoing | token-storage |") {
		t.Error("missing outgoing edge")
	}
	if !strings.Contains(got, "| BLOCKS | incoming | api-gateway |") {
		t.Error("missing incoming edge")
	}
}

func TestEdgeListEmpty(t *testing.T) {
	got := EdgeList("test", nil)
	if !strings.Contains(got, "No edges found.") {
		t.Error("expected empty message")
	}
}
