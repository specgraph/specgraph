// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package skillvalidate validates agentskills.io-shaped SKILL.md packages
// in the repo. The contract is intentionally minimal so it stays useful
// across tooling churn: each <name>/SKILL.md must have YAML frontmatter
// whose required keys (name, description) are present and well-formed.
//
// The validator is invoked by `task skills:validate`; it walks the paths
// passed on the command line and reports per-file pass/fail. Exit code is
// non-zero if any package fails.
package skillvalidate

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Frontmatter is the parsed YAML frontmatter of a SKILL.md file. Only
// fields the validator actively checks are typed; unknown keys are tolerated.
type Frontmatter struct {
	Name          string         `yaml:"name"`
	Description   string         `yaml:"description"`
	License       string         `yaml:"license,omitempty"`
	Compatibility []string       `yaml:"compatibility,omitempty"`
	Metadata      map[string]any `yaml:"metadata,omitempty"`
}

// Result is the outcome of validating a single SKILL.md file.
type Result struct {
	Path    string
	OK      bool
	Reasons []string
}

// minDesc and maxDesc match the agentskills.io spec.
const (
	minDesc = 1
	maxDesc = 1024
)

// ValidateRoots walks each root looking for SKILL.md files, validates them,
// and returns the per-file results. A root may be an explicit SKILL.md
// path, a single skill directory, or a parent directory containing many.
func ValidateRoots(roots []string) ([]Result, error) {
	var results []Result
	for _, root := range roots {
		root = strings.TrimSuffix(root, "/...")
		info, err := os.Stat(root)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", root, err)
		}
		if !info.IsDir() {
			results = append(results, validateFile(root))
			continue
		}
		// filepath.WalkDir does not follow symlinks; resolve the root first
		// so that a repo-root reverse-symlink (e.g. skills →
		// internal/mcp/skills/embedded) is walked correctly.
		walkRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			return nil, fmt.Errorf("eval symlinks %s: %w", root, err)
		}
		walkErr := filepath.WalkDir(walkRoot, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Base(path) != "SKILL.md" {
				return nil
			}
			results = append(results, validateFile(path))
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("walk %s: %w", root, walkErr)
		}
	}
	return results, nil
}

// validateFile reads a single SKILL.md file and applies the spec checks.
func validateFile(path string) Result {
	res := Result{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		res.Reasons = append(res.Reasons, fmt.Sprintf("read: %v", err))
		return res
	}

	fm, body, err := splitFrontmatter(bufio.NewReader(bytes.NewReader(data)))
	if err != nil {
		res.Reasons = append(res.Reasons, err.Error())
		return res
	}
	var parsed Frontmatter
	if err := yaml.Unmarshal(fm, &parsed); err != nil {
		res.Reasons = append(res.Reasons, fmt.Sprintf("parse frontmatter: %v", err))
		return res
	}

	dirName := filepath.Base(filepath.Dir(path))
	if parsed.Name == "" {
		res.Reasons = append(res.Reasons, "frontmatter.name is required")
	} else if parsed.Name != dirName {
		res.Reasons = append(res.Reasons, fmt.Sprintf("frontmatter.name=%q must match directory name %q", parsed.Name, dirName))
	}

	desc := strings.TrimSpace(parsed.Description)
	switch {
	case desc == "":
		res.Reasons = append(res.Reasons, "frontmatter.description is required")
	case len(desc) < minDesc:
		res.Reasons = append(res.Reasons, fmt.Sprintf("frontmatter.description too short (%d < %d)", len(desc), minDesc))
	case len(desc) > maxDesc:
		res.Reasons = append(res.Reasons, fmt.Sprintf("frontmatter.description too long (%d > %d)", len(desc), maxDesc))
	}

	if strings.TrimSpace(string(body)) == "" {
		res.Reasons = append(res.Reasons, "skill body is empty")
	}

	res.OK = len(res.Reasons) == 0
	return res
}

// splitFrontmatter reads a SKILL.md file and returns the YAML frontmatter
// bytes and the body bytes. The expected layout is `---\n<yaml>\n---\n<body>`;
// missing frontmatter is an error.
func splitFrontmatter(r *bufio.Reader) (frontmatter, body []byte, err error) {
	first, err := r.ReadString('\n')
	if err != nil {
		return nil, nil, fmt.Errorf("read first line: %w", err)
	}
	if strings.TrimSpace(first) != "---" {
		return nil, nil, fmt.Errorf("expected frontmatter delimiter '---' on first line, got %q", strings.TrimSpace(first))
	}

	var fm strings.Builder
	for {
		line, lineErr := r.ReadString('\n')
		// bufio.Reader.ReadString returns both the partial line *and* io.EOF
		// when the delimiter is missing at end-of-file. Check the line content
		// before deciding whether the EOF is fatal — a SKILL.md ending with
		// `---` and no final newline is well-formed.
		if strings.TrimSpace(line) == "---" {
			break
		}
		if lineErr != nil {
			if errors.Is(lineErr, io.EOF) {
				return nil, nil, fmt.Errorf("frontmatter not closed before EOF")
			}
			return nil, nil, fmt.Errorf("frontmatter not closed: %w", lineErr)
		}
		fm.WriteString(line)
	}

	bodyBuf, readErr := io.ReadAll(r)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, nil, fmt.Errorf("read body: %w", readErr)
	}
	return []byte(fm.String()), bodyBuf, nil
}

// Summarize formats results as a human-readable report and returns whether
// every result passed.
func Summarize(results []Result, w *strings.Builder) (allOK bool) {
	allOK = true
	for _, r := range results {
		if r.OK {
			fmt.Fprintf(w, "OK    %s\n", r.Path)
			continue
		}
		allOK = false
		fmt.Fprintf(w, "FAIL  %s\n", r.Path)
		for _, reason := range r.Reasons {
			fmt.Fprintf(w, "      - %s\n", reason)
		}
	}
	fmt.Fprintf(w, "\n%d package(s) checked, ", len(results))
	if allOK {
		fmt.Fprint(w, "all OK\n")
	} else {
		fmt.Fprint(w, "some FAIL\n")
	}
	return allOK
}
