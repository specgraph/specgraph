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
