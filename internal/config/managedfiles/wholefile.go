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
		return wholeFileWrite(full, renderWholeFile(canonical), ActionCreated, mf.Path), nil

	case StateStale:
		return wholeFileWrite(full, renderWholeFile(canonical), ActionRefreshed, mf.Path), nil

	case StateDrifted:
		if !opts.Force {
			return SyncResult{Path: mf.Path, Action: ActionSkipped, Detail: state.Detail}, nil
		}
		if opts.KeepEdits {
			body := stripFirstLine(existing)
			return wholeFileWrite(full, renderWholeFile(body), ActionForced, mf.Path), nil
		}
		return wholeFileWrite(full, renderWholeFile(canonical), ActionForced, mf.Path), nil
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
	sentinel, perr := ParseSentinel(CommentSlash, string(firstLine))
	if perr != nil {
		return FileState{}, nil, nil, perr
	}
	if sentinel.Version == 0 {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "no sentinel", EmbeddedHash: canonicalHash}, canonical, existing, nil
	}

	diskHash := HashExcludingSentinel(CommentSlash, existing)

	if sentinel.SHA256 != diskHash {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "sentinel hash != disk hash", DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	if diskHash != canonicalHash {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateStale, DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateSynced, DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
}

// renderWholeFile emits the canonical content prefixed by a v=2 sentinel
// line. The hash is over `canonical` verbatim; HashExcludingSentinel on
// the rendered output drops the first line (the sentinel) and gets back
// the same bytes, so disk-hash and sentinel-hash agree on re-inspect.
func renderWholeFile(canonical []byte) []byte {
	hash := hashBytes(canonical)
	var b bytes.Buffer
	b.WriteString("// specgraph:init v=2 sha256=")
	b.WriteString(hash)
	b.WriteString("\n")
	b.Write(canonical)
	if len(canonical) == 0 || canonical[len(canonical)-1] != '\n' {
		b.WriteString("\n")
	}
	return b.Bytes()
}

// stripFirstLine returns content with line 0 removed. Used in the
// --force --keep-edits path to compute a fresh sentinel over the
// user's (sentinel-less) body.
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
