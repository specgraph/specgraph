// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build linux

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const (
	systemdUnit     = "specgraph"
	systemdFilename = "specgraph.service"
)

var unitTmpl = template.Must(template.New("unit").Parse(`[Unit]
Description=SpecGraph Development Server
After=docker.service

[Service]
Type=exec
ExecStart={{.BinaryPath}} serve
Restart=on-failure
RestartSec=5
Environment=SPECGRAPH_CONFIG={{.ConfigPath}}

[Install]
WantedBy=default.target
`))

func generate(dir string, cfg Config) (string, error) {
	path := filepath.Join(dir, systemdFilename)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create unit file: %w", err)
	}
	defer f.Close()
	if err := unitTmpl.Execute(f, cfg); err != nil {
		return "", fmt.Errorf("render unit template: %w", err)
	}
	return path, nil
}

func install(_ string) error {
	cmd := exec.Command("systemctl", "--user", "enable", "--now", systemdUnit)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl enable: %w: %s", err, out)
	}
	return nil
}

func uninstall(defPath string) error {
	cmd := exec.Command("systemctl", "--user", "disable", "--now", systemdUnit)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl disable: %w: %s", err, out)
	}
	return os.Remove(defPath)
}

func stop() error {
	cmd := exec.Command("systemctl", "--user", "stop", systemdUnit)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl stop: %w: %s", err, out)
	}
	return nil
}

func isInstalled() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	path := filepath.Join(home, ".config", "systemd", "user", systemdFilename)
	_, err = os.Stat(path)
	return err == nil
}
