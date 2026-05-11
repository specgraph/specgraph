// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"strings"
	"testing"
)

func TestRenderV1AgentsBlockBody(t *testing.T) {
	params := ProjectParams{Slug: "myproj", ServerURL: "http://localhost:9090"}
	got := string(renderV1AgentsBlockBody(params))

	mustContain := []string{
		"# SpecGraph project pointer",
		"Server: http://localhost:9090",
		"Project: myproj (sent as the X-Specgraph-Project header)",
		"This block is managed by `specgraph init`. Edit content outside the markers.",
		"Resources to consult: `specgraph://prime`, `specgraph://constitution`, `specgraph://spec/{slug}`.",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("body missing %q\nFull body:\n%s", s, got)
		}
	}
	// Must NOT contain markers — body bytes only.
	if strings.Contains(got, "<!-- specgraph:init") {
		t.Error("body must not include marker lines")
	}
}

func TestRenderV1CursorBlockBody(t *testing.T) {
	params := ProjectParams{Slug: "x", ServerURL: "http://h"}
	agents := string(renderV1AgentsBlockBody(params))
	cursor := string(renderV1CursorBlockBody(params))
	if cursor != agents {
		t.Errorf("cursor body must equal agents body for v=1; got\n%q\nvs\n%q", cursor, agents)
	}
}
