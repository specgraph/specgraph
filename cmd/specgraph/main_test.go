// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"path/filepath"
	"testing"

	"github.com/specgraph/specgraph/internal/xdg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobalConfigPath_DefaultFallsBackToXDG(t *testing.T) {
	old := cfgFile
	cfgFile = ""
	t.Cleanup(func() { cfgFile = old })

	assert.Equal(t, xdg.ConfigFile(), globalConfigPath())
}

func TestGlobalConfigPath_FlagOverride(t *testing.T) {
	old := cfgFile
	cfgFile = "/etc/specgraph/config.yaml"
	t.Cleanup(func() { cfgFile = old })

	assert.Equal(t, "/etc/specgraph/config.yaml", globalConfigPath())
}

func TestLegacyConfigPath_DefaultIsRepoLocal(t *testing.T) {
	old := cfgFile
	cfgFile = ""
	t.Cleanup(func() { cfgFile = old })

	assert.Equal(t, ".specgraph/config.yaml", legacyConfigPath())
}

func TestLegacyConfigPath_FlagOverride(t *testing.T) {
	old := cfgFile
	cfgFile = "/tmp/custom.yaml"
	t.Cleanup(func() { cfgFile = old })

	assert.Equal(t, "/tmp/custom.yaml", legacyConfigPath())
}

func TestLoadGlobalCfg_ExplicitMissingPathErrors(t *testing.T) {
	old := cfgFile
	t.Cleanup(func() { cfgFile = old })
	cfgFile = filepath.Join(t.TempDir(), "missing.yaml")

	_, err := loadGlobalCfg()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
}
