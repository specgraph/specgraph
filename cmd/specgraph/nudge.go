// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"hash/crc32"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/config/managedfiles"
	"github.com/specgraph/specgraph/internal/xdg"
)

// nudgeIsTerminal is the function used to detect whether stderr is a
// TTY. It exists as a package-level variable so unit tests can stub it
// out — `go test` runs with stderr not attached to a TTY, which would
// otherwise prevent any test exercising the emit path.
var nudgeIsTerminal = func() bool { return term.IsTerminal(int(os.Stderr.Fd())) }

// driftNudgeAllowList enumerates the top-level command names whose
// subtrees skip the drift-nudge entirely. Matched against the
// top-level command (one level under rootCmd) per design.
var driftNudgeAllowList = map[string]bool{
	"init":              true,
	"doctor":            true,
	"health":            true,
	"read-mcp-resource": true,
	"serve":             true,
	"version":           true,
	"bundle":            true,
	"up":                true,
	"confluence":        true,
}

// nudgePreRun is rootCmd.PersistentPreRunE. Runs InspectAll and emits
// one stderr line if any file is Stale or Drifted. Multiple skip gates
// (subcommand allow-list, isatty, env, config, throttle) keep the
// fast path cheap.
func nudgePreRun(cmd *cobra.Command, _ []string) error {
	// 1. Subcommand allow-list: walk to the top-level command.
	top := cmd
	for top.HasParent() && top.Parent() != rootCmd {
		top = top.Parent()
	}
	if driftNudgeAllowList[top.Name()] {
		return nil
	}
	// 2. isatty(stderr).
	if !nudgeIsTerminal() {
		return nil
	}
	// 3. Env-var mute.
	if os.Getenv("SPECGRAPH_DRIFT_NUDGE") == "off" {
		return nil
	}
	// 4. Project-level mute, project root, and harness list — together,
	//    because all three derive from ProjectConfig.
	cwd, err := os.Getwd()
	if err != nil {
		return nil //nolint:nilerr // advisory feature; never fail the CLI
	}
	root, err := config.FindProjectRoot(cwd)
	if err != nil {
		// No project up the tree — nothing to inspect, nothing to nudge.
		return nil //nolint:nilerr // advisory: missing project is normal
	}
	pc, err := config.LoadProject(root)
	if err != nil {
		return nil //nolint:nilerr // advisory: malformed config not our problem here
	}
	if pc.Nudges.Quiet {
		return nil
	}
	// 5. Build ProjectParams matching init's path so sentinel hashes line
	// up (harnessSliceFromConfig + ResolveServer).
	globalCfg, err := loadGlobalCfg()
	if err != nil {
		return nil //nolint:nilerr // advisory: global config error not our problem
	}
	params := managedfiles.ProjectParams{
		Slug:      pc.Slug,
		ServerURL: globalCfg.ResolveServer(pc.Slug, pc.Server),
	}
	harnesses := harnessSliceFromConfig(pc.Harnesses)

	states, err := managedfiles.InspectAll(root, harnesses, params)
	if err != nil {
		return nil //nolint:nilerr // advisory: inspect failure isn't fatal here
	}
	var stale, drifted int
	for _, s := range states {
		switch s.State {
		case managedfiles.StateStale:
			stale++
		case managedfiles.StateDrifted:
			drifted++
		}
	}
	if stale == 0 && drifted == 0 {
		return nil
	}
	// 6. Throttle — consulted ONLY when we'd otherwise emit. Checking it
	// before drift detection would have every clean invocation create the
	// throttle file, suppressing the first real nudge for up to 24h after
	// drift actually appears.
	if !shouldEmitAfterThrottle(root) {
		return nil
	}
	fmt.Fprintf(os.Stderr,
		"note: %d managed files out of date with this binary (%d stale, %d drifted); run `specgraph init` to refresh, `specgraph doctor` for details\n",
		stale+drifted, stale, drifted)
	gcOldNudgeFiles()
	return nil
}

// shouldEmitAfterThrottle returns true if the throttle file for
// (projectRoot, binaryVersionHash) is missing or older than 24h.
// On error or unwritable cache, returns true (fail open) per design.
func shouldEmitAfterThrottle(projectRoot string) bool {
	path := throttleFilePath(projectRoot)
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// First time — create and emit.
			if mkErr := os.MkdirAll(filepath.Dir(path), 0o750); mkErr != nil {
				return true
			}
			if wErr := os.WriteFile(path, nil, 0o600); wErr != nil {
				return true
			}
			return true
		}
		return true
	}
	if time.Since(info.ModTime()) < 24*time.Hour {
		return false
	}
	if err := os.Chtimes(path, time.Now(), time.Now()); err != nil {
		return true
	}
	return true
}

// throttleFilePath builds the per-(project, binary-version) path under
// xdg.CacheHome()/nudges. buildTime is not a declared ldflags variable,
// so the version hash covers version+commit only.
func throttleFilePath(projectRoot string) string {
	resolved, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		resolved = projectRoot
	}
	projectHash := fmt.Sprintf("%x", sha256.Sum256([]byte(resolved)))
	versionHash := fmt.Sprintf("%x", crc32.ChecksumIEEE([]byte(version+commit)))
	return filepath.Join(xdg.CacheHome(), "nudges", projectHash+"-"+versionHash)
}

// gcOldNudgeFiles deletes throttle entries with mtime > 30 days. One
// readdir per nudge; cheap and prevents indefinite accumulation.
func gcOldNudgeFiles() {
	dir := filepath.Join(xdg.CacheHome(), "nudges")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name())) //nolint:errcheck // best-effort GC
		}
	}
}
