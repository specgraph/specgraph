// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package service manages specgraph as a user-level OS service.
// On macOS it uses launchd; on Linux it uses systemd.
package service

// Config holds the parameters needed to generate and install the service definition.
type Config struct {
	BinaryPath string // absolute path to specgraph binary
	ConfigPath string // path to config.yaml
	LogPath    string // path to server.log
}

// Generate writes the service definition file to dir and returns the path to the generated file.
func Generate(dir string, cfg Config) (string, error) {
	return generate(dir, cfg)
}

// Install loads/enables the service so the OS manages it.
func Install(defPath string) error {
	return install(defPath)
}

// Uninstall stops and removes the service.
func Uninstall(defPath string) error {
	return uninstall(defPath)
}

// Stop stops the service without removing it.
func Stop() error {
	return stop()
}

// IsInstalled returns true if the service definition exists in the standard OS location.
func IsInstalled() bool {
	return isInstalled()
}
