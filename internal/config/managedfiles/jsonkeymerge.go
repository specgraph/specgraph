// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

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

//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func (jsonKeyMergeStrategy) Inspect(cwd string, mf ManagedFile, params ProjectParams) (FileState, error) {
	if err := rejectSymlinkComponents(cwd, mf.Path); err != nil {
		return FileState{}, err
	}
	full := filepath.Join(cwd, mf.Path)
	existing, err := readFileNoFollow(full)
	switch {
	case noFollowIsSymlink(err):
		return FileState{}, fmt.Errorf("%w: %s", ErrSymlinkRejected, full)
	case errors.Is(err, fs.ErrNotExist):
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateMissing, Detail: "file does not exist"}, nil
	case err != nil:
		return FileState{}, fmt.Errorf("read %s: %w", full, err)
	}
	// Pre-validate JSON to surface parse errors with a clear message
	// before they get wrapped inside jsonpatch.MergePatch.
	var probe any
	if jerr := json.Unmarshal(existing, &probe); jerr != nil {
		return FileState{}, fmt.Errorf("parse %s: %w", full, jerr)
	}
	canonical, err := jsonKeyMergeCanonical(existing, mf, params)
	if err != nil {
		return FileState{}, err
	}
	if bytes.Equal(existing, canonical) {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateSynced}, nil
	}
	return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateStale}, nil
}

//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func (jsonKeyMergeStrategy) Sync(cwd string, mf ManagedFile, params ProjectParams, _ SyncOptions) (SyncResult, error) {
	if err := rejectSymlinkComponents(cwd, mf.Path); err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil //nolint:nilerr // err is carried in SyncResult.Err per framework contract
	}
	full := filepath.Join(cwd, mf.Path)

	// opencode.json refuses if opencode.jsonc exists alongside.
	if mf.Path == "opencode.json" {
		jsoncPath := filepath.Join(cwd, "opencode.jsonc")
		if _, statErr := os.Lstat(jsoncPath); statErr == nil {
			return SyncResult{Path: mf.Path, Action: ActionError, Err: fmt.Errorf("found opencode.jsonc alongside opencode.json; consolidate to one file")}, nil
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return SyncResult{Path: mf.Path, Action: ActionError, Err: fmt.Errorf("lstat %s: %w", jsoncPath, statErr)}, nil
		}
	}

	existing, rerr := readFileNoFollow(full)
	fileExisted := rerr == nil
	switch {
	case noFollowIsSymlink(rerr):
		return SyncResult{Path: mf.Path, Action: ActionError, Err: fmt.Errorf("%w: %s", ErrSymlinkRejected, full)}, nil
	case rerr != nil && !errors.Is(rerr, fs.ErrNotExist):
		return SyncResult{Path: mf.Path, Action: ActionError, Err: fmt.Errorf("read %s: %w", full, rerr)}, nil
	}
	if fileExisted {
		var probe any
		if jerr := json.Unmarshal(existing, &probe); jerr != nil {
			return SyncResult{Path: mf.Path, Action: ActionError, Err: fmt.Errorf("parse %s: %w", full, jerr)}, nil
		}
	}

	canonical, err := jsonKeyMergeCanonical(existing, mf, params)
	if err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil //nolint:nilerr // err is carried in SyncResult.Err per framework contract
	}

	if fileExisted && bytes.Equal(existing, canonical) {
		return SyncResult{Path: mf.Path, Action: ActionNoOp}, nil
	}

	mode := os.FileMode(0o600)
	if info, statErr := os.Stat(full); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: fmt.Errorf("mkdir %s: %w", filepath.Dir(full), err)}, nil
	}
	if err := atomicWrite(full, canonical, mode); err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil //nolint:nilerr // err is carried in SyncResult.Err per framework contract
	}
	if fileExisted {
		return SyncResult{Path: mf.Path, Action: ActionRefreshed}, nil
	}
	return SyncResult{Path: mf.Path, Action: ActionCreated}, nil
}

// jsonKeyMergeCanonical computes the canonical disk content for an entry.
// Routes on whether the entry uses the new declarative JSONKeys field or
// the legacy Build closure. JSONKeys path handles KeyManagedValue
// (merge-patch), KeyManagedPresence (preserve existing), and
// KeyManagedArrayUnion (set-union with existing array).
//
//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func jsonKeyMergeCanonical(existing []byte, mf ManagedFile, params ProjectParams) ([]byte, error) {
	if len(mf.JSONKeys) > 0 {
		return jsonKeyMergeCanonicalFromKeys(existing, mf, params)
	}
	return jsonKeyMergeCanonicalFromBuild(existing, mf, params)
}

// jsonKeyMergeCanonicalFromBuild is the pre-PR-E legacy path retained
// transitionally until the three remaining Build-style entries migrate.
// Task 7 removes it.
//
//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func jsonKeyMergeCanonicalFromBuild(existing []byte, mf ManagedFile, params ProjectParams) ([]byte, error) {
	patch, err := mf.Build(params)
	if err != nil {
		return nil, fmt.Errorf("build patch for %s: %w", mf.Path, err)
	}
	src := existing
	if len(src) == 0 {
		src = []byte(`{}`)
	}
	merged, err := jsonpatch.MergePatch(src, patch)
	if err != nil {
		return nil, fmt.Errorf("merge patch %s: %w", mf.Path, err)
	}
	canonical, err := canonicalize(merged)
	if err != nil {
		return nil, err
	}
	if mf.Path == "opencode.json" {
		canonical, err = unionPluginArray(existing, canonical)
		if err != nil {
			return nil, fmt.Errorf("union plugin array for %s: %w", mf.Path, err)
		}
	}
	return canonical, nil
}

//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func jsonKeyMergeCanonicalFromKeys(existing []byte, mf ManagedFile, params ProjectParams) ([]byte, error) {
	src := existing
	if len(src) == 0 {
		src = []byte(`{}`)
	}
	// Phase 1: build patch from KeyManagedValue keys.
	patch := map[string]any{}
	for _, k := range mf.JSONKeys {
		if k.Mode != KeyManagedValue {
			continue
		}
		v, err := k.Value(params)
		if err != nil {
			return nil, fmt.Errorf("value for %s: %w", k.Path, err)
		}
		if err := jsonPointerSet(patch, k.Path, v); err != nil {
			return nil, fmt.Errorf("set %s: %w", k.Path, err)
		}
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("marshal patch for %s: %w", mf.Path, err)
	}
	merged, err := jsonpatch.MergePatch(src, patchBytes)
	if err != nil {
		return nil, fmt.Errorf("merge patch %s: %w", mf.Path, err)
	}
	// Subsequent phases (Presence, ArrayUnion) added in Tasks 3 and 4.
	return canonicalize(merged)
}
