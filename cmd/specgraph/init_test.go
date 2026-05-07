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
// restoring them on cleanup. Uses an empty global config (defaults).
func runInitInDir(t *testing.T, workDir string, args []string) (stdout string, err error) {
	t.Helper()
	return runInitInDirWithGlobalCfg(t, workDir, args, "")
}

// runInitInDirWithGlobalCfg is like runInitInDir but lets the caller supply
// the global-config YAML body (e.g. to set a non-default `client.default_server`
// and assert that URL flows through to the rendered MCP configs).
func runInitInDirWithGlobalCfg(t *testing.T, workDir string, args []string, globalCfgYAML string) (stdout string, err error) {
	t.Helper()

	// Isolate cfgFile so loadGlobalCfg() doesn't touch the developer's
	// real ~/.config/specgraph/config.yaml. Mirrors the pattern in
	// lifecycle_test.go and other tests in this package.
	//
	// We write the body to a temp file: when cfgFile != "" loadGlobalCfg
	// dispatches to config.LoadGlobalExplicit, which errors with "config
	// file not found" if the path doesn't exist. An empty body parses to
	// globalDefaults() (DefaultServer http://127.0.0.1:9090).
	oldCfgFile := cfgFile
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if writeErr := os.WriteFile(cfgPath, []byte(globalCfgYAML), 0o600); writeErr != nil {
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
	// Success banner fires on the project-creating run regardless of whether
	// the slug came from an arg or was derived. The slug itself is unstable
	// across machines (depends on git remote / dir name), so we match only
	// the constant prefix.
	if !strings.Contains(out, "Initialized project") {
		t.Errorf("stdout missing 'Initialized project' banner on fresh init: %s", out)
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

func TestRunInit_ResolvedServerURLFlowsIntoConfigs(t *testing.T) {
	// Pin the wiring from globalCfg.ResolveServer through ManagedConfigs
	// into the rendered file's `url` field. Without this test, a regression
	// that hardcoded "127.0.0.1:9090" or skipped the resolution step would
	// pass every other init test (they all use the default URL).
	dir := t.TempDir()
	const customServer = "https://specgraph.example.com:8443"
	cfgYAML := "client:\n  default_server: " + customServer + "\n"

	if _, err := runInitInDirWithGlobalCfg(t, dir, []string{"my-project"}, cfgYAML); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	// Each rendered config must carry the resolved URL with /mcp/ appended.
	wantURL := customServer + "/mcp/"
	for _, p := range []string{".cursor/mcp.json", ".mcp.json", "opencode.json"} {
		data, readErr := os.ReadFile(filepath.Join(dir, p))
		if readErr != nil {
			t.Fatalf("read %s: %v", p, readErr)
		}
		if !strings.Contains(string(data), wantURL) {
			t.Errorf("%s does not contain resolved URL %q; content:\n%s", p, wantURL, data)
		}
	}
}

func TestRunInit_RejectsInvalidServerURLs(t *testing.T) {
	// Pin the URL-validation strictness contract: relative URLs, missing
	// schemes, non-http(s) schemes, and the empty string must all be
	// refused — and the refusal must happen BEFORE .specgraph.yaml is
	// written, so a fresh project doesn't end up with a yaml on disk
	// pointing at a global config we already know is broken.
	// Note: an empty default_server in the YAML is defaulted to the
	// loopback URL by globalDefaults() upstream of ResolveServer, so
	// it never reaches the validator through the YAML path. The other
	// invalid-URL cases all flow through.
	cases := []struct {
		name      string
		serverURL string
	}{
		{"bare path", "/api"},
		{"host:port no scheme", "localhost:3000"},
		{"hostname only", "example.com"},
		{"non-http scheme", "ftp://example.com"},
		{"file scheme", "file:///tmp/x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfgYAML := "client:\n  default_server: " + tc.serverURL + "\n"

			_, err := runInitInDirWithGlobalCfg(t, dir, []string{"my-project"}, cfgYAML)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", tc.serverURL)
			}
			if !strings.Contains(err.Error(), "absolute http or https URL") {
				t.Errorf("error %q should mention 'absolute http or https URL'", err.Error())
			}

			// Validation runs before any writes — .specgraph.yaml must NOT
			// exist after a refused init on a fresh project.
			if _, statErr := os.Stat(filepath.Join(dir, ".specgraph.yaml")); !os.IsNotExist(statErr) {
				t.Errorf(".specgraph.yaml should not exist after URL refusal: %v", statErr)
			}
			// Likewise none of the managed MCP configs.
			for _, p := range []string{".cursor/mcp.json", ".mcp.json", "opencode.json"} {
				if _, statErr := os.Stat(filepath.Join(dir, p)); !os.IsNotExist(statErr) {
					t.Errorf("%s should not exist after URL refusal: %v", p, statErr)
				}
			}
		})
	}
}

func TestRunInit_NoSuccessBannerWhenSyncFails(t *testing.T) {
	// Pre-create a malformed .cursor/mcp.json so Sync fails on the first
	// file. .specgraph.yaml is still created (the success banner used to
	// fire here, before Sync ran), but the user's exit is non-zero — so
	// printing "Initialized project ..." would mislead. Banner must move
	// to after a successful Sync.
	dir := t.TempDir()
	cursorDir := filepath.Join(dir, ".cursor")
	if err := os.MkdirAll(cursorDir, 0o755); err != nil { //nolint:gosec // 0755 is intentional for test fixture dirs
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cursorDir, "mcp.json"), []byte(`{not valid json`), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := runInitInDir(t, dir, []string{"my-project"})
	if err == nil {
		t.Fatal("expected error from malformed .cursor/mcp.json, got nil")
	}
	if strings.Contains(out, "Initialized project") {
		t.Errorf("stdout contains success banner despite Sync failure:\n%s", out)
	}
}

func TestRunInit_ConflictingArg_DoesNotMutateMCPConfigs(t *testing.T) {
	// The slug-conflict refusal must fire before any file is touched. A
	// reordered runInit could write managed MCP configs with the conflicting
	// slug before the slug check, leaving partial state. Snapshot all
	// managed files before the conflicting call and assert byte-equality
	// after.
	dir := t.TempDir()
	if _, err := runInitInDir(t, dir, []string{"original-slug"}); err != nil {
		t.Fatalf("first runInit: %v", err)
	}

	managedPaths := []string{".specgraph.yaml", ".cursor/mcp.json", ".mcp.json", "opencode.json"}
	snaps := make(map[string][]byte, len(managedPaths))
	for _, p := range managedPaths {
		data, readErr := os.ReadFile(filepath.Join(dir, p))
		if readErr != nil {
			t.Fatal(readErr)
		}
		snaps[p] = data
	}

	if _, err := runInitInDir(t, dir, []string{"different-slug"}); err == nil {
		t.Fatal("expected slug-conflict error, got nil")
	}

	for _, p := range managedPaths {
		got, readErr := os.ReadFile(filepath.Join(dir, p))
		if readErr != nil {
			t.Fatalf("read %s after conflict: %v", p, readErr)
		}
		if !bytes.Equal(got, snaps[p]) {
			t.Errorf("%s mutated by failed conflicting-slug init; got:\n%s\nwant:\n%s", p, got, snaps[p])
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

func TestInit_FreshProject_WritesPointers(t *testing.T) {
	dir := t.TempDir()
	if _, err := runInitInDir(t, dir, []string{"specgraph"}); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	for _, p := range []string{"AGENTS.md", ".cursor/rules/specgraph-bootstrap.md"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("expected %s to exist: %v", p, err)
		}
	}
}

func TestInit_RerunIsNoOp(t *testing.T) {
	dir := t.TempDir()
	if _, err := runInitInDir(t, dir, []string{"specgraph"}); err != nil {
		t.Fatalf("first runInit: %v", err)
	}
	files := []string{
		".mcp.json", ".cursor/mcp.json", "opencode.json",
		"AGENTS.md", ".cursor/rules/specgraph-bootstrap.md",
	}
	snaps := map[string][]byte{}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		snaps[f] = data
	}
	if _, err := runInitInDir(t, dir, []string{"specgraph"}); err != nil {
		t.Fatalf("second runInit: %v", err)
	}
	for _, f := range files {
		got, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			t.Fatalf("read %s after second run: %v", f, err)
		}
		if !bytes.Equal(got, snaps[f]) {
			t.Errorf("%s: bytes changed between idempotent runs", f)
		}
	}
}

func TestInit_PurgesLegacyInjectArtifacts(t *testing.T) {
	dir := t.TempDir()
	seed := "# my AGENTS\n" +
		"<!-- specgraph:foo:start -->\nA\n<!-- specgraph:foo:end -->\n" +
		"<!-- specgraph:My.spec_v2:start -->\nB\n<!-- specgraph:My.spec_v2:end -->\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := runInitInDir(t, dir, []string{"specgraph"}); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	bs := string(body)
	if strings.Contains(bs, "specgraph:foo:") || strings.Contains(bs, "specgraph:My.spec_v2:") {
		t.Errorf("legacy markers not purged:\n%s", bs)
	}
	if !strings.Contains(bs, "<!-- specgraph:init:start v=1 -->") {
		t.Errorf("init block missing:\n%s", bs)
	}
}
