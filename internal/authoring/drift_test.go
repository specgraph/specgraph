// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"regexp"
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestContentProtoDrift(t *testing.T) {
	cases := []struct {
		file    string
		message protoreflect.MessageDescriptor
	}{
		{"stage-shape.md", (&specv1.ShapeOutput{}).ProtoReflect().Descriptor()},
		{"stage-specify.md", (&specv1.SpecifyOutput{}).ProtoReflect().Descriptor()},
		{"stage-decompose.md", (&specv1.DecomposeOutput{}).ProtoReflect().Descriptor()},
		{"stage-spark.md", (&specv1.SparkOutput{}).ProtoReflect().Descriptor()},
	}

	fieldPattern := regexp.MustCompile("`([a-z][a-z0-9_]*)`")
	// Strip fenced code blocks (JSON/CLI examples) so their inline snake_case
	// tokens don't pollute the drift scan. The scan targets prose references
	// to proto fields, not example payload keys.
	fenceRE := regexp.MustCompile("(?s)```.*?```")

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			content, err := Content(tc.file)
			if err != nil {
				t.Fatalf("load %s: %v", tc.file, err)
			}
			scanned := fenceRE.ReplaceAllString(string(content), "")
			knownFields := map[string]bool{}
			fields := tc.message.Fields()
			for i := 0; i < fields.Len(); i++ {
				knownFields[string(fields.Get(i).Name())] = true
			}
			// Narrow allowlist: tokens that are demonstrably NOT proto fields
			// (English phrases, other-struct names, CLI/MCP args). Do NOT
			// include actual proto field names — the point is to CATCH drift
			// on those.
			allowlist := map[string]bool{
				"specgraph":   true, // package / URI prefix
				"author":      true, // MCP tool name
				"graph_query": true, // MCP tool name
				"spec_slug":   true, // MCP argument name
			}
			matches := fieldPattern.FindAllStringSubmatch(scanned, -1)
			for _, m := range matches {
				tok := m[1]
				if !strings.Contains(tok, "_") {
					continue // single-word tokens are common English; skip
				}
				if knownFields[tok] || allowlist[tok] {
					continue
				}
				t.Errorf("%s references %q which is not a field on %s and is not allowlisted (drift or typo?)",
					tc.file, tok, tc.message.Name())
			}
		})
	}
}
