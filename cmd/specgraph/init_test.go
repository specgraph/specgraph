// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/seanb4t/specgraph/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withStdin replaces os.Stdin with a pipe containing the given input for the
// duration of the test.
func withStdin(t *testing.T, input string) {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	_, err = w.WriteString(input)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })
}

func withCfgFile(t *testing.T, path string) {
	t.Helper()
	old := cfgFile
	cfgFile = path
	t.Cleanup(func() { cfgFile = old })
}

func TestInitNonInteractive(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".specgraph", "config.yaml")

	withCfgFile(t, configPath)

	initYes = true
	err := runInit(nil, nil)
	require.NoError(t, err)

	_, err = os.Stat(configPath)
	require.NoError(t, err)

	cfg, err := config.Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "memgraph", cfg.Storage.Backend)
	assert.Equal(t, "docker", cfg.Server.Mode)
}

func TestInitAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	require.NoError(t, os.WriteFile(configPath, []byte("existing"), 0o600))
	withCfgFile(t, configPath)

	initYes = true
	err := runInit(nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "config already exists")
}

func TestInitInteractiveDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".specgraph", "config.yaml")

	withCfgFile(t, configPath)
	// All empty lines → accept defaults
	withStdin(t, "\n\n")

	initYes = false
	err := runInit(nil, nil)
	require.NoError(t, err)

	cfg, err := config.Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "memgraph", cfg.Storage.Backend)
	assert.Equal(t, "docker", cfg.Server.Mode)
}

func TestInitInteractivePostgresExternal(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".specgraph", "config.yaml")

	withCfgFile(t, configPath)
	// Select postgres backend, external mode, provide URL
	withStdin(t, "postgres\nexternal\npostgres://user:pass@host/db\n")

	initYes = false
	err := runInit(nil, nil)
	require.NoError(t, err)

	cfg, err := config.Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "postgres", cfg.Storage.Backend)
	assert.Equal(t, "external", cfg.Server.Mode)
	assert.Equal(t, "postgres://user:pass@host/db", cfg.Storage.Postgres.URL)
}

func TestInitInteractiveMemgraphExternal(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".specgraph", "config.yaml")

	withCfgFile(t, configPath)
	// Select memgraph backend, external mode, provide custom bolt URI
	withStdin(t, "memgraph\nexternal\nbolt://custom:7688\n")

	initYes = false
	err := runInit(nil, nil)
	require.NoError(t, err)

	cfg, err := config.Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "memgraph", cfg.Storage.Backend)
	assert.Equal(t, "external", cfg.Server.Mode)
	assert.Equal(t, "bolt://custom:7688", cfg.Storage.Memgraph.BoltURI)
}

func TestInitMkdirAllFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod not effective on Windows")
	}

	dir := t.TempDir()
	// Create a read-only parent so MkdirAll fails
	readOnlyDir := filepath.Join(dir, "readonly")
	require.NoError(t, os.MkdirAll(readOnlyDir, 0o750))
	require.NoError(t, os.Chmod(readOnlyDir, 0o444))   //nolint:gosec // intentionally restrictive for test
	t.Cleanup(func() { os.Chmod(readOnlyDir, 0o750) }) //nolint:gosec // restore perms

	configPath := filepath.Join(readOnlyDir, "subdir", "config.yaml")
	withCfgFile(t, configPath)

	initYes = true
	err := runInit(nil, nil)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "creating config directory"),
		"expected MkdirAll error, got: %v", err)
}

func TestInitWriteFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod not effective on Windows")
	}

	dir := t.TempDir()
	// Create config directory as read-only so Write fails
	cfgDir := filepath.Join(dir, ".specgraph")
	require.NoError(t, os.MkdirAll(cfgDir, 0o750))
	require.NoError(t, os.Chmod(cfgDir, 0o444))   //nolint:gosec // intentionally restrictive for test
	t.Cleanup(func() { os.Chmod(cfgDir, 0o750) }) //nolint:gosec // restore perms

	configPath := filepath.Join(cfgDir, "config.yaml")
	withCfgFile(t, configPath)

	initYes = true
	err := runInit(nil, nil)
	require.Error(t, err)
}
