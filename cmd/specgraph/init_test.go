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

func TestInitNonInteractive(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".specgraph", "config.yaml")

	// Override global cfgFile for this test
	oldCfgFile := cfgFile
	cfgFile = configPath
	t.Cleanup(func() { cfgFile = oldCfgFile })

	initYes = true
	err := runInit(nil, nil)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Load and verify contents
	cfg, err := config.Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "memgraph", cfg.Storage.Backend)
	assert.Equal(t, "docker", cfg.Server.Mode)
}

func TestRunConstitutionScanWithGoMod(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".specgraph")
	require.NoError(t, os.MkdirAll(configDir, 0o750))
	configPath := filepath.Join(configDir, "config.yaml")

	// Create a go.mod so the scanner detects Go as the primary language
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0o600))

	// runConstitutionScan uses scanner.Scan(".") so we must chdir
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(oldWd) }) //nolint:errcheck // best-effort restore in test cleanup

	err = runConstitutionScan(configPath)
	require.NoError(t, err)

	constitutionPath := filepath.Join(configDir, "constitution.yaml")
	c, err := config.LoadConstitutionYAML(constitutionPath)
	require.NoError(t, err)
	assert.Equal(t, "project", c.Name)
	assert.Equal(t, "project", c.Layer)
	assert.Equal(t, "go", c.Tech.Languages.Primary)
}

func TestRunConstitutionScanEmptyDir(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".specgraph")
	require.NoError(t, os.MkdirAll(configDir, 0o750))
	configPath := filepath.Join(configDir, "config.yaml")

	// Empty directory — no language markers
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(oldWd) }) //nolint:errcheck // best-effort restore in test cleanup

	err = runConstitutionScan(configPath)
	require.NoError(t, err)

	constitutionPath := filepath.Join(configDir, "constitution.yaml")
	c, err := config.LoadConstitutionYAML(constitutionPath)
	require.NoError(t, err)
	assert.Equal(t, "project", c.Name)
	assert.Equal(t, "project", c.Layer)
	assert.Empty(t, c.Tech.Languages.Primary)
}

func TestInitWithScanFlag(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".specgraph", "config.yaml")

	// Create a go.mod so the scan has something to detect
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0o600))

	oldCfgFile := cfgFile
	cfgFile = configPath
	t.Cleanup(func() { cfgFile = oldCfgFile })

	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(oldWd) }) //nolint:errcheck // best-effort restore in test cleanup

	initYes = true
	initScan = true
	t.Cleanup(func() { initScan = false; initYes = false })

	err = runInit(nil, nil)
	require.NoError(t, err)

	// Verify config was created
	cfg, err := config.Load(configPath)
	require.NoError(t, err)
	assert.Equal(t, "memgraph", cfg.Storage.Backend)

	// Verify constitution was created by the scan
	constitutionPath := filepath.Join(dir, ".specgraph", "constitution.yaml")
	c, err := config.LoadConstitutionYAML(constitutionPath)
	require.NoError(t, err)
	assert.Equal(t, "go", c.Tech.Languages.Primary)
}

func TestInitAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	// Create existing file
	require.NoError(t, os.WriteFile(configPath, []byte("existing"), 0o600))

	oldCfgFile := cfgFile
	cfgFile = configPath
	t.Cleanup(func() { cfgFile = oldCfgFile })

	initYes = true
	err := runInit(nil, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "config already exists")
}
