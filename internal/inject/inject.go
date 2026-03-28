// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package inject writes spec execution context into tool-specific workspace files.
package inject

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/specgraph/specgraph/internal/storage"
)

// atomicWriteFile writes data to path atomically via a temp file + rename.
// The temp file is created in the same directory to ensure same-filesystem rename.
func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, writeErr := tmp.Write(data); writeErr != nil {
		tmp.Close()        //nolint:errcheck // best-effort cleanup on write failure
		os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("write temp file: %w", writeErr)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()        //nolint:errcheck // best-effort cleanup on chmod failure
		os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("rename temp to target: %w", err)
	}
	return nil
}

// safeSlugPattern matches slugs containing only alphanumerics, dots, underscores, and hyphens.
var safeSlugPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// Inject writes spec (and optional constitution) context to tool-specific files
// under outputDir. It returns the list of files written.
func Inject(spec *storage.Spec, constitution *storage.Constitution, tool storage.InjectToolType, outputDir string) ([]string, error) {
	if spec == nil {
		return nil, fmt.Errorf("spec cannot be nil")
	}
	if strings.ContainsRune(spec.Slug, filepath.Separator) || strings.ContainsRune(spec.Slug, '/') {
		return nil, fmt.Errorf("invalid spec slug %q: must not contain path separators", spec.Slug)
	}
	safeSlug := filepath.Base(spec.Slug)
	if safeSlug == "." || safeSlug == "" || !safeSlugPattern.MatchString(safeSlug) {
		return nil, fmt.Errorf("invalid spec slug: %q", spec.Slug)
	}
	content := renderMarkdown(spec, constitution)

	switch tool {
	case storage.InjectToolClaudeCode:
		return writeClaudeCode(content, safeSlug, outputDir)
	case storage.InjectToolCursor:
		return writeCursor(content, safeSlug, spec.Intent, outputDir)
	case storage.InjectToolAgentsMD:
		return writeAgentsMD(content, safeSlug, outputDir)
	default:
		return nil, fmt.Errorf("unsupported inject tool: %s", tool)
	}
}

// escapeTableCell escapes pipe characters and replaces newlines in markdown table cell values.
func escapeTableCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func renderMarkdown(spec *storage.Spec, con *storage.Constitution) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Spec: %s\n\n", escapeTableCell(spec.Slug))

	b.WriteString("| Field | Value |\n")
	b.WriteString("|-------|-------|\n")
	fmt.Fprintf(&b, "| Slug | %s |\n", escapeTableCell(spec.Slug))
	fmt.Fprintf(&b, "| Intent | %s |\n", escapeTableCell(spec.Intent))
	fmt.Fprintf(&b, "| Stage | %s |\n", escapeTableCell(string(spec.Stage)))
	fmt.Fprintf(&b, "| Priority | %s |\n", escapeTableCell(string(spec.Priority)))
	fmt.Fprintf(&b, "| Complexity | %s |\n", escapeTableCell(string(spec.Complexity)))
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
	unlock, lockErr := acquireFileLock(p)
	if lockErr != nil {
		return nil, lockErr
	}
	defer unlock()
	if err := atomicWriteFile(p, []byte(content)); err != nil {
		return nil, fmt.Errorf("write claude code spec: %w", err)
	}
	return []string{p}, nil
}

func writeCursor(content, slug, intent, outputDir string) ([]string, error) {
	dir := filepath.Join(outputDir, ".cursor", "rules")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create cursor rules dir: %w", err)
	}

	safeIntent := strings.ReplaceAll(intent, `\`, `\\`)
	safeIntent = strings.ReplaceAll(safeIntent, `"`, `\"`)
	safeIntent = strings.ReplaceAll(safeIntent, "\n", " ")
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "description: \"SpecGraph spec %s: %s\"\n", slug, safeIntent)
	b.WriteString("alwaysApply: false\n")
	b.WriteString("---\n\n")
	b.WriteString(content)

	p := filepath.Join(dir, "specgraph-"+slug+".md")
	unlock, lockErr := acquireFileLock(p)
	if lockErr != nil {
		return nil, lockErr
	}
	defer unlock()
	if err := atomicWriteFile(p, []byte(b.String())); err != nil {
		return nil, fmt.Errorf("write cursor rule: %w", err)
	}
	return []string{p}, nil
}

func writeAgentsMD(content, slug, outputDir string) ([]string, error) {
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	p := filepath.Clean(filepath.Join(outputDir, "AGENTS.md"))

	// Acquire file lock to prevent TOCTOU races between concurrent inject calls.
	unlock, err := acquireFileLock(p)
	if err != nil {
		return nil, err
	}
	defer unlock()

	startMarker := fmt.Sprintf("<!-- specgraph:%s:start -->", slug)
	endMarker := fmt.Sprintf("<!-- specgraph:%s:end -->", slug)
	section := startMarker + "\n" + content + "\n" + endMarker

	existing, readErr := os.ReadFile(p)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return nil, fmt.Errorf("read existing AGENTS.md: %w", readErr)
	}

	if errors.Is(readErr, fs.ErrNotExist) || len(existing) == 0 {
		if writeErr := atomicWriteFile(p, []byte(section+"\n")); writeErr != nil {
			return nil, fmt.Errorf("write AGENTS.md: %w", writeErr)
		}
		return []string{p}, nil
	}

	text := string(existing)
	startIdx := strings.Index(text, startMarker)
	endIdx := strings.Index(text, endMarker)

	switch {
	case startIdx >= 0 && endIdx >= 0 && startIdx < endIdx:
		// Replace existing section for this slug.
		text = text[:startIdx] + section + text[endIdx+len(endMarker):]
	case startIdx >= 0 && endIdx >= 0:
		// End marker appears before start marker — corrupted file.
		return nil, fmt.Errorf("corrupted AGENTS.md: end marker for slug %q appears before start marker", slug)
	case startIdx >= 0 || endIdx >= 0:
		// Only one marker present — corrupted file.
		return nil, fmt.Errorf("corrupted AGENTS.md: mismatched markers for slug %q (start=%v, end=%v)", slug, startIdx >= 0, endIdx >= 0)
	default:
		// Neither marker found — append new section.
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += "\n" + section + "\n"
	}

	if writeErr := atomicWriteFile(p, []byte(text)); writeErr != nil {
		return nil, fmt.Errorf("write AGENTS.md: %w", writeErr)
	}
	return []string{p}, nil
}
