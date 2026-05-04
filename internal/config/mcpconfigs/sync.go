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

	jsonpatch "github.com/evanphx/json-patch/v5"
)

// SyncResult reports what Sync did to a single managed config file.
type SyncResult struct {
	// Path is the project-relative path of the file (matches ManagedConfig.Path).
	Path string

	// Action is one of: "created" (file did not exist before), "updated"
	// (file existed and bytes changed), or "no-op" (file existed with
	// canonical content already).
	Action string
}

// Sync applies each ManagedConfig's merge patch to its target file under
// projectDir. For each file: if the file is missing, write the canonicalized
// patch as the full file content (action "created"). Otherwise read existing
// content, apply the patch via RFC 7396 merge, canonicalize the result, and
// write only if the result differs from the existing bytes (action "updated"
// or "no-op").
//
// Special: for opencode.json, refuse if a sibling opencode.jsonc exists in
// the same directory; OpenCode supports both formats and writing
// opencode.json next to a user's pre-existing opencode.jsonc would create
// ambiguous active-config state.
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

	// OpenCode-only: refuse if opencode.jsonc sibling exists.
	if cfg.Path == "opencode.json" {
		jsoncPath := filepath.Join(projectDir, "opencode.jsonc")
		if _, statErr := os.Stat(jsoncPath); statErr == nil {
			return SyncResult{}, fmt.Errorf(
				"found opencode.jsonc alongside opencode.json; consolidate to one file (init does not yet manage opencode.jsonc)",
			)
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return SyncResult{}, fmt.Errorf("stat %s: %w", jsoncPath, statErr)
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
		return SyncResult{Path: cfg.Path, Action: "no-op"}, nil
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil { //nolint:gosec // 0755 is intentional for config dirs
		return SyncResult{}, fmt.Errorf("mkdir %s: %w", filepath.Dir(fullPath), err)
	}
	if err := os.WriteFile(fullPath, canonical, 0o600); err != nil {
		return SyncResult{}, fmt.Errorf("write %s: %w", fullPath, err)
	}

	if fileExisted {
		return SyncResult{Path: cfg.Path, Action: "updated"}, nil
	}
	return SyncResult{Path: cfg.Path, Action: "created"}, nil
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
