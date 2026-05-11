// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func (wholeFileStrategy) Inspect(cwd string, mf ManagedFile, _ ProjectParams) (FileState, error) {
	state, _, _, err := wholeFileClassify(cwd, mf)
	return state, err
}

//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func (wholeFileStrategy) Sync(cwd string, mf ManagedFile, _ ProjectParams, opts SyncOptions) (SyncResult, error) {
	if err := rejectSymlinkComponents(cwd, mf.Path); err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
	}
	full := filepath.Join(cwd, mf.Path)

	// Ensure the parent directory exists before acquiring the lock file
	// (the lock sibling lives in the same directory).
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
	}

	unlock, lerr := acquireFileLock(full)
	if lerr != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: lerr}, nil
	}
	defer func() {
		if uerr := unlock(); uerr != nil {
			slog.Error("unlock failed", "path", full, "error", uerr)
		}
	}()

	state, canonical, existing, cerr := wholeFileClassify(cwd, mf)
	if cerr != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: cerr}, nil
	}

	switch state.State {
	case StateSynced:
		return SyncResult{Path: mf.Path, Action: ActionNoOp}, nil

	case StateMissing:
		return wholeFileWrite(full, renderWholeFile(mf.Comment, canonical), ActionCreated, mf.Path), nil

	case StateStale:
		return wholeFileWrite(full, renderWholeFile(mf.Comment, canonical), ActionRefreshed, mf.Path), nil

	case StateDrifted:
		if !opts.Force {
			return SyncResult{Path: mf.Path, Action: ActionSkipped, Detail: state.Detail}, nil
		}
		if opts.KeepEdits {
			// Strip the first line ONLY when it's an actual sentinel.
			// StateDrifted is reached two ways: (a) sentinel hash !=
			// disk hash — first line is a sentinel, must strip;
			// (b) state.Detail == "no sentinel" — first line is user
			// content, must NOT strip (would silently drop content).
			body := existing
			if state.Detail != "no sentinel" {
				body = stripFirstLine(existing)
			}
			return wholeFileWrite(full, renderWholeFile(mf.Comment, body), ActionForced, mf.Path), nil
		}
		return wholeFileWrite(full, renderWholeFile(mf.Comment, canonical), ActionForced, mf.Path), nil
	}
	return SyncResult{Path: mf.Path, Action: ActionError, Err: fmt.Errorf("unhandled state %v", state.State)}, nil
}

//nolint:gocritic // ManagedFile is the framework's standard parameter shape
func wholeFileClassify(cwd string, mf ManagedFile) (FileState, []byte, []byte, error) {
	if err := rejectSymlinkComponents(cwd, mf.Path); err != nil {
		return FileState{}, nil, nil, err
	}
	full := filepath.Join(cwd, mf.Path)

	canonical, srcErr := readSource(mf)
	if srcErr != nil {
		return FileState{}, nil, nil, fmt.Errorf("read source for %s: %w", mf.Path, srcErr)
	}
	canonicalHash := hashBytes(canonical)

	existing, rerr := readFileNoFollow(full)
	switch {
	case noFollowIsSymlink(rerr):
		return FileState{}, nil, nil, fmt.Errorf("%w: %s", ErrSymlinkRejected, full)
	case errors.Is(rerr, fs.ErrNotExist):
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateMissing, EmbeddedHash: canonicalHash}, canonical, nil, nil
	case rerr != nil:
		return FileState{}, nil, nil, fmt.Errorf("read %s: %w", full, rerr)
	}

	// Parse the first line as a sentinel.
	firstLine, _, _ := bytes.Cut(existing, []byte("\n"))
	sentinel, perr := ParseSentinel(mf.Comment, string(firstLine))
	if perr != nil {
		return FileState{}, nil, nil, fmt.Errorf("parse sentinel for %s: %w", mf.Path, perr)
	}
	if sentinel.Version == 0 {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "no sentinel", EmbeddedHash: canonicalHash}, canonical, existing, nil
	}

	diskHash := HashExcludingSentinel(mf.Comment, existing)

	if sentinel.SHA256 != diskHash {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "sentinel hash != disk hash", DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	if diskHash != canonicalHash {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateStale, DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateSynced, DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
}

// renderWholeFile emits the canonical content prefixed by a v=2 sentinel
// line using the manifest entry's comment syntax. The hash is over
// `canonical` verbatim; HashExcludingSentinel on the rendered output
// drops the first line (the sentinel) and gets back the same bytes, so
// disk-hash and sentinel-hash agree on re-inspect.
func renderWholeFile(syntax CommentSyntax, canonical []byte) []byte {
	hash := hashBytes(canonical)
	sentinel := RenderSentinel(syntax, Sentinel{Version: 2, SHA256: hash})
	var b bytes.Buffer
	b.WriteString(sentinel)
	b.WriteString("\n")
	b.Write(canonical)
	if len(canonical) == 0 || canonical[len(canonical)-1] != '\n' {
		b.WriteString("\n")
	}
	return b.Bytes()
}

// stripFirstLine returns content with line 0 (the existing sentinel
// line) removed. Used in the --force --keep-edits path to peel off the
// stale sentinel so renderWholeFile can compute a fresh one over the
// user-edited body.
func stripFirstLine(content []byte) []byte {
	idx := bytes.IndexByte(content, '\n')
	if idx < 0 {
		return []byte{}
	}
	return content[idx+1:]
}

func wholeFileWrite(full string, content []byte, action Action, displayPath string) SyncResult {
	mode := preserveMode(full)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		return SyncResult{Path: displayPath, Action: ActionError, Err: err}
	}
	if err := atomicWrite(full, content, mode); err != nil {
		return SyncResult{Path: displayPath, Action: ActionError, Err: err}
	}
	return SyncResult{Path: displayPath, Action: action}
}
