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

	var res SyncResult
	switch state.State {
	case StateSynced:
		res = SyncResult{Path: mf.Path, Action: ActionNoOp}

	case StateMissing:
		res = wholeFileWrite(full, renderWholeFile(mf, canonical), ActionCreated, mf.Path)

	case StateStale:
		res = wholeFileWrite(full, renderWholeFile(mf, canonical), ActionRefreshed, mf.Path)

	case StateDrifted:
		if !opts.Force {
			return SyncResult{Path: mf.Path, Action: ActionSkipped, Detail: state.Detail}, nil
		}
		if opts.KeepEdits {
			if mf.HasFrontmatter {
				// For .mdc files, splitting on frontmatter and stripping the
				// sentinel from the post-frontmatter body is the only safe way to
				// preserve user edits. The default stripFirstLine path would strip
				// the `---` frontmatter opener and break splitFrontmatter on the
				// next call to renderWholeFile.
				if state.Detail == "frontmatter missing or unclosed" {
					// User broke the frontmatter shape — nothing recoverable for
					// KeepEdits. Skip with an explanatory Detail.
					return SyncResult{Path: mf.Path, Action: ActionSkipped, Detail: "force --keep-edits cannot preserve content with malformed frontmatter; remove or re-add `---` delimiters"}, nil
				}
				front, body, _ := splitFrontmatter(existing)
				// Strip the sentinel from body[0] ONLY if there was a sentinel
				// there. state.Detail == "no sentinel" means body[0] is user
				// content; preserve it as-is.
				if state.Detail != "no sentinel" {
					body = stripFirstLine(body)
				}
				reassembled := append(append([]byte{}, front...), body...)
				res = wholeFileWrite(full, renderWholeFile(mf, reassembled), ActionForced, mf.Path)
			} else {
				// Non-frontmatter path: original behavior.
				// Strip the first line ONLY when it's an actual sentinel.
				// StateDrifted is reached two ways: (a) sentinel hash !=
				// disk hash — first line is a sentinel, must strip;
				// (b) state.Detail == "no sentinel" — first line is user
				// content, must NOT strip (would silently drop content).
				body := existing
				if state.Detail != "no sentinel" {
					body = stripFirstLine(existing)
				}
				res = wholeFileWrite(full, renderWholeFile(mf, body), ActionForced, mf.Path)
			}
		} else {
			res = wholeFileWrite(full, renderWholeFile(mf, canonical), ActionForced, mf.Path)
		}

	default:
		return SyncResult{Path: mf.Path, Action: ActionError, Err: fmt.Errorf("unhandled state %v", state.State)}, nil
	}

	// Early-exit on write errors so supersedes cleanup doesn't run.
	if res.Action == ActionError {
		return res, nil
	}

	// Supersedes-guarded delete of any pre-rename path. Runs on the same
	// action set as markdownblock.go:163-176 (Created, Refreshed, Forced,
	// NoOp) so a user who's already at Synced still gets cleanup if they
	// happen to also have the old .md sitting around.
	if mf.SupersedesPath != "" && (res.Action == ActionCreated || res.Action == ActionRefreshed || res.Action == ActionForced || res.Action == ActionNoOp) {
		priorHash := vestigialCursorRulePriorHash(mf.SupersedesPath)
		if err := supersedesGuardedDelete(cwd, mf.SupersedesPath, priorHash); err != nil {
			if errors.Is(err, ErrPriorCanonicalMismatch) {
				if res.Detail != "" {
					res.Detail += "; "
				}
				res.Detail += fmt.Sprintf("supersedes path %q left in place: prior-canonical mismatch", mf.SupersedesPath)
			} else {
				return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
			}
		}
	}

	return res, nil
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

	var canonicalHash string
	if mf.HasFrontmatter {
		// Canonical .mdc lives on disk with frontmatter and no sentinel.
		// Use the sibling hash function so canonical_hash on disk is
		// computed the same way as disk_hash on a synced file (sentinel
		// removed from body[0], everything else hashed).
		h, herr := HashExcludingSentinelAfterFrontmatter(mf.Comment, canonical)
		if herr != nil {
			return FileState{}, nil, nil, fmt.Errorf("hash canonical %s: %w", mf.Path, herr)
		}
		canonicalHash = h
	} else {
		canonicalHash = hashBytes(canonical)
	}

	existing, rerr := readFileNoFollow(full)
	switch {
	case noFollowIsSymlink(rerr):
		return FileState{}, nil, nil, fmt.Errorf("%w: %s", ErrSymlinkRejected, full)
	case errors.Is(rerr, fs.ErrNotExist):
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateMissing, EmbeddedHash: canonicalHash}, canonical, nil, nil
	case rerr != nil:
		return FileState{}, nil, nil, fmt.Errorf("read %s: %w", full, rerr)
	}

	if mf.HasFrontmatter {
		return classifyMdcWholeFile(mf, existing, canonical, canonicalHash)
	}
	return classifyNonFrontmatterWholeFile(mf, existing, canonical, canonicalHash)
}

// classifyNonFrontmatterWholeFile mirrors the pre-HasFrontmatter behavior:
// sentinel on line 1, hash via HashExcludingSentinel.
//
//nolint:gocritic // see wholeFileClassify
func classifyNonFrontmatterWholeFile(mf ManagedFile, existing, canonical []byte, canonicalHash string) (FileState, []byte, []byte, error) {
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

// classifyMdcWholeFile is the frontmatter-aware variant.
//
//nolint:gocritic // see wholeFileClassify
func classifyMdcWholeFile(mf ManagedFile, existing, canonical []byte, canonicalHash string) (FileState, []byte, []byte, error) {
	_, body, fmErr := splitFrontmatter(existing)
	if fmErr != nil {
		// User broke the frontmatter — refuse to mutate without --force.
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "frontmatter missing or unclosed", EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	firstLine, _, _ := bytes.Cut(body, []byte("\n"))
	sentinel, perr := ParseSentinel(mf.Comment, string(firstLine))
	if perr != nil {
		return FileState{}, nil, nil, fmt.Errorf("parse sentinel for %s: %w", mf.Path, perr)
	}
	if sentinel.Version == 0 {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "no sentinel", EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	diskHash, hashErr := HashExcludingSentinelAfterFrontmatter(mf.Comment, existing)
	if hashErr != nil {
		return FileState{}, nil, nil, fmt.Errorf("hash %s: %w", mf.Path, hashErr)
	}
	if sentinel.SHA256 != diskHash {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "sentinel hash != disk hash", DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	if diskHash != canonicalHash {
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateStale, DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
	}
	return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateSynced, DiskHash: diskHash, SentinelHash: sentinel.SHA256, EmbeddedHash: canonicalHash}, canonical, existing, nil
}

// renderWholeFile emits canonical content prefixed by a v=2 sentinel line.
// For HasFrontmatter==false (default), the sentinel sits on line 1 of the
// file. For HasFrontmatter==true, the sentinel sits on the first body line
// after the leading YAML frontmatter — Cursor's .mdc parser requires
// `---` on line 1, so the sentinel can't sit above frontmatter.
//
// The hash is computed over the canonical content using the same hashing
// rules as classify: HashExcludingSentinel for line-1 sentinels,
// HashExcludingSentinelAfterFrontmatter when HasFrontmatter==true. On
// re-inspect, disk-hash equals sentinel-hash.
func renderWholeFile(mf ManagedFile, canonical []byte) []byte {
	if mf.HasFrontmatter {
		return renderWholeFileWithFrontmatter(mf, canonical)
	}
	hash := hashBytes(canonical)
	sentinel := RenderSentinel(mf.Comment, Sentinel{Version: 2, SHA256: hash})
	var b bytes.Buffer
	b.WriteString(sentinel)
	b.WriteString("\n")
	b.Write(canonical)
	if len(canonical) == 0 || canonical[len(canonical)-1] != '\n' {
		b.WriteString("\n")
	}
	return b.Bytes()
}

func renderWholeFileWithFrontmatter(mf ManagedFile, canonical []byte) []byte {
	front, body, err := splitFrontmatter(canonical)
	if err != nil {
		panic(fmt.Sprintf("canonical %s has malformed frontmatter: %v", mf.Path, err))
	}
	// Hash input: front + body (no sentinel). HashExcludingSentinelAfter-
	// Frontmatter on the rendered output drops the inserted sentinel and
	// returns the same hash, so disk-hash agrees with sentinel-hash.
	hashInput := bytes.Join([][]byte{front, body}, nil)
	hash := hashBytes(hashInput)
	sentinel := RenderSentinel(mf.Comment, Sentinel{Version: 2, SHA256: hash})

	var b bytes.Buffer
	b.Write(front)
	b.WriteString(sentinel)
	b.WriteString("\n")
	b.Write(body)
	if len(body) == 0 || body[len(body)-1] != '\n' {
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
