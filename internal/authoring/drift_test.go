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
				// Cross-stage ShapeOutput field references: stage-specify.md
				// prose maps each Shape success criterion to a verify
				// assertion. These are valid proto fields on ShapeOutput, not
				// SpecifyOutput, so they aren't in this case's field set.
				"success_must":   true,
				"success_should": true,
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

// TestContentPersistenceContractSnakeCase guards the fenced ```yaml / ```json
// example payloads in the stage content files (the "Persistence Contract"
// blocks composed into the MCP prompts) against the camelCase protojson forms
// that the `author` tool's snake_case YAML parser rejects. This mirrors the
// authoring-snake-case-guard subtest in
// internal/mcp/skills/skill_mcp_reference_test.go so the composed prompt an
// agent receives first cannot drift back to teaching the #1002-class shape.
func TestContentPersistenceContractSnakeCase(t *testing.T) {
	fenceRE := regexp.MustCompile("(?s)```.*?```")
	// The camelCase stage-field forms the parser rejects. If any appears in a
	// fenced example block, an agent following the composed prompt would feed
	// the tool a shape it silently drops.
	banned := []string{
		"scopeIn", "scopeOut", "chosenApproach", "successMust", "successShould",
		"successWont", "scopeSniff", "killTest", "verifyCriteria", "changeType",
		"dependsOn",
	}
	files := []string{"stage-spark.md", "stage-shape.md", "stage-specify.md", "stage-decompose.md"}
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			content, err := Content(file)
			if err != nil {
				t.Fatalf("load %s: %v", file, err)
			}
			blocks := strings.Join(fenceRE.FindAllString(string(content), -1), "\n")
			if blocks == "" {
				t.Fatalf("%s: no fenced example blocks found (persistence contract missing?)", file)
			}
			for _, bad := range banned {
				if strings.Contains(blocks, bad) {
					t.Errorf("%s: fenced example teaches camelCase %q — the author-tool YAML parser rejects it (#1002 class); use snake_case", file, bad)
				}
			}
		})
	}
}
