// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package config_test

import (
	"os"
	"path/filepath"
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
