// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package inject writes spec execution context into tool-specific workspace files.
package inject

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/seanb4t/specgraph/internal/storage"
)

// Inject writes spec (and optional constitution) context to tool-specific files
// under outputDir. It returns the list of files written.
func Inject(spec *storage.Spec, constitution *storage.Constitution, tool storage.InjectToolType, outputDir string) ([]string, error) {
	if spec == nil {
		return nil, fmt.Errorf("spec cannot be nil")
	}
	safeSlug := filepath.Base(spec.Slug)
	if safeSlug == "." || safeSlug == "/" || safeSlug == "" {
		return nil, fmt.Errorf("invalid spec slug: %q", spec.Slug)
	}
	content := renderMarkdown(spec, constitution)

	switch tool {
	case storage.InjectToolClaudeCode:
		return writeClaudeCode(content, safeSlug, outputDir)
	case storage.InjectToolCursor:
		return writeCursor(content, safeSlug, spec.Intent, outputDir)
	case storage.InjectToolAgentsMD:
		return writeAgentsMD(content, outputDir)
	default:
		return nil, fmt.Errorf("unsupported inject tool: %s", tool)
	}
}

func renderMarkdown(spec *storage.Spec, con *storage.Constitution) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Spec: %s\n\n", spec.Slug)

	b.WriteString("| Field | Value |\n")
	b.WriteString("|-------|-------|\n")
	fmt.Fprintf(&b, "| Slug | %s |\n", spec.Slug)
	fmt.Fprintf(&b, "| Intent | %s |\n", spec.Intent)
	fmt.Fprintf(&b, "| Stage | %s |\n", string(spec.Stage))
	fmt.Fprintf(&b, "| Priority | %s |\n", string(spec.Priority))
	fmt.Fprintf(&b, "| Complexity | %s |\n", spec.Complexity)
	fmt.Fprintf(&b, "| Version | %d |\n", spec.Version)

	if con == nil {
		return b.String()
	}

	b.WriteString("\n## Constitution\n\n")

	if con.Tech != nil && con.Tech.Languages != nil {
		fmt.Fprintf(&b, "**Primary Language:** %s\n\n", con.Tech.Languages.Primary)
		if len(con.Tech.Languages.Allowed) > 0 {
			fmt.Fprintf(&b, "**Allowed Languages:** %s\n\n", strings.Join(con.Tech.Languages.Allowed, ", "))
		}
	}

	if con.Tech != nil && len(con.Tech.Frameworks) > 0 {
		b.WriteString("### Frameworks\n\n")
		for k, v := range con.Tech.Frameworks {
			fmt.Fprintf(&b, "- **%s:** %s\n", k, v)
		}
		b.WriteString("\n")
	}

	if len(con.Constraints) > 0 {
		b.WriteString("### Constraints\n\n")
		for _, c := range con.Constraints {
			fmt.Fprintf(&b, "- %s\n", c)
		}
		b.WriteString("\n")
	}

	if len(con.Antipatterns) > 0 {
		b.WriteString("### Antipatterns\n\n")
		for _, ap := range con.Antipatterns {
			fmt.Fprintf(&b, "- **%s** — %s → %s\n", ap.Pattern, ap.Why, ap.Instead)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func writeClaudeCode(content, slug, outputDir string) ([]string, error) {
	dir := filepath.Join(outputDir, ".claude", "specs")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create claude specs dir: %w", err)
	}
	p := filepath.Join(dir, slug+".md")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		return nil, fmt.Errorf("write claude code spec: %w", err)
	}
	return []string{p}, nil
}

func writeCursor(content, slug, intent, outputDir string) ([]string, error) {
	dir := filepath.Join(outputDir, ".cursor", "rules")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create cursor rules dir: %w", err)
	}

	safeIntent := strings.ReplaceAll(intent, `"`, `\"`)
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "description: \"SpecGraph spec %s: %s\"\n", slug, safeIntent)
	b.WriteString("alwaysApply: false\n")
	b.WriteString("---\n\n")
	b.WriteString(content)

	p := filepath.Join(dir, "specgraph-"+slug+".md")
	if err := os.WriteFile(p, []byte(b.String()), 0o600); err != nil {
		return nil, fmt.Errorf("write cursor rule: %w", err)
	}
	return []string{p}, nil
}

func writeAgentsMD(content, outputDir string) ([]string, error) {
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	p := filepath.Join(outputDir, "AGENTS.md")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		return nil, fmt.Errorf("write AGENTS.md: %w", err)
	}
	return []string{p}, nil
}
