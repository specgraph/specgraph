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
func (markdownBlockStrategy) Inspect(cwd string, mf ManagedFile, params ProjectParams) (FileState, error) {
	state, _, _, err := markdownBlockClassify(cwd, mf, params)
	return state, err
}

//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func (markdownBlockStrategy) Sync(cwd string, mf ManagedFile, params ProjectParams, opts SyncOptions) (SyncResult, error) {
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

	state, existing, purgedAfter, err := markdownBlockClassify(cwd, mf, params)
	if err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
	}

	// Count purged + skipped-malformed legacy blocks for Detail reporting.
	// purgeLegacyBlocks returns (cleaned-bytes, purged-count,
	// skipped-malformed-count). The cleaned bytes are already captured
	// upstream as purgedAfter inside markdownBlockClassify; we only need
	// the two counts here for the Detail string.
	var purgedCount, skippedMalformed int
	if len(existing) > 0 {
		var working []byte
		if isMDCPath(mf.Path) {
			if _, body, ferr := splitFrontmatter(existing); ferr == nil {
				working = body
			} else {
				working = existing
			}
		} else {
			working = existing
		}
		_, purgedCount, skippedMalformed = purgeLegacyBlocks(working)
	}

	body, err := mf.Build(params)
	if err != nil {
		return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
	}
	canonicalBlock := renderV2Block(body)

	var newContent []byte
	var action Action
	switch state.State {
	case StateSynced:
		// Skip the main atomicWrite below — Synced means no body rewrite
		// needed. But if purgeLegacyBlocks removed something, we MUST
		// write purgedAfter back (the file changed) and that's
		// ActionRefreshed, not ActionNoOp.
		if purgedCount > 0 && !bytes.Equal(existing, purgedAfter) {
			mode := preserveMode(full)
			if werr := atomicWrite(full, purgedAfter, mode); werr != nil {
				return SyncResult{Path: mf.Path, Action: ActionError, Err: werr}, nil
			}
			action = ActionRefreshed
		} else {
			action = ActionNoOp
		}
		// Fall through to the supersedes block; both NoOp and Refreshed
		// should run the guarded delete so steady-state cleanup happens
		// even when the .mdc is already in canonical shape.
		newContent = nil // signal: main atomicWrite below should be skipped

	case StateMissing:
		// Build full file: frontmatter (if .mdc) + canonical block.
		if isMDCPath(mf.Path) {
			newContent = []byte(defaultCursorFrontmatter)
			newContent = append(newContent, canonicalBlock...)
		} else {
			newContent = canonicalBlock
		}
		if !bytes.HasSuffix(newContent, []byte("\n")) {
			newContent = append(newContent, '\n')
		}
		action = ActionCreated

	case StateStale:
		if state.Detail == "no markers" {
			// File exists but has no init block — append.
			newContent = appendBlockToExisting(purgedAfter, canonicalBlock)
			action = ActionCreated
		} else {
			newContent = replaceInitBlock(purgedAfter, canonicalBlock)
			action = ActionRefreshed
		}

	case StateDrifted:
		if !opts.Force {
			return SyncResult{Path: mf.Path, Action: ActionSkipped, Detail: state.Detail}, nil
		}
		if opts.KeepEdits {
			// Refresh sentinel hash to match disk; keep user body.
			newContent = refreshSentinelToDisk(purgedAfter)
			action = ActionForced
		} else {
			newContent = replaceInitBlock(purgedAfter, canonicalBlock)
			action = ActionForced
		}
	}

	// Write canonical content. Skipped for the StateSynced+no-purge
	// path which signals nil newContent.
	if newContent != nil {
		mode := preserveMode(full)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
		}
		if err := atomicWrite(full, newContent, mode); err != nil {
			return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
		}
	}

	res := SyncResult{Path: mf.Path, Action: action}
	switch {
	case purgedCount > 0 && skippedMalformed > 0:
		res.Detail = fmt.Sprintf("purged %d legacy block%s; skipped %d malformed",
			purgedCount, pluralS(purgedCount), skippedMalformed)
	case purgedCount > 0:
		res.Detail = fmt.Sprintf("purged %d legacy block%s", purgedCount, pluralS(purgedCount))
	case skippedMalformed > 0:
		res.Detail = fmt.Sprintf("skipped %d malformed legacy block%s",
			skippedMalformed, pluralS(skippedMalformed))
	case state.Detail != "" && state.Detail != "no markers":
		res.Detail = state.Detail
	}

	// Supersedes-guarded delete. Runs on Created, Refreshed, Forced,
	// and NoOp; skips Skipped and Error.
	if mf.SupersedesPath != "" && (action == ActionCreated || action == ActionRefreshed || action == ActionForced || action == ActionNoOp) {
		priorCanonical := computePriorCanonical(mf, params)
		priorHash := hashBytes(priorCanonical)
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

func preserveMode(full string) os.FileMode {
	mode := os.FileMode(0o600)
	if info, statErr := os.Stat(full); statErr == nil {
		mode = info.Mode().Perm()
	}
	return mode
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// markdownBlockClassify returns (FileState, existing-bytes,
// post-purge-bytes, error). The two byte-blobs are conveniences for
// Sync; Inspect ignores them.
//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func markdownBlockClassify(cwd string, mf ManagedFile, params ProjectParams) (FileState, []byte, []byte, error) {
	if err := rejectSymlinkComponents(cwd, mf.Path); err != nil {
		return FileState{}, nil, nil, err
	}
	full := filepath.Join(cwd, mf.Path)
	existing, rerr := readFileNoFollow(full)
	switch {
	case noFollowIsSymlink(rerr):
		return FileState{}, nil, nil, fmt.Errorf("%w: %s", ErrSymlinkRejected, full)
	case errors.Is(rerr, fs.ErrNotExist):
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateMissing}, nil, nil, nil
	case rerr != nil:
		return FileState{}, nil, nil, fmt.Errorf("read %s: %w", full, rerr)
	}

	// For .mdc, split frontmatter off so marker validation and purge
	// operate on the body.
	var front, working []byte
	if isMDCPath(mf.Path) {
		f, b, ferr := splitFrontmatter(existing)
		if ferr != nil {
			return FileState{}, nil, nil, ferr
		}
		front, working = f, b
	} else {
		front, working = nil, existing
	}

	if err := validateInitMarkers(mf.Path, working); err != nil {
		return FileState{}, nil, nil, err
	}
	purged, _, _ := purgeLegacyBlocks(working)
	purgedFull := append([]byte{}, front...)
	purgedFull = append(purgedFull, purged...)

	// Body present?
	body, ok := extractManagedBlockBody(purged)
	if !ok {
		// No init block in the file.
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateStale, Detail: "no markers"}, existing, purgedFull, nil
	}

	// Compare disk body hash against sentinel-recorded hash.
	startIdx := initStartAnyVersion.FindIndex(purged)
	if startIdx == nil {
		// Defensive: extractManagedBlockBody just confirmed exactly one
		// start marker exists, so this should be unreachable. If a
		// future change invalidates that invariant, surface as
		// corruption rather than panicking on nil-deref.
		return FileState{}, nil, nil, fmt.Errorf("%w: %s: start marker matched once for body extract but not for sentinel parse", ErrCorruptedMarkers, mf.Path)
	}
	sentinelLine := string(purged[startIdx[0]:startIdx[1]])
	sentinel, perr := ParseSentinel(CommentHTML, sentinelLine)
	if perr != nil {
		return FileState{}, nil, nil, perr
	}

	canonicalBody, berr := mf.Build(params)
	if berr != nil {
		return FileState{}, nil, nil, berr
	}
	diskHash := hashBytes(body)
	canonicalHash := hashBytes(trimEdgeNewlines(canonicalBody))

	switch sentinel.Version {
	case 1:
		// v=1 has no recorded hash to compare against. Two ways a v=1 block
		// is considered "untouched and eligible for upgrade":
		//   (a) the disk body matches the current canonical Build, or
		//   (b) the disk body matches the vestigial v=1 renderer's body.
		// Either way, the user's content matches a known canonical; rewrite
		// with v=2 markers. Otherwise the user has hand-edited the block —
		// surface as Drifted so --force is required.
		var v1Body []byte
		if mf.Path == "AGENTS.md" {
			v1Body = renderV1AgentsBlockBody(params)
		} else {
			v1Body = renderV1CursorBlockBody(params)
		}
		if bytes.Equal(body, trimEdgeNewlines(canonicalBody)) ||
			bytes.Equal(body, trimEdgeNewlines(v1Body)) {
			return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateStale, Detail: "v=1 upgrade"}, existing, purgedFull, nil
		}
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "v=1 user-edited"}, existing, purgedFull, nil

	case 2:
		if sentinel.SHA256 != diskHash {
			return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "sentinel hash != disk hash"}, existing, purgedFull, nil
		}
		if diskHash != canonicalHash {
			return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateStale}, existing, purgedFull, nil
		}
		return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateSynced}, existing, purgedFull, nil
	}
	return FileState{Path: mf.Path, Strategy: mf.Strategy, State: StateDrifted, Detail: "unknown sentinel version"}, existing, purgedFull, nil
}

// renderV2Block emits the canonical v=2 marker pair around body. The
// recorded sha256 is the hash of the body as extractManagedBlockBody
// will re-extract it (i.e. with the one newline adjacent to each marker
// stripped). Callers that pass a body already free of edge newlines get
// the same output as callers who pass body with a leading or trailing
// newline; the layout on disk is:
//
//	<!-- specgraph:init:start v=2 sha256=H -->\n<trimmed-body>\n<!-- specgraph:init:end -->
//
// where H = sha256(trimmed-body). This keeps the recorded hash and the
// hash recomputed by Inspect (over extractManagedBlockBody's output)
// byte-identical.
func renderV2Block(body []byte) []byte {
	trimmed := body
	if len(trimmed) > 0 && trimmed[0] == '\n' {
		trimmed = trimmed[1:]
	}
	if len(trimmed) > 0 && trimmed[len(trimmed)-1] == '\n' {
		trimmed = trimmed[:len(trimmed)-1]
	}
	hash := hashBytes(trimmed)
	var b bytes.Buffer
	b.WriteString("<!-- specgraph:init:start v=2 sha256=")
	b.WriteString(hash)
	b.WriteString(" -->\n")
	b.Write(trimmed)
	b.WriteString("\n")
	b.WriteString(initEndMarker)
	return b.Bytes()
}

func replaceInitBlock(data, canonicalBlock []byte) []byte {
	startLoc := initStartAnyVersion.FindIndex(data)
	endIdx := bytes.Index(data, []byte(initEndMarker))
	if startLoc == nil || endIdx < 0 {
		return data
	}
	endLen := len(initEndMarker)
	out := make([]byte, 0, len(data)+len(canonicalBlock))
	out = append(out, data[:startLoc[0]]...)
	out = append(out, canonicalBlock...)
	out = append(out, data[endIdx+endLen:]...)
	return out
}

func appendBlockToExisting(existing, canonicalBlock []byte) []byte {
	if len(existing) == 0 {
		out := append([]byte{}, canonicalBlock...)
		return append(out, '\n')
	}
	out := append([]byte{}, existing...)
	if !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, '\n')
	}
	out = append(out, '\n')
	out = append(out, canonicalBlock...)
	out = append(out, '\n')
	return out
}

func refreshSentinelToDisk(existing []byte) []byte {
	body, ok := extractManagedBlockBody(existing)
	if !ok {
		return existing
	}
	return replaceInitBlock(existing, renderV2Block(body))
}

func isMDCPath(p string) bool { return filepath.Ext(p) == ".mdc" }

// computePriorCanonical returns the byte sequence the deleted pointers/
// package would have written at mf.SupersedesPath. Used to hash-compare
// against the on-disk supersedes file.
//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the strategy interface
func computePriorCanonical(mf ManagedFile, params ProjectParams) []byte {
	if mf.SupersedesPath != ".cursor/rules/specgraph-bootstrap.md" {
		// PR B has only one SupersedesPath; later PRs may add more.
		// Panic loud rather than silently producing zero bytes.
		panic(fmt.Sprintf("no prior-canonical renderer for SupersedesPath %q", mf.SupersedesPath))
	}
	body := renderV1CursorBlockBody(params)
	var b bytes.Buffer
	b.WriteString(defaultCursorFrontmatter)
	b.WriteString("<!-- specgraph:init:start v=1 -->")
	b.Write(body)
	b.WriteString(initEndMarker)
	b.WriteString("\n")
	return b.Bytes()
}

// trimEdgeNewlines strips at most one '\n' from the start and end of b.
// This mirrors extractManagedBlockBody's newline-adjacent-to-marker
// handling so canonical-body hashes computed from a Build closure (or
// vestigial v=1 renderer) match disk-body hashes computed from the
// extractor's output.
func trimEdgeNewlines(b []byte) []byte {
	if len(b) > 0 && b[0] == '\n' {
		b = b[1:]
	}
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}
	return b
}
