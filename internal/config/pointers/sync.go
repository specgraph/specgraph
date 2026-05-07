// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package pointers

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Action describes what Sync did to a single managed pointer file. The string
// values are deliberately identical to mcpconfigs.Action values so init can
// render a unified "<path>: <action>" output.
type Action string

const (
	ActionCreated Action = "created"
	ActionUpdated Action = "updated"
	ActionNoOp    Action = "no-op"
	ActionError   Action = "error"
)

// SyncResult reports what Sync did to a single managed pointer file.
type SyncResult struct {
	Path               string
	Action             Action
	Err                error
	LegacyBlocksPurged int
}

// Options carries the canonical values that init derives once and threads
// into the pointer templates.
type Options struct {
	ServerURL   string
	ProjectSlug string
}

// Sync reconciles all pointer files for the project. Returns a slice with
// one SyncResult per file in the order [AGENTS.md, .cursor/rules/specgraph-bootstrap.md].
//
// If projectDir is missing or not a directory, Sync returns a single-element
// slice with Action == ActionError and Path == "<projectDir>".
//
// A failure on one pointer file is reported via SyncResult.Err with
// Action == ActionError; the other file is still processed. This differs
// from mcpconfigs.Sync, which aborts on first error. The caller (init)
// reconciles by running mcpconfigs first and only invoking pointers.Sync if
// mcpconfigs succeeded.
func Sync(projectDir string, opts Options) []SyncResult {
	info, err := os.Stat(projectDir)
	if err != nil || !info.IsDir() {
		msg := fmt.Errorf("projectDir %q is not a directory", projectDir)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			msg = fmt.Errorf("stat %s: %w", projectDir, err)
		}
		return []SyncResult{{Path: projectDir, Action: ActionError, Err: msg}}
	}
	return []SyncResult{
		syncAgents(projectDir, opts),
		syncCursor(projectDir, opts),
	}
}

// errResult is a small convenience for syncAgents / syncCursor.
func errResult(path string, err error) SyncResult {
	return SyncResult{Path: path, Action: ActionError, Err: err}
}

// rejectSymlinkComponents copies the helper from internal/config/mcpconfigs/sync.go
// to keep packages independent. Reproducing it is cheaper than introducing a
// shared util.
func rejectSymlinkComponents(projectDir, relPath string) error {
	cur := projectDir
	for _, part := range strings.Split(filepath.Clean(relPath), string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("lstat %s: %w", cur, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to follow symlink %s", cur)
		}
	}
	return nil
}

// atomicWrite writes data to <fullPath>.tmp.<random> in the same directory
// then renames over fullPath. Removes the temp on failure.
func atomicWrite(fullPath string, data []byte) error {
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(fullPath)+".tmp.*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, werr := tmp.Write(data); werr != nil {
		tmp.Close()        //nolint:errcheck // best-effort cleanup
		os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("write temp: %w", werr)
	}
	if cerr := tmp.Chmod(0o600); cerr != nil {
		tmp.Close()        //nolint:errcheck // best-effort cleanup
		os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("chmod temp: %w", cerr)
	}
	if cerr := tmp.Close(); cerr != nil {
		os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("close temp: %w", cerr)
	}
	if rerr := os.Rename(tmpName, fullPath); rerr != nil {
		os.Remove(tmpName) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("rename: %w", rerr)
	}
	return nil
}
