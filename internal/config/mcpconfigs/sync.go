// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package mcpconfigs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
)

// Action describes what Sync did to a single managed config file.
type Action string

// The set of valid Action values returned by Sync. Any future code branching
// on a SyncResult should compare against these constants rather than string
// literals so typos surface at compile time.
const (
	ActionCreated Action = "created" // file did not exist before; written from canonicalized patch.
	ActionUpdated Action = "updated" // file existed; canonicalized merge result differs from prior bytes.
	ActionNoOp    Action = "no-op"   // file existed; canonicalized merge result matches prior bytes byte-for-byte.
)

// SyncResult reports what Sync did to a single managed config file.
type SyncResult struct {
	// Path is the project-relative path of the file (matches ManagedConfig.Path).
	Path string

	// Action is the outcome for this file. Always one of ActionCreated,
	// ActionUpdated, or ActionNoOp.
	Action Action
}

// Sync applies each ManagedConfig's merge patch to its target file under
// projectDir. For each file: if the file is missing, write the canonicalized
// patch as the full file content (ActionCreated). Otherwise read existing
// content, apply the patch via RFC 7396 merge, canonicalize the result, and
// write only if the result differs from the existing bytes (ActionUpdated
// or ActionNoOp).
//
// Each file is gated by rejectSymlinkComponents: any symlink along the path
// inside projectDir aborts the operation before any FS write so a malicious
// or accidental symlink cannot redirect writes outside projectDir.
//
// Special: for opencode.json, refuse if a sibling opencode.jsonc exists in
// the same directory (probed with os.Lstat so dangling symlinks also block);
// OpenCode supports both formats and writing opencode.json next to a user's
// pre-existing opencode.jsonc would create ambiguous active-config state.
//
// On any error, returns the partial results collected up to that point and
// the error wrapped with the offending path.
func Sync(projectDir string, configs []ManagedConfig) ([]SyncResult, error) {
	results := make([]SyncResult, 0, len(configs))
	for _, cfg := range configs {
		result, err := syncOne(projectDir, cfg)
		if err != nil {
			return results, err
		}
		results = append(results, result)
	}
	return results, nil
}

func syncOne(projectDir string, cfg ManagedConfig) (SyncResult, error) {
	fullPath := filepath.Join(projectDir, cfg.Path)

	// Reject the operation if any path component within projectDir is a
	// symlink. os.ReadFile / os.MkdirAll / os.WriteFile all follow symlinks,
	// so a checkout containing e.g. ".cursor -> ~/.config" would let init
	// silently overwrite files outside projectDir.
	if err := rejectSymlinkComponents(projectDir, cfg.Path); err != nil {
		return SyncResult{}, err
	}

	// OpenCode-only: refuse if opencode.jsonc sibling exists. Lstat (not
	// Stat) so a broken/dangling jsonc symlink also blocks the write —
	// from the user's perspective, the directory entry is what matters,
	// not whether its target resolves.
	if cfg.Path == "opencode.json" {
		jsoncPath := filepath.Join(projectDir, "opencode.jsonc")
		if _, statErr := os.Lstat(jsoncPath); statErr == nil {
			return SyncResult{}, fmt.Errorf(
				"found opencode.jsonc alongside opencode.json; consolidate to one file (init does not yet manage opencode.jsonc)",
			)
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return SyncResult{}, fmt.Errorf("lstat %s: %w", jsoncPath, statErr)
		}
	}

	existing, err := os.ReadFile(fullPath)
	fileExisted := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return SyncResult{}, fmt.Errorf("read %s: %w", fullPath, err)
	}

	// If the file exists, validate it parses as JSON before any merge.
	if fileExisted {
		var probe any
		if jerr := json.Unmarshal(existing, &probe); jerr != nil {
			return SyncResult{}, fmt.Errorf("parse %s: %w", fullPath, jerr)
		}
	}

	// Compute the merged document.
	var merged []byte
	if fileExisted {
		merged, err = jsonpatch.MergePatch(existing, cfg.Patch)
		if err != nil {
			return SyncResult{}, fmt.Errorf("merge patch %s: %w", fullPath, err)
		}
	} else {
		// Equivalent to MergePatch({}, cfg.Patch) — produce the patch as
		// the canonical doc.
		merged, err = jsonpatch.MergePatch([]byte(`{}`), cfg.Patch)
		if err != nil {
			return SyncResult{}, fmt.Errorf("merge patch %s: %w", fullPath, err)
		}
	}

	// Canonicalize: 2-space indent + trailing newline. Map keys are emitted
	// in alphabetical order by encoding/json, giving deterministic output.
	canonical, err := canonicalize(merged)
	if err != nil {
		return SyncResult{}, fmt.Errorf("canonicalize %s: %w", fullPath, err)
	}

	if fileExisted && bytes.Equal(existing, canonical) {
		return SyncResult{Path: cfg.Path, Action: ActionNoOp}, nil
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil { //nolint:gosec // 0755 is intentional for config dirs
		return SyncResult{}, fmt.Errorf("mkdir %s: %w", filepath.Dir(fullPath), err)
	}
	if err := os.WriteFile(fullPath, canonical, 0o600); err != nil {
		return SyncResult{}, fmt.Errorf("write %s: %w", fullPath, err)
	}

	if fileExisted {
		return SyncResult{Path: cfg.Path, Action: ActionUpdated}, nil
	}
	return SyncResult{Path: cfg.Path, Action: ActionCreated}, nil
}

// rejectSymlinkComponents returns an error if any path component below
// projectDir on the way to relPath is a symlink. Components that don't
// exist yet are fine — the goal is to prevent an attacker (or accident)
// from arranging for a routine os.WriteFile inside what looks like the
// project tree to land outside it.
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

// canonicalize returns the JSON document re-marshaled with 2-space indent
// and a trailing newline. This is the form Sync compares against existing
// file bytes for the no-op short-circuit.
func canonicalize(data []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal indent: %w", err)
	}
	return append(out, '\n'), nil
}
