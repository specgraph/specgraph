// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadProject_ExplicitSlug(t *testing.T) {
	dir := t.TempDir()
	yaml := "project: my-cool-project\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".specgraph.yaml"), []byte(yaml), 0o600))

	p, err := config.LoadProject(dir)
	require.NoError(t, err)
	assert.Equal(t, "my-cool-project", p.Slug)
	assert.Empty(t, p.Server)
}

func TestLoadProject_WithServerOverride(t *testing.T) {
	dir := t.TempDir()
	yaml := "project: foo\nserver: https://remote:7890\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".specgraph.yaml"), []byte(yaml), 0o600))

	p, err := config.LoadProject(dir)
	require.NoError(t, err)
	assert.Equal(t, "foo", p.Slug)
	assert.Equal(t, "https://remote:7890", p.Server)
}

func TestLoadProject_NoFile_DeriveFromDir(t *testing.T) {
	dir := t.TempDir()
	p, err := config.LoadProject(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Base(dir), p.Slug)
}

func TestFindProjectRoot_WalksUp(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".specgraph.yaml"), []byte("project: root-proj\n"), 0o600))
	child := filepath.Join(root, "src", "pkg")
	require.NoError(t, os.MkdirAll(child, 0o750))

	found, err := config.FindProjectRoot(child)
	require.NoError(t, err)
	assert.Equal(t, root, found)
}

func TestProjectConfig_DecodesNewFields(t *testing.T) {
	dir := t.TempDir()
	yaml := `project: my-spec
server: https://example.com
harnesses:
  - claude
  - cursor
nudges:
  quiet: true
`
	if err := os.WriteFile(filepath.Join(dir, ".specgraph.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cfg, err := config.LoadProject(dir)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if got := cfg.Slug; got != "my-spec" {
		t.Errorf("Slug = %q, want my-spec", got)
	}
	if got := cfg.Server; got != "https://example.com" {
		t.Errorf("Server = %q", got)
	}
	if !reflect.DeepEqual(cfg.Harnesses, []string{"claude", "cursor"}) {
		t.Errorf("Harnesses = %v, want [claude cursor]", cfg.Harnesses)
	}
	if !cfg.Nudges.Quiet {
		t.Errorf("Nudges.Quiet = false, want true")
	}
}

func TestProjectConfig_EmptyHarnessesAcceptedAsLegacy(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".specgraph.yaml"), []byte("project: legacy\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	cfg, err := config.LoadProject(dir)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if len(cfg.Harnesses) != 0 {
		t.Errorf("Harnesses = %v, want empty", cfg.Harnesses)
	}
	if cfg.Nudges.Quiet {
		t.Errorf("Nudges.Quiet = true, want false (zero value)")
	}
}

func TestValidateProjectStrict_AcceptsKnownKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".specgraph.yaml")
	yaml := `project: x
server: https://example.com
harnesses: [claude]
nudges:
  quiet: false
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := config.ValidateProjectStrict(path); err != nil {
		t.Errorf("ValidateProjectStrict on known-keys config: %v", err)
	}
}

func TestValidateProjectStrict_RejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".specgraph.yaml")
	yaml := `project: x
fnord: 42
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	err := config.ValidateProjectStrict(path)
	if err == nil {
		t.Fatal("expected strict-decode error on unknown key, got nil")
	}
	if !strings.Contains(err.Error(), "fnord") {
		t.Errorf("error %q does not name the unknown key 'fnord'", err.Error())
	}
}

func TestNormalizeSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"owner/repo", "owner-repo"},
		{"git@github.com:owner/repo.git", "owner-repo"},
		{"https://github.com/owner/repo.git", "owner-repo"},
		{"simple", "simple"},
		{"UPPER-Case", "upper-case"},
		// Edge cases
		{"", ""},
		{"https://github.com", "github.com"},
		{"ssh://git@host:22/owner/repo.git", "owner-repo"},
		{"my-slug", "my-slug"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, config.NormalizeSlug(tt.input))
		})
	}
}
