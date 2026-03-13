// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package inject_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanb4t/specgraph/internal/inject"
	"github.com/seanb4t/specgraph/internal/storage"
)

func testSpec() *storage.Spec {
	return &storage.Spec{
		ID:         "spec-001",
		Slug:       "add-auth",
		Intent:     "Add authentication to the API",
		Stage:      storage.SpecStageApproved,
		Priority:   storage.SpecPriorityP1,
		Complexity: "medium",
		Version:    3,
	}
}

func testConstitution() *storage.Constitution {
	return &storage.Constitution{
		Name: "project-alpha",
		Tech: &storage.TechStack{
			Languages: &storage.Languages{
				Primary: "Go",
				Allowed: []string{"Go", "TypeScript"},
			},
			Frameworks: map[string]string{
				"web":  "ConnectRPC",
				"test": "testing",
			},
		},
		Constraints: []string{
			"All APIs must be idempotent",
			"No direct DB access from handlers",
		},
		Antipatterns: []storage.Antipattern{
			{
				Pattern: "God object",
				Why:     "Violates single responsibility",
				Instead: "Use composition",
			},
		},
	}
}

func TestInject_ClaudeCode(t *testing.T) {
	dir := t.TempDir()
	spec := testSpec()
	con := testConstitution()

	files, err := inject.Inject(spec, con, storage.InjectToolClaudeCode, dir)
	if err != nil {
		t.Fatalf("Inject returned error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	expected := filepath.Join(dir, ".claude", "specs", "add-auth.md")
	if files[0] != expected {
		t.Errorf("expected path %s, got %s", expected, files[0])
	}

	content, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	s := string(content)
	for _, want := range []string{
		"add-auth",
		"Add authentication to the API",
		"approved",
		"p1",
		"medium",
		"Go",
		"TypeScript",
		"ConnectRPC",
		"All APIs must be idempotent",
		"God object",
		"Use composition",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("content missing %q", want)
		}
	}
}

func TestInject_Cursor(t *testing.T) {
	dir := t.TempDir()
	spec := testSpec()
	con := testConstitution()

	files, err := inject.Inject(spec, con, storage.InjectToolCursor, dir)
	if err != nil {
		t.Fatalf("Inject returned error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	expected := filepath.Join(dir, ".cursor", "rules", "specgraph-add-auth.md")
	if files[0] != expected {
		t.Errorf("expected path %s, got %s", expected, files[0])
	}

	content, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	s := string(content)
	// Cursor files must have YAML frontmatter.
	if !strings.HasPrefix(s, "---\n") {
		t.Error("cursor file missing YAML frontmatter prefix")
	}
	for _, want := range []string{
		"description:",
		"alwaysApply: false",
		"add-auth",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("content missing %q", want)
		}
	}
}

func TestInject_AgentsMD(t *testing.T) {
	dir := t.TempDir()
	spec := testSpec()
	con := testConstitution()

	files, err := inject.Inject(spec, con, storage.InjectToolAgentsMD, dir)
	if err != nil {
		t.Fatalf("Inject returned error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	expected := filepath.Join(dir, "AGENTS.md")
	if files[0] != expected {
		t.Errorf("expected path %s, got %s", expected, files[0])
	}

	content, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if !strings.Contains(string(content), "add-auth") {
		t.Error("AGENTS.md missing spec slug")
	}
}

func TestInject_AgentsMD_ReplaceExisting(t *testing.T) {
	dir := t.TempDir()
	spec := testSpec()

	// First inject
	_, err := inject.Inject(spec, nil, storage.InjectToolAgentsMD, dir)
	if err != nil {
		t.Fatalf("first Inject: %v", err)
	}

	// Second inject with updated intent — should replace, not duplicate
	spec2 := &storage.Spec{
		ID: "spec-001", Slug: "add-auth", Intent: "Updated authentication intent",
		Stage: storage.SpecStageApproved, Priority: storage.SpecPriorityP1,
		Complexity: "medium", Version: 4,
	}
	_, err = inject.Inject(spec2, nil, storage.InjectToolAgentsMD, dir)
	if err != nil {
		t.Fatalf("second Inject: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	s := string(content)

	startMarker := "<!-- specgraph:add-auth:start -->"
	if count := strings.Count(s, startMarker); count != 1 {
		t.Errorf("expected 1 start marker, got %d", count)
	}
	if !strings.Contains(s, "Updated authentication intent") {
		t.Error("replaced section should contain updated intent")
	}
}

func TestInject_AgentsMD_AppendNewSlug(t *testing.T) {
	dir := t.TempDir()
	specA := testSpec() // slug "add-auth"

	_, err := inject.Inject(specA, nil, storage.InjectToolAgentsMD, dir)
	if err != nil {
		t.Fatalf("first Inject: %v", err)
	}

	specB := &storage.Spec{
		ID: "spec-002", Slug: "add-logging", Intent: "Add structured logging",
		Stage: storage.SpecStageSpark, Priority: storage.SpecPriorityP2,
		Complexity: "low", Version: 1,
	}
	_, err = inject.Inject(specB, nil, storage.InjectToolAgentsMD, dir)
	if err != nil {
		t.Fatalf("second Inject: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, "<!-- specgraph:add-auth:start -->") {
		t.Error("missing section for slug add-auth")
	}
	if !strings.Contains(s, "<!-- specgraph:add-logging:start -->") {
		t.Error("missing section for slug add-logging")
	}
}

func TestInject_AgentsMD_ReversedMarkers(t *testing.T) {
	dir := t.TempDir()
	slug := "my-spec"
	// Write file with end marker before start marker
	corrupted := fmt.Sprintf("<!-- specgraph:%s:end -->\nsome content\n<!-- specgraph:%s:start -->\n", slug, slug)
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(corrupted), 0o600); err != nil {
		t.Fatal(err)
	}

	spec := &storage.Spec{
		ID: "spec-003", Slug: slug, Intent: "test",
		Stage: storage.SpecStageSpark, Priority: storage.SpecPriorityP2,
	}
	_, err := inject.Inject(spec, nil, storage.InjectToolAgentsMD, dir)
	if err == nil {
		t.Fatal("expected error for reversed markers, got nil")
	}
	if !strings.Contains(err.Error(), "end marker") || !strings.Contains(err.Error(), "before start marker") {
		t.Errorf("error should mention reversed markers, got: %v", err)
	}
}

func TestInject_AgentsMD_MismatchedMarkers(t *testing.T) {
	dir := t.TempDir()
	slug := "my-spec"
	// Write file with only a start marker (no end marker)
	partial := fmt.Sprintf("<!-- specgraph:%s:start -->\norphaned content\n", slug)
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(partial), 0o600); err != nil {
		t.Fatal(err)
	}

	spec := &storage.Spec{
		ID: "spec-003", Slug: slug, Intent: "test",
		Stage: storage.SpecStageSpark, Priority: storage.SpecPriorityP2,
	}
	_, err := inject.Inject(spec, nil, storage.InjectToolAgentsMD, dir)
	if err == nil {
		t.Fatal("expected error for mismatched markers, got nil")
	}
	if !strings.Contains(err.Error(), "mismatched markers") {
		t.Errorf("error should mention mismatched markers, got: %v", err)
	}
}

func TestInject_UnsupportedTool(t *testing.T) {
	dir := t.TempDir()
	spec := testSpec()

	_, err := inject.Inject(spec, nil, storage.InjectToolType("unknown-tool"), dir)
	if err == nil {
		t.Fatal("expected error for unsupported tool, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("error should mention 'unsupported', got: %v", err)
	}
}

func TestInject_NilSpec(t *testing.T) {
	dir := t.TempDir()
	_, err := inject.Inject(nil, nil, storage.InjectToolClaudeCode, dir)
	if err == nil {
		t.Fatal("expected error for nil spec, got nil")
	}
	if !strings.Contains(err.Error(), "spec cannot be nil") {
		t.Errorf("error should mention nil spec, got: %v", err)
	}
}

func TestInject_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	spec := &storage.Spec{
		Slug:   "../../etc/passwd",
		Intent: "malicious",
	}
	_, err := inject.Inject(spec, nil, storage.InjectToolClaudeCode, dir)
	if err == nil {
		t.Fatal("expected error for path-traversal slug, got nil")
	}
	if !strings.Contains(err.Error(), "must not contain path separators") {
		t.Errorf("error should mention path separators, got: %v", err)
	}
}

func TestInject_NilConstitution(t *testing.T) {
	dir := t.TempDir()
	spec := testSpec()

	files, err := inject.Inject(spec, nil, storage.InjectToolClaudeCode, dir)
	if err != nil {
		t.Fatalf("Inject returned error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	content, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	s := string(content)
	// Spec fields should still be present.
	if !strings.Contains(s, "add-auth") {
		t.Error("content missing spec slug")
	}
	if !strings.Contains(s, "Add authentication to the API") {
		t.Error("content missing spec intent")
	}
	// Constitution-specific content should NOT be present.
	if strings.Contains(s, "Primary Language") {
		t.Error("nil constitution should not produce language section")
	}
}

func TestInject_Cursor_SpecialCharacters(t *testing.T) {
	dir := t.TempDir()
	spec := testSpec()
	spec.Intent = `Support "quoted" identifiers with back\slash`
	con := testConstitution()

	files, err := inject.Inject(spec, con, storage.InjectToolCursor, dir)
	if err != nil {
		t.Fatalf("Inject returned error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	expected := filepath.Join(dir, ".cursor", "rules", "specgraph-add-auth.md")
	if files[0] != expected {
		t.Errorf("expected path %s, got %s", expected, files[0])
	}

	content, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	s := string(content)
	// Frontmatter must be present and valid.
	if !strings.HasPrefix(s, "---\n") {
		t.Error("cursor file missing YAML frontmatter prefix")
	}
	if !strings.Contains(s, "description:") {
		t.Error("frontmatter missing description field")
	}
	// Double-quotes must be escaped as \" in the YAML value.
	if !strings.Contains(s, `\"quoted\"`) {
		t.Errorf("frontmatter should contain escaped quotes, got:\n%s", s)
	}
	// Backslashes must be escaped as \\ in the YAML value.
	if !strings.Contains(s, `back\\slash`) {
		t.Errorf("frontmatter should contain escaped backslash, got:\n%s", s)
	}
}

func TestInject_InvalidSlugSpecialChars(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		slug string
	}{
		{"newline", "slug\nwith\nnewlines"},
		{"null byte", "slug\x00evil"},
		{"space", "slug with spaces"},
		{"tab", "slug\twith\ttabs"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := &storage.Spec{
				Slug:   tc.slug,
				Intent: "test",
			}
			_, err := inject.Inject(spec, nil, storage.InjectToolClaudeCode, dir)
			if err == nil {
				t.Fatalf("expected error for slug with %s, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), "invalid spec slug") {
				t.Errorf("error should mention invalid slug, got: %v", err)
			}
		})
	}
}
