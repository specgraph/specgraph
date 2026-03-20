// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package service_test

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/service"
)

func TestGenerate(t *testing.T) {
	cfg := service.Config{
		BinaryPath: "/usr/local/bin/specgraph",
		ConfigPath: "/home/user/.specgraph/config.yaml",
		LogPath:    "/home/user/.specgraph/server.log",
	}

	dir := t.TempDir()
	path, err := service.Generate(dir, cfg)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if path == "" {
		t.Fatal("Generate() returned empty path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	content := string(data)

	if !strings.Contains(content, cfg.BinaryPath) {
		t.Errorf("generated file missing BinaryPath %q", cfg.BinaryPath)
	}
	if !strings.Contains(content, cfg.ConfigPath) {
		t.Errorf("generated file missing ConfigPath %q", cfg.ConfigPath)
	}

	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(content, cfg.LogPath) {
			t.Errorf("generated plist missing LogPath %q", cfg.LogPath)
		}
		if !strings.HasSuffix(path, ".plist") {
			t.Errorf("expected .plist extension, got %q", path)
		}
		if !strings.Contains(content, "com.specgraph.server") {
			t.Error("generated plist missing label com.specgraph.server")
		}
		if !strings.Contains(content, "<key>KeepAlive</key>") {
			t.Error("generated plist missing KeepAlive key")
		}
	case "linux":
		if !strings.HasSuffix(path, ".service") {
			t.Errorf("expected .service extension, got %q", path)
		}
		if !strings.Contains(content, "SpecGraph Development Server") {
			t.Error("generated unit file missing description")
		}
		if !strings.Contains(content, "WantedBy=default.target") {
			t.Error("generated unit file missing WantedBy directive")
		}
	default:
		t.Skipf("unsupported platform: %s", runtime.GOOS)
	}
}

func TestGenerateDistinctOutputPerCall(t *testing.T) {
	cfg1 := service.Config{
		BinaryPath: "/usr/local/bin/specgraph",
		ConfigPath: "/home/user1/config.yaml",
		LogPath:    "/home/user1/server.log",
	}
	cfg2 := service.Config{
		BinaryPath: "/opt/specgraph/bin/specgraph",
		ConfigPath: "/home/user2/config.yaml",
		LogPath:    "/home/user2/server.log",
	}

	dir1 := t.TempDir()
	dir2 := t.TempDir()

	path1, err := service.Generate(dir1, cfg1)
	if err != nil {
		t.Fatalf("Generate() cfg1 error = %v", err)
	}
	path2, err := service.Generate(dir2, cfg2)
	if err != nil {
		t.Fatalf("Generate() cfg2 error = %v", err)
	}

	data1, err := os.ReadFile(path1)
	if err != nil {
		t.Fatalf("ReadFile() path1 error = %v", err)
	}
	data2, err := os.ReadFile(path2)
	if err != nil {
		t.Fatalf("ReadFile() path2 error = %v", err)
	}

	if strings.Contains(string(data1), cfg2.BinaryPath) {
		t.Error("file1 unexpectedly contains cfg2 BinaryPath")
	}
	if strings.Contains(string(data2), cfg1.BinaryPath) {
		t.Error("file2 unexpectedly contains cfg1 BinaryPath")
	}
}

func TestGenerate_EmptyBinaryPath(t *testing.T) {
	cfg := service.Config{
		BinaryPath: "",
		ConfigPath: "/home/user/.config/specgraph/config.yaml",
		LogPath:    "/home/user/.local/state/specgraph/server.log",
	}

	dir := t.TempDir()
	_, err := service.Generate(dir, cfg)
	// Both platforms validate BinaryPath is absolute.
	if err == nil {
		t.Fatal("Generate() with empty BinaryPath should return an error")
	}
}

func TestGenerate_NonExistentDestDir(t *testing.T) {
	cfg := service.Config{
		BinaryPath: "/usr/local/bin/specgraph",
		ConfigPath: "/home/user/.config/specgraph/config.yaml",
		LogPath:    "/home/user/.local/state/specgraph/server.log",
	}

	nonExistent := "/tmp/specgraph-test-nonexistent-dir-xyz/subdir"
	// Ensure the directory doesn't exist.
	_ = os.RemoveAll(nonExistent)

	_, err := service.Generate(nonExistent, cfg)
	if err == nil {
		t.Fatal("Generate() with non-existent destDir: expected error, got nil")
	}
}
