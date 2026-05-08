// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package xdg provides XDG Base Directory paths for specgraph configuration, data, and state.
package xdg

import (
	"fmt"
	"os"
	"path/filepath"
)

const appName = "specgraph"

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
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

// CacheHome returns XDG_CACHE_HOME/specgraph or ~/.cache/specgraph.
// Cache holds non-essential data that can be regenerated on demand
// (e.g. drift-nudge throttle files for `specgraph doctor`).
//
// Not pre-created — callers create lazily at use site per XDG convention
// for cache directories. Removing the cache dir externally is always safe;
// the next caller recreates it.
func CacheHome() string {
	if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
		return filepath.Join(v, appName)
	}
	return filepath.Join(homeDir(), ".cache", appName)
}

// ConfigFile returns the path to the global config file.
func ConfigFile() string {
	return filepath.Join(ConfigHome(), "config.yaml")
}

// CredentialsFile returns the path to the credentials file.
func CredentialsFile() string {
	return filepath.Join(ConfigHome(), "credentials.yaml")
}

// OAuthTokenFile returns the path to the cached OAuth token file.
func OAuthTokenFile() string {
	return filepath.Join(ConfigHome(), "oauth_token.json")
}

// EnsureDirs creates all XDG directories if they don't exist.
// Returns an error if any path is relative (e.g., $HOME is unset).
func EnsureDirs() error {
	for _, dir := range []string{ConfigHome(), DataHome(), StateHome()} {
		if !filepath.IsAbs(dir) {
			return fmt.Errorf("xdg: refusing to create relative directory %q (is $HOME set?)", dir)
		}
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}
	return nil
}
