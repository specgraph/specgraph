// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"testing"

	"github.com/specgraph/specgraph/internal/xdg"
	"github.com/stretchr/testify/assert"
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
