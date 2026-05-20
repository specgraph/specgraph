// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrProjectNotFound is returned when no .specgraph.yaml is found walking up
// the directory tree.
var ErrProjectNotFound = errors.New("project config not found")

// ProjectConfig is the per-repo .specgraph.yaml.
type ProjectConfig struct {
	Slug      string   `yaml:"project,omitempty"`
	Server    string   `yaml:"server,omitempty"`
	Harnesses []string `yaml:"harnesses,omitempty"`
	Nudges    Nudges   `yaml:"nudges,omitempty"`
}

// Nudges configures the drift-nudge that fires on every CLI invocation.
// Quiet suppresses the nudge at the project level (the SPECGRAPH_DRIFT_NUDGE
// environment variable does the same at the user level).
type Nudges struct {
	Quiet bool `yaml:"quiet,omitempty"`
}

const projectFileName = ".specgraph.yaml"

// LoadProject loads project config from dir, walking up to find
// .specgraph.yaml. If no file found, derives slug from git remote or dir name.
func LoadProject(dir string) (*ProjectConfig, error) {
	root, findErr := FindProjectRoot(dir)
	if findErr != nil {
		// Only fall back to derived slug if the error indicates no config file
		// was found. Other errors (e.g., permission denied) should propagate.
		if !errors.Is(findErr, ErrProjectNotFound) {
			return nil, fmt.Errorf("find project root: %w", findErr)
		}
		slug := deriveSlug(dir)
		return &ProjectConfig{Slug: slug}, nil
	}

	data, err := os.ReadFile(filepath.Join(root, projectFileName))
	if err != nil {
		return nil, fmt.Errorf("read project config: %w", err)
	}

	var pc ProjectConfig
	if err := yaml.Unmarshal(data, &pc); err != nil {
		return nil, fmt.Errorf("parse project config: %w", err)
	}

	if pc.Slug == "" {
		pc.Slug = deriveSlug(root)
	}
	return &pc, nil
}

// FindProjectRoot walks from dir upward looking for .specgraph.yaml.
func FindProjectRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, projectFileName)); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("%w: no %s found", ErrProjectNotFound, projectFileName)
		}
		abs = parent
	}
}

// WriteProject writes a .specgraph.yaml to dir.
func WriteProject(dir string, pc *ProjectConfig) error {
	data, err := yaml.Marshal(pc)
	if err != nil {
		return fmt.Errorf("marshal project config: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, projectFileName), data, 0o644); err != nil { //nolint:gosec // 0644 is intentional for user-readable config
		return fmt.Errorf("write project config: %w", err)
	}
	return nil
}

// NormalizeSlug converts a remote URL or owner/repo into a kebab-case slug.
func NormalizeSlug(raw string) string {
	s := raw
	s = strings.TrimSuffix(s, ".git")
	if idx := strings.LastIndex(s, ":"); idx != -1 && strings.Contains(s, "@") && !strings.Contains(s, "://") {
		// SSH-style remote (git@host:owner/repo) — strip everything before the colon.
		s = s[idx+1:]
	} else if strings.Contains(s, "://") {
		parts := strings.SplitN(s, "://", 2)
		if len(parts) == 2 {
			s = parts[1]
		}
		if idx := strings.Index(s, "/"); idx != -1 {
			s = s[idx+1:]
		}
	}
	s = strings.ReplaceAll(s, "/", "-")
	return strings.ToLower(s)
}

// ValidateProjectStrict re-reads the file at path and decodes it with
// KnownFields(true). Returns nil if the file decodes cleanly; returns
// an error naming the offending field(s) otherwise. Doctor's Project
// config group is the only caller; init/nudge/everywhere else stays
// on LoadProject (lenient).
func ValidateProjectStrict(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read project config: %w", err)
	}
	var pc ProjectConfig
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&pc); err != nil {
		return fmt.Errorf("strict decode: %w", err)
	}
	return nil
}

func deriveSlug(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	cmd := exec.Command("git", "-C", abs, "remote", "get-url", "origin")
	out, gitErr := cmd.Output()
	if gitErr == nil {
		remote := strings.TrimSpace(string(out))
		if remote != "" {
			return NormalizeSlug(remote)
		}
	}
	return strings.ToLower(filepath.Base(abs))
}
