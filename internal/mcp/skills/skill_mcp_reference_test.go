// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"context"
	"regexp"
	"strings"
	"testing"

	constload "github.com/specgraph/specgraph/internal/constitution/load"
	"github.com/specgraph/specgraph/internal/storage"
)

// cliAppendixHeader is the uniform gated-appendix marker every SKILL.md must
// carry so an MCP-only agent knows to skip the CLI section (D-05/D-07).
const cliAppendixHeader = "Requires local CLI"

// cliCommandRE matches a CLI-primary invocation: the lowercase binary name
// followed by a space and a subcommand letter (e.g. "specgraph constitution").
// It deliberately does NOT match the MCP resource prefix `specgraph://`
// (colon), the MCP tool prefix `specgraph_` (underscore), skill-name
// references like `specgraph-drift` (hyphen), or prose "SpecGraph" (capital).
var cliCommandRE = regexp.MustCompile(`specgraph [a-z]`)

// mcpReferenceRE matches at least one MCP tool or resource reference in a
// skill body: an MCP tool namespace (`specgraph_`) or a resource URI
// (`specgraph://`).
var mcpReferenceRE = regexp.MustCompile(`specgraph_|specgraph://`)

// bodyOf returns the verbatim SKILL.md bytes for a named skill.
func bodyOf(t *testing.T, src Source, name string) string {
	t.Helper()
	sk, err := src.Get(context.Background(), name)
	if err != nil {
		t.Fatalf("Get(%q): %v", name, err)
	}
	return string(sk.Body)
}

// TestSkillMCPReference guards the MCP-first posture of every embedded skill
// (criteria #1/#4): each SKILL.md leads with MCP tools/resources, gates the
// CLI behind a "Requires local CLI" appendix, and preserves its front-matter.
// It also binds the taught constitution/authoring YAML to the parsers that
// accept it, so a field-name typo fails this gate instead of reproducing the
// #1002 "skill teaches something the tool cannot accept" defect.
func TestSkillMCPReference(t *testing.T) {
	src, err := NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}

	cases := []struct {
		name string
		// mustContain are extra substrings the body MUST reference beyond the
		// generic MCP-reference check. For the two critical-path skills this
		// pins the exact write pattern they must teach.
		mustContain []string
	}{
		{"specgraph-analytical-passes", nil},
		{"specgraph-authoring", []string{"author", "output"}},
		{"specgraph-constitution", []string{"constitution", "update"}},
		{"specgraph-conventions", nil},
		{"specgraph-drift", nil},
		{"specgraph-graph-query", nil},
		{"specgraph-troubleshooting", nil},
	}

	for _, tc := range cases {
		t.Run("MCPReference/"+tc.name, func(t *testing.T) {
			body := bodyOf(t, src, tc.name)

			// (1) references at least one MCP tool/resource.
			if !mcpReferenceRE.MatchString(body) {
				t.Errorf("%s: body has no MCP tool/resource reference (expected `specgraph_` or `specgraph://`)", tc.name)
			}
			for _, sub := range tc.mustContain {
				if !strings.Contains(body, sub) {
					t.Errorf("%s: body must reference %q (MCP write pattern)", tc.name, sub)
				}
			}

			// (2) gated CLI appendix header is present.
			headerIdx := strings.Index(body, cliAppendixHeader)
			if headerIdx < 0 {
				t.Errorf("%s: missing gated appendix header %q", tc.name, cliAppendixHeader)
			}

			// (2b) CLI-appendix ordering guard: any `specgraph <cmd>` CLI
			// invocation must appear AFTER the appendix header. Scanned over
			// the body AFTER the front-matter — the front-matter `description`
			// legitimately names CLI trigger phrases. Skills with no CLI
			// commands pass trivially.
			content := stripFrontmatter(body)
			contentHeaderIdx := strings.Index(content, cliAppendixHeader)
			if loc := cliCommandRE.FindStringIndex(content); loc != nil {
				if contentHeaderIdx < 0 || loc[0] < contentHeaderIdx {
					t.Errorf("%s: CLI command %q appears at offset %d before the %q appendix (offset %d) — CLI must be gated in the appendix",
						tc.name, content[loc[0]:min(loc[0]+30, len(content))], loc[0], cliAppendixHeader, contentHeaderIdx)
				}
			}

			// (3) front-matter name + summary preserved.
			if !strings.Contains(body, "name: "+tc.name) {
				t.Errorf("%s: front-matter missing `name: %s`", tc.name, tc.name)
			}
			if !strings.Contains(body, "summary:") {
				t.Errorf("%s: front-matter missing `summary:`", tc.name)
			}
		})
	}

	t.Run("MCPReference/constitution-parse-binding", func(t *testing.T) {
		body := bodyOf(t, src, "specgraph-constitution")
		block := firstYAMLBlockContaining(body, "layer:")
		if block == "" {
			t.Fatal("specgraph-constitution: no ```yaml block containing `layer:` (the constitution write payload)")
		}
		c, err := constload.FromYAML([]byte(block))
		if err != nil {
			t.Fatalf("constitution write block does not parse through load.FromYAML: %v", err)
		}
		if c.Layer != storage.ConstitutionLayerProject {
			t.Errorf("constitution write block layer = %q, want %q", c.Layer, storage.ConstitutionLayerProject)
		}
	})

	t.Run("MCPReference/authoring-snake-case-guard", func(t *testing.T) {
		body := bodyOf(t, src, "specgraph-authoring")
		// Scope the key guard to the taught ```yaml `output` blocks — not the
		// surrounding prose (which legitimately names the camelCase forms as
		// anti-examples). The friendly-YAML stage `output` MUST use the
		// snake_case funnel keys the parser accepts, never their camelCase
		// equivalents (the exact `chosenApproach` vs `chosen_approach` typo
		// class that caused #1002).
		yamlBlocks := strings.Join(allYAMLBlocks(body), "\n")
		if yamlBlocks == "" {
			t.Fatal("specgraph-authoring: no ```yaml output blocks found")
		}
		for _, key := range []string{"scope_in", "chosen_approach", "success_must", "verify_criteria", "strategy"} {
			if !strings.Contains(yamlBlocks, key) {
				t.Errorf("specgraph-authoring: taught `output` must use snake_case key %q", key)
			}
		}
		for _, bad := range []string{"scopeIn", "chosenApproach", "verifyCriteria"} {
			if strings.Contains(yamlBlocks, bad) {
				t.Errorf("specgraph-authoring: taught `output` uses camelCase %q — the parser rejects it (#1002 class); use snake_case", bad)
			}
		}
	})

	t.Run("MCPReference/authoring-exchanges-gate", func(t *testing.T) {
		body := bodyOf(t, src, "specgraph-authoring")
		// The skill MUST teach the mandatory post-spark ConversationExchanges
		// payload (server enforces ≥1 exchange for shape/specify/decompose).
		for _, key := range []string{"exchanges", "sequence"} {
			if !strings.Contains(body, key) {
				t.Errorf("specgraph-authoring: must teach the `%s` payload for post-spark stages", key)
			}
		}
	})
}

// firstYAMLBlockContaining returns the inner content of the first fenced
// ```yaml block whose body contains needle, or "" if none.
func firstYAMLBlockContaining(body, needle string) string {
	for _, block := range allYAMLBlocks(body) {
		if strings.Contains(block, needle) {
			return block
		}
	}
	return ""
}

// stripFrontmatter returns the SKILL.md content after the closing `---`
// front-matter fence. If no front-matter is present, the whole body is
// returned. Used so the CLI-ordering guard ignores CLI trigger phrases in the
// front-matter `description`.
func stripFrontmatter(body string) string {
	const fence = "---\n"
	if !strings.HasPrefix(body, fence) {
		return body
	}
	rest := body[len(fence):]
	end := strings.Index(rest, "\n"+fence[:len(fence)-1])
	if end < 0 {
		return body
	}
	return rest[end:]
}

// allYAMLBlocks returns the inner content of every fenced ```yaml block in
// body, in order.
func allYAMLBlocks(body string) []string {
	const open = "```yaml"
	var out []string
	rest := body
	for {
		i := strings.Index(rest, open)
		if i < 0 {
			return out
		}
		afterOpen := rest[i+len(open):]
		nl := strings.IndexByte(afterOpen, '\n')
		if nl < 0 {
			return out
		}
		inner := afterOpen[nl+1:]
		closeIdx := strings.Index(inner, "```")
		if closeIdx < 0 {
			return out
		}
		out = append(out, inner[:closeIdx])
		rest = inner[closeIdx+3:]
	}
}
