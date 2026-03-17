// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build linux

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// shellMetachars are characters that could cause shell injection in systemd ExecStart values.
const shellMetachars = "|&;<>`$\"'\\!{}()*?#~"

func generate(dir string, cfg Config) (string, error) {
	if !filepath.IsAbs(cfg.BinaryPath) {
		return "", fmt.Errorf("binary path must be absolute: %s", cfg.BinaryPath)
	}
	if strings.ContainsAny(cfg.BinaryPath, shellMetachars) {
		return "", fmt.Errorf("binary path contains shell metacharacters: %s", cfg.BinaryPath)
	}
	path := filepath.Join(dir, systemdFilename)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create unit file: %w", err)
	}
	defer f.Close() //nolint:errcheck // best-effort close on write path
	if err := unitTmpl.Execute(f, cfg); err != nil {
		return "", fmt.Errorf("render unit template: %w", err)
	}
	return path, nil
}

// install enables the systemd user service. defPath is unused because
// systemctl operates by unit name, not file path.
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
	if err := os.Remove(defPath); err != nil {
		return fmt.Errorf("remove unit file: %w", err)
	}
	reload := exec.Command("systemctl", "--user", "daemon-reload")
	if out, err := reload.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w: %s", err, out)
	}
	return nil
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
