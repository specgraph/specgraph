// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runInitInDir executes runInit with the given args in workDir, capturing
// stdout. Isolates the global config file (cfgFile package var) and CWD,
// restoring them on cleanup.
func runInitInDir(t *testing.T, workDir string, args []string) (stdout string, err error) {
	t.Helper()

	// Isolate cfgFile so loadGlobalCfg() doesn't touch the developer's
	// real ~/.config/specgraph/config.yaml. Mirrors the pattern in
	// lifecycle_test.go and other tests in this package.
	//
	// We write an empty YAML file rather than just naming a path: when
	// cfgFile != "" loadGlobalCfg dispatches to config.LoadGlobalExplicit,
	// which errors with "config file not found" if the path doesn't exist.
	// An empty YAML body parses to globalDefaults() (DefaultServer
	// http://127.0.0.1:9090), which is fine for these tests.
	oldCfgFile := cfgFile
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if writeErr := os.WriteFile(cfgPath, []byte(""), 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	cfgFile = cfgPath
	t.Cleanup(func() { cfgFile = oldCfgFile })

	origDir, dirErr := os.Getwd()
	if dirErr != nil {
		t.Fatal(dirErr)
	}
	if err := os.Chdir(workDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Capture stdout. Restore inline before reading from the pipe so
	// downstream test helpers see normal stdout.
	origStdout := os.Stdout
	r, w, perr := os.Pipe()
	if perr != nil {
		t.Fatal(perr)
	}
	os.Stdout = w

	runErr := runInit(initCmd, args)

	_ = w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, copyErr := buf.ReadFrom(r); copyErr != nil {
		t.Fatal(copyErr)
	}

	return buf.String(), runErr
}

func TestRunInit_FreshProject_NoArg(t *testing.T) {
	dir := t.TempDir()
	out, err := runInitInDir(t, dir, nil)
	if err != nil {
		t.Fatalf("runInit: %v", err)
	}
	// .specgraph.yaml exists.
	if _, statErr := os.Stat(filepath.Join(dir, ".specgraph.yaml")); statErr != nil {
		t.Errorf(".specgraph.yaml not created: %v", statErr)
	}
	// All three configs exist.
	for _, p := range []string{".cursor/mcp.json", ".mcp.json", "opencode.json"} {
		if _, statErr := os.Stat(filepath.Join(dir, p)); statErr != nil {
			t.Errorf("%s not created: %v", p, statErr)
		}
		if !strings.Contains(out, p+": created") {
			t.Errorf("stdout missing %q: %s", p+": created", out)
		}
	}
}

func TestRunInit_FreshProject_WithArg(t *testing.T) {
	dir := t.TempDir()
	out, err := runInitInDir(t, dir, []string{"explicit-slug"})
	if err != nil {
		t.Fatalf("runInit: %v", err)
	}
	// .specgraph.yaml has the explicit slug.
	data, readErr := os.ReadFile(filepath.Join(dir, ".specgraph.yaml"))
	if readErr != nil {
		t.Fatalf("read .specgraph.yaml: %v", readErr)
	}
	if !strings.Contains(string(data), "explicit-slug") {
		t.Errorf(".specgraph.yaml missing slug; content: %s", data)
	}
	if !strings.Contains(out, "Initialized project explicit-slug") {
		t.Errorf("stdout missing init message: %s", out)
	}
}

func TestRunInit_ExistingProject_NoArg_NoOp(t *testing.T) {
	dir := t.TempDir()
	if _, err := runInitInDir(t, dir, []string{"my-project"}); err != nil {
		t.Fatalf("first runInit: %v", err)
	}

	out, err := runInitInDir(t, dir, nil)
	if err != nil {
		t.Fatalf("second runInit: %v", err)
	}
	// All three configs report no-op on the second run.
	for _, p := range []string{".cursor/mcp.json", ".mcp.json", "opencode.json"} {
		if !strings.Contains(out, p+": no-op") {
			t.Errorf("stdout missing %q on idempotent re-run: %s", p+": no-op", out)
		}
	}
	// Should NOT print the "Initialized project" message on the second run.
	if strings.Contains(out, "Initialized project") {
		t.Errorf("stdout has unexpected init message on re-run: %s", out)
	}
}

func TestRunInit_ExistingProject_MatchingArg_NoOp(t *testing.T) {
	dir := t.TempDir()
	if _, err := runInitInDir(t, dir, []string{"my-project"}); err != nil {
		t.Fatalf("first runInit: %v", err)
	}

	out, err := runInitInDir(t, dir, []string{"my-project"})
	if err != nil {
		t.Fatalf("second runInit with matching arg: %v", err)
	}
	for _, p := range []string{".cursor/mcp.json", ".mcp.json", "opencode.json"} {
		if !strings.Contains(out, p+": no-op") {
			t.Errorf("stdout missing %q: %s", p+": no-op", out)
		}
	}
}

func TestRunInit_ExistingProject_ConflictingArg_Refuses(t *testing.T) {
	dir := t.TempDir()
	if _, err := runInitInDir(t, dir, []string{"original-slug"}); err != nil {
		t.Fatalf("first runInit: %v", err)
	}

	_, err := runInitInDir(t, dir, []string{"different-slug"})
	if err == nil {
		t.Fatal("expected slug-conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot change project slug") {
		t.Errorf("error %q should mention slug change", err.Error())
	}
	// .specgraph.yaml should still hold the original slug.
	data, readErr := os.ReadFile(filepath.Join(dir, ".specgraph.yaml"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(data), "original-slug") {
		t.Errorf(".specgraph.yaml mutated; content: %s", data)
	}
	if strings.Contains(string(data), "different-slug") {
		t.Errorf(".specgraph.yaml gained the conflicting slug; content: %s", data)
	}
}

func TestRunInit_Idempotent_ByteEqualSecondRun(t *testing.T) {
	dir := t.TempDir()
	if _, err := runInitInDir(t, dir, []string{"my-project"}); err != nil {
		t.Fatalf("first runInit: %v", err)
	}

	// Snapshot all three managed config files after run 1.
	snaps := map[string][]byte{}
	for _, p := range []string{".cursor/mcp.json", ".mcp.json", "opencode.json"} {
		data, err := os.ReadFile(filepath.Join(dir, p))
		if err != nil {
			t.Fatal(err)
		}
		snaps[p] = data
	}

	// Run 2: should be no-op for all configs; bytes unchanged.
	if _, err := runInitInDir(t, dir, nil); err != nil {
		t.Fatalf("second runInit: %v", err)
	}
	for p, want := range snaps {
		got, err := os.ReadFile(filepath.Join(dir, p))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: bytes changed between idempotent runs", p)
		}
	}
}

// --- Preserved tests: slug derivation, config round-trip, flag acceptance ---
// These test the config package layer and CLI flag plumbing, which are not
// covered by the runInit-level tests above.

// changeDir temporarily changes the working directory for the duration of the test.
func changeDir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func TestInitDeriveSlugFromDirName(t *testing.T) {
	dir := t.TempDir()
	changeDir(t, dir)

	// With no git remote, LoadProject falls back to dir name.
	proj, err := config.LoadProject(dir)
	require.NoError(t, err)
	// Slug should be derived (non-empty).
	assert.NotEmpty(t, proj.Slug)
}

func TestInitWriteProjectConfig(t *testing.T) {
	dir := t.TempDir()

	pc := &config.ProjectConfig{Slug: "test-slug"}
	err := config.WriteProject(dir, pc)
	require.NoError(t, err)

	yamlPath := filepath.Join(dir, ".specgraph.yaml")
	_, err = os.Stat(yamlPath)
	require.NoError(t, err, ".specgraph.yaml should exist")

	loaded, err := config.LoadProject(dir)
	require.NoError(t, err)
	assert.Equal(t, "test-slug", loaded.Slug)
}

func TestInitYesFlagAccepted(t *testing.T) {
	t.Run("default is false", func(t *testing.T) {
		// --yes defaults to false (non-interactive mode is always on)
		assert.False(t, initYes)
	})

	t.Run("--yes flag is accepted by the command", func(t *testing.T) {
		dir := t.TempDir()
		changeDir(t, dir)

		// Restore global state after test.
		origYes := initYes
		t.Cleanup(func() {
			initYes = origYes
			initCmd.SetArgs(nil)
		})

		// Execute the init command with --yes; the command may fail due to
		// config isolation, but the flag itself must not be rejected as unknown.
		initCmd.SetArgs([]string{"--yes", "test-slug"})
		err := initCmd.Execute()
		// The command may fail, but it must NOT fail with "unknown flag: --yes".
		if err != nil {
			assert.NotContains(t, err.Error(), "unknown flag")
		}
	})
}
