// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package inject_test

import (
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
