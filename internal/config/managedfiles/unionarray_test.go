// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"testing"
)

func TestUnionPluginArray(t *testing.T) {
	cases := []struct {
		name     string
		existing string
		canon    string
		want     []string
	}{
		{
			name:     "missing existing -> canon only",
			existing: ``,
			canon:    `{"plugin":["./.specgraph/agents/opencode/specgraph.ts"]}`,
			want:     []string{"./.specgraph/agents/opencode/specgraph.ts"},
		},
		{
			name:     "existing has only our path -> no change",
			existing: `{"plugin":["./.specgraph/agents/opencode/specgraph.ts"]}`,
			canon:    `{"plugin":["./.specgraph/agents/opencode/specgraph.ts"]}`,
			want:     []string{"./.specgraph/agents/opencode/specgraph.ts"},
		},
		{
			name:     "existing has user path -> union, canonical first",
			existing: `{"plugin":["./user-plugin.ts"]}`,
			canon:    `{"plugin":["./.specgraph/agents/opencode/specgraph.ts"]}`,
			want:     []string{"./.specgraph/agents/opencode/specgraph.ts", "./user-plugin.ts"},
		},
		{
			name:     "existing has our path + user path -> dedup, canon first",
			existing: `{"plugin":["./user-plugin.ts","./.specgraph/agents/opencode/specgraph.ts"]}`,
			canon:    `{"plugin":["./.specgraph/agents/opencode/specgraph.ts"]}`,
			want:     []string{"./.specgraph/agents/opencode/specgraph.ts", "./user-plugin.ts"},
		},
		{
			name:     "existing has plugin field absent -> just canon",
			existing: `{"mcp":{}}`,
			canon:    `{"mcp":{},"plugin":["./.specgraph/agents/opencode/specgraph.ts"]}`,
			want:     []string{"./.specgraph/agents/opencode/specgraph.ts"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := unionPluginArray([]byte(tc.existing), []byte(tc.canon))
			if err != nil {
				t.Fatalf("unionPluginArray: %v", err)
			}
			got, perr := readPluginArray(out)
			if perr != nil {
				t.Fatalf("extract plugin array from output: %v", perr)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d; out=%s", len(got), len(tc.want), out)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestUnionPluginArrayTrailingNewline(t *testing.T) {
	out, _ := unionPluginArray([]byte("{}"), []byte("{\"plugin\":[\"a\"]}\n"))
	if !bytes.HasSuffix(out, []byte("\n")) {
		t.Error("output missing trailing newline")
	}
}
