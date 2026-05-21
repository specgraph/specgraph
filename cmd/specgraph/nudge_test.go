// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// captureStderr runs fn with os.Stderr piped to a bytes buffer and
// returns the captured output. Restores os.Stderr regardless of fn's
// outcome.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

func TestNudge_SkippedByAllowList(t *testing.T) {
	t.Setenv("SPECGRAPH_DRIFT_NUDGE", "") // ensure not off
	cmd := &cobra.Command{Use: "init"}
	rootCmd.AddCommand(cmd)
	t.Cleanup(func() { rootCmd.RemoveCommand(cmd) })

	out := captureStderr(t, func() {
		if err := nudgePreRun(cmd, nil); err != nil {
			t.Errorf("init allow-list hit returned error: %v", err)
		}
	})
	if out != "" {
		t.Errorf("expected no stderr for allow-list skip, got %q", out)
	}
}

func TestNudge_SkippedByEnvVar(t *testing.T) {
	t.Setenv("SPECGRAPH_DRIFT_NUDGE", "off")
	cmd := &cobra.Command{Use: "nudge-test-list"}
	rootCmd.AddCommand(cmd)
	t.Cleanup(func() { rootCmd.RemoveCommand(cmd) })

	out := captureStderr(t, func() {
		if err := nudgePreRun(cmd, nil); err != nil {
			t.Errorf("env-off returned error: %v", err)
		}
	})
	if out != "" {
		t.Errorf("expected no stderr when env=off, got %q", out)
	}
}

func TestNudge_SkippedWhenNoProject(t *testing.T) {
	// Use a fresh XDG_CACHE_HOME so any side effects stay isolated.
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	t.Setenv("SPECGRAPH_DRIFT_NUDGE", "")

	tmp := t.TempDir()
	oldwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := &cobra.Command{Use: "nudge-test-noproj"}
	rootCmd.AddCommand(cmd)
	t.Cleanup(func() { rootCmd.RemoveCommand(cmd) })

	out := captureStderr(t, func() {
		if err := nudgePreRun(cmd, nil); err != nil {
			t.Errorf("no-project case returned error: %v", err)
		}
	})
	if out != "" {
		t.Errorf("expected no stderr when no project found, got %q", out)
	}
	// No nudge file should have been created (the throttle gate is past
	// the project-root gate).
	nudgesDir := filepath.Join(cacheDir, "specgraph", "nudges")
	if entries, err := os.ReadDir(nudgesDir); err == nil && len(entries) > 0 {
		t.Errorf("unexpected throttle file(s) under %s: %v", nudgesDir, entries)
	}
}

func TestNudge_ThrottledWithinWindow(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	root := t.TempDir()
	path := throttleFilePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if shouldEmitAfterThrottle(root) {
		t.Error("fresh throttle file should suppress emit")
	}
}

func TestNudge_EmittedAfterWindow(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	root := t.TempDir()
	path := throttleFilePath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	old := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if !shouldEmitAfterThrottle(root) {
		t.Error("expired throttle file should permit emit")
	}
}

func TestNudge_GarbageCollectsOldEntries(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	nudgesDir := filepath.Join(cacheDir, "specgraph", "nudges")
	if err := os.MkdirAll(nudgesDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	oldPath := filepath.Join(nudgesDir, "old-entry")
	freshPath := filepath.Join(nudgesDir, "fresh-entry")
	if err := os.WriteFile(oldPath, nil, 0o600); err != nil {
		t.Fatalf("write old: %v", err)
	}
	if err := os.WriteFile(freshPath, nil, 0o600); err != nil {
		t.Fatalf("write fresh: %v", err)
	}
	veryOld := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(oldPath, veryOld, veryOld); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	gcOldNudgeFiles()

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Errorf("old entry not GC'd: %v", err)
	}
	if _, err := os.Stat(freshPath); err != nil {
		t.Errorf("fresh entry incorrectly GC'd: %v", err)
	}
}
