// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package xdg

import (
	"os"
	"path/filepath"
)

const appName = "specgraph"

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

// ConfigHome returns XDG_CONFIG_HOME/specgraph or ~/.config/specgraph.
func ConfigHome() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, appName)
	}
	return filepath.Join(homeDir(), ".config", appName)
}

// DataHome returns XDG_DATA_HOME/specgraph or ~/.local/share/specgraph.
func DataHome() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, appName)
	}
	return filepath.Join(homeDir(), ".local", "share", appName)
}

// StateHome returns XDG_STATE_HOME/specgraph or ~/.local/state/specgraph.
func StateHome() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return filepath.Join(v, appName)
	}
	return filepath.Join(homeDir(), ".local", "state", appName)
}

// ConfigFile returns the path to the global config file.
func ConfigFile() string {
	return filepath.Join(ConfigHome(), "config.yaml")
}

// EnsureDirs creates all XDG directories if they don't exist.
func EnsureDirs() error {
	for _, dir := range []string{ConfigHome(), DataHome(), StateHome()} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	}
	return nil
}
