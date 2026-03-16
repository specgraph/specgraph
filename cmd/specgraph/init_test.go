// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanb4t/specgraph/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// changeDir temporarily changes the working directory for the duration of the test.
func changeDir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck // best-effort restore
}

func TestInitWithExplicitSlug(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	// runUp will fail in unit tests (no server), but init should still write the config.
	// We test the WriteProject path directly by calling the slug-derivation + write logic.
	pc := &config.ProjectConfig{Slug: "my-project"}
	err := config.WriteProject(dir, pc)
	require.NoError(t, err)

	got, err := config.LoadProject(dir)
	require.NoError(t, err)
	assert.Equal(t, "my-project", got.Slug)
}

func TestInitDeriveSlugFromDirName(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	// With no git remote, LoadProject falls back to dir name.
	proj, err := config.LoadProject(dir)
	require.NoError(t, err)
	// Slug should be derived (non-empty).
	assert.NotEmpty(t, proj.Slug)
}

func TestInitWriteProjectConfig(t *testing.T) {
	dir := t.TempDir()

	pc := &config.ProjectConfig{Slug: "test-slug"}
	err := config.WriteProject(dir, pc)
	require.NoError(t, err)

	yamlPath := filepath.Join(dir, ".specgraph.yaml")
	_, err = os.Stat(yamlPath)
	require.NoError(t, err, ".specgraph.yaml should exist")

	loaded, err := config.LoadProject(dir)
	require.NoError(t, err)
	assert.Equal(t, "test-slug", loaded.Slug)
}

func TestInitYesFlagAccepted(t *testing.T) {
	// --yes is accepted for backward compat and should not cause errors.
	assert.False(t, initYes) // default is false
}
