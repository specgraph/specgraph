// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/config/managedfiles"
)

// initProjectWithStaleFile sets up a real synced project in tmp via
// SyncAll, then corrupts AGENTS.md (a MarkdownBlock file) so it appears
// as Drifted. Returns the project root.
func initProjectWithStaleFile(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	pc := &config.ProjectConfig{Slug: "nudge-test"}
	if err := config.WriteProject(tmp, pc); err != nil {
		t.Fatalf("WriteProject: %v", err)
	}
	params := managedfiles.ProjectParams{Slug: pc.Slug, ServerURL: "http://127.0.0.1:9090"}
	harnesses := []managedfiles.Harness{
		managedfiles.HarnessClaude,
		managedfiles.HarnessCursor,
		managedfiles.HarnessOpenCode,
	}
	if _, err := managedfiles.SyncAll(tmp, harnesses, params, managedfiles.SyncOptions{}); err != nil {
		t.Fatalf("SyncAll: %v", err)
	}
	// Corrupt AGENTS.md so its sentinel-recorded hash no longer matches
	// disk content. This produces a Drifted classification.
	agentsPath := filepath.Join(tmp, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# corrupted by test\n"), 0o600); err != nil {
		t.Fatalf("corrupt AGENTS.md: %v", err)
	}
	return tmp
}

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

// TestNudge_EmitsWhenStaleFilesPresent pins the emit path that all
// prior nudge tests only exercise by skip. Builds a synced project,
// corrupts a managed file so it classifies as Drifted, stubs the
// TTY check so the function continues past gate 2, and asserts the
// expected stderr line.
func TestNudge_EmitsWhenStaleFilesPresent(t *testing.T) {
	tmp := initProjectWithStaleFile(t)

	// Isolated XDG roots so the test doesn't depend on the dev's home
	// dir and doesn't leak throttle files.
	cacheDir := t.TempDir()
	cfgDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	t.Setenv("SPECGRAPH_DRIFT_NUDGE", "")

	// Stub the TTY check so we actually reach the inspect path.
	origTTY := nudgeIsTerminal
	nudgeIsTerminal = func() bool { return true }
	t.Cleanup(func() { nudgeIsTerminal = origTTY })

	oldwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := &cobra.Command{Use: "nudge-test-emit"}
	rootCmd.AddCommand(cmd)
	t.Cleanup(func() { rootCmd.RemoveCommand(cmd) })

	out := captureStderr(t, func() {
		if err := nudgePreRun(cmd, nil); err != nil {
			t.Errorf("nudgePreRun returned error: %v", err)
		}
	})

	if !strings.Contains(out, "managed files out of date") {
		t.Errorf("expected emit line on stderr, got %q", out)
	}
	if !strings.Contains(out, "drifted") {
		t.Errorf("expected drifted count in emit line, got %q", out)
	}
}

// TestNudge_SkippedByNudgesQuiet pins the .specgraph.yaml `nudges:
// quiet: true` skip gate. With a stale file present AND the TTY stub
// in place, the quiet flag must still suppress stderr output.
func TestNudge_SkippedByNudgesQuiet(t *testing.T) {
	tmp := initProjectWithStaleFile(t)

	// Overwrite .specgraph.yaml with nudges.quiet=true.
	pc := &config.ProjectConfig{Slug: "nudge-test", Nudges: config.Nudges{Quiet: true}}
	if err := config.WriteProject(tmp, pc); err != nil {
		t.Fatalf("WriteProject quiet: %v", err)
	}

	cacheDir := t.TempDir()
	cfgDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	t.Setenv("XDG_CONFIG_HOME", cfgDir)
	t.Setenv("SPECGRAPH_DRIFT_NUDGE", "")

	origTTY := nudgeIsTerminal
	nudgeIsTerminal = func() bool { return true }
	t.Cleanup(func() { nudgeIsTerminal = origTTY })

	oldwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cmd := &cobra.Command{Use: "nudge-test-quiet"}
	rootCmd.AddCommand(cmd)
	t.Cleanup(func() { rootCmd.RemoveCommand(cmd) })

	out := captureStderr(t, func() {
		if err := nudgePreRun(cmd, nil); err != nil {
			t.Errorf("nudgePreRun returned error: %v", err)
		}
	})

	if out != "" {
		t.Errorf("expected no stderr with nudges.quiet=true, got %q", out)
	}
}

// TestRunDoctorFix_ReinspectAfterSync pins the --fix loop: a Stale
// file is re-synced via Sync, and a subsequent runManagedGroup classifies
// it as Synced. Drifted files are left alone (only guidance printed).
func TestRunDoctorFix_ReinspectAfterSync(t *testing.T) {
	tmp := initProjectWithStaleFile(t)
	params := managedfiles.ProjectParams{Slug: "nudge-test", ServerURL: "http://127.0.0.1:9090"}
	harnesses := []managedfiles.Harness{
		managedfiles.HarnessClaude,
		managedfiles.HarnessCursor,
		managedfiles.HarnessOpenCode,
	}

	// Pre-fix: corrupt a different file to Stale-like state by deleting
	// it. Missing files are also handled by runDoctorFix (re-created).
	mcpPath := filepath.Join(tmp, ".mcp.json")
	if err := os.Remove(mcpPath); err != nil {
		t.Fatalf("remove .mcp.json: %v", err)
	}

	pre := runManagedGroup(tmp, harnesses, params)
	if pre.OK {
		t.Fatalf("pre-fix expected non-OK report, got %+v", pre)
	}

	// Redirect stdout because runDoctorFix prints guidance lines for the
	// AGENTS.md drifted file.
	origStdout := os.Stdout
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	t.Cleanup(func() {
		os.Stdout = origStdout
		_ = devNull.Close()
	})

	if err := runDoctorFix(tmp, pre, harnesses, params); err != nil {
		t.Fatalf("runDoctorFix: %v", err)
	}

	// Post-fix: .mcp.json must be back; AGENTS.md is drifted, untouched.
	post := runManagedGroup(tmp, harnesses, params)
	mcpState := managedfiles.StateMissing
	for _, f := range post.Files {
		if f.Path == ".mcp.json" {
			mcpState = f.State
		}
	}
	if mcpState != managedfiles.StateSynced {
		t.Errorf(".mcp.json post-fix state = %v, want Synced", mcpState)
	}
}
