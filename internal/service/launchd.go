// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build darwin

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const (
	launchdLabel    = "com.specgraph.server"
	launchdFilename = "com.specgraph.server.plist"
)

var plistTmpl = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.specgraph.server</string>
  <key>ProgramArguments</key>
  <array>
    <string>{{.BinaryPath}}</string>
    <string>serve</string>
  </array>
  <key>KeepAlive</key>
  <true/>
  <key>RunAtLoad</key>
  <true/>
  <key>StandardOutPath</key>
  <string>{{.LogPath}}</string>
  <key>StandardErrorPath</key>
  <string>{{.LogPath}}</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>SPECGRAPH_CONFIG</key>
    <string>{{.ConfigPath}}</string>
  </dict>
</dict>
</plist>
`))

func generate(dir string, cfg Config) (string, error) {
	path := filepath.Join(dir, launchdFilename)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create plist file: %w", err)
	}
	defer func() { _ = f.Close() }()
	if err := plistTmpl.Execute(f, cfg); err != nil {
		return "", fmt.Errorf("render plist template: %w", err)
	}
	return path, nil
}

func install(defPath string) error {
	uid := os.Getuid()
	target := fmt.Sprintf("gui/%d", uid)
	cmd := exec.Command("launchctl", "bootstrap", target, defPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %w: %s", err, out)
	}
	return nil
}

func uninstall(defPath string) error {
	if err := stop(); err != nil {
		return err
	}
	if err := os.Remove(defPath); err != nil {
		return fmt.Errorf("remove plist file: %w", err)
	}
	return nil
}

func stop() error {
	uid := os.Getuid()
	service := fmt.Sprintf("gui/%d/%s", uid, launchdLabel)
	cmd := exec.Command("launchctl", "bootout", service)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootout: %w: %s", err, out)
	}
	return nil
}

func isInstalled() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	path := filepath.Join(home, "Library", "LaunchAgents", launchdFilename)
	_, err = os.Stat(path)
	return err == nil
}
