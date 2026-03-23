// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build darwin

package service

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// escapedConfig holds XML-safe copies of Config string fields for plist rendering.
type escapedConfig struct {
	BinaryPath string
	ConfigPath string
	LogPath    string
}

// xmlEscape returns s with XML special characters escaped for safe plist embedding.
func xmlEscape(s string) string {
	var buf strings.Builder
	xml.EscapeText(&buf, []byte(s)) //nolint:errcheck // strings.Builder.Write never returns an error
	return buf.String()
}

func generate(dir string, cfg Config) (string, error) {
	if !filepath.IsAbs(cfg.BinaryPath) {
		return "", fmt.Errorf("binary path must be absolute: %s", cfg.BinaryPath)
	}
	path := filepath.Join(dir, launchdFilename)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create plist file: %w", err)
	}
	defer f.Close() //nolint:errcheck // best-effort close; write errors caught by Execute
	escaped := escapedConfig{
		BinaryPath: xmlEscape(cfg.BinaryPath),
		ConfigPath: xmlEscape(cfg.ConfigPath),
		LogPath:    xmlEscape(cfg.LogPath),
	}
	if err := plistTmpl.Execute(f, escaped); err != nil {
		return "", fmt.Errorf("render plist template: %w", err)
	}
	return path, nil
}

func install(defPath string) error {
	uid := os.Getuid()

	// If the service is already bootstrapped (e.g., from a previous run),
	// bootout first — launchctl bootstrap fails with exit 5 if the label
	// is already loaded.
	service := fmt.Sprintf("gui/%d/%s", uid, launchdLabel)
	bootout := exec.Command("launchctl", "bootout", service)
	_ = bootout.Run() // intentionally ignore error: service may not be loaded

	target := fmt.Sprintf("gui/%d", uid)
	cmd := exec.Command("launchctl", "bootstrap", target, defPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %w: %s", err, out)
	}
	return nil
}

func uninstall(defPath string) error {
	// Best-effort stop: launchctl bootout fails if the service isn't loaded,
	// which is non-fatal since the goal is to remove the plist file.
	stop() //nolint:errcheck // intentionally non-fatal; service may not be loaded
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
