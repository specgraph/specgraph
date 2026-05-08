// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package pointers

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Action describes what Sync did to a single managed pointer file. The string
// values are deliberately identical to mcpconfigs.Action values so init can
// render a unified "<path>: <action>" output.
type Action string

// Action values mirror mcpconfigs.Action string tags for unified init output.
const (
	ActionCreated Action = "created"
	ActionUpdated Action = "updated"
	ActionNoOp    Action = "no-op"
	ActionError   Action = "error"
)

// Unlocker releases a file lock acquired via acquireFileLock. It returns
// any error from the underlying flock LOCK_UN (Unix) or LockFileEx
// release (Windows) plus any error closing the lock-file handle. Callers
// MUST invoke Unlocker via a deferred wrapper that propagates the error.
type Unlocker func() error

// SyncResult reports what Sync did to a single managed pointer file.
//
// Invariant: Action == ActionError ⇔ Err != nil. Constructed via
// errResult / okResult / noopResult; do not build SyncResult literals
// outside this package.
//
// LegacyBlocksPurged is the number of pre-init per-slug blocks removed
// from AGENTS.md. Always 0 for the cursor pointer file. Meaningful only
// when Action == ActionUpdated or ActionCreated.
//
// LegacyBlocksSkippedMalformed counts legacy slug-pair blocks that were
// detected but not purged because the start and end slugs differ. They
// remain in the file; the user must repair manually.
type SyncResult struct {
	Path                         string
	Action                       Action
	Err                          error
	LegacyBlocksPurged           int
	LegacyBlocksSkippedMalformed int
}

// Options carries the canonical values that init derives once and threads
// into the pointer templates. Construct via NewOptions to validate inputs.
type Options struct {
	ServerURL   string
	ProjectSlug string
}

// safeSlugPattern mirrors the inject-era slug class:
// `[a-zA-Z0-9][a-zA-Z0-9._-]*`. Slugs flow into the rendered managed
// block; rejecting newlines and marker-shaped fragments keeps the block
// syntactically inviolable.
var safeSlugPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// NewOptions validates serverURL and projectSlug then returns a usable
// Options. serverURL must be an absolute http or https URL with a non-empty
// host. projectSlug must match safeSlugPattern.
func NewOptions(serverURL, projectSlug string) (Options, error) {
	parsed, perr := url.Parse(serverURL)
	if perr != nil {
		return Options{}, fmt.Errorf("server URL %q: %w", serverURL, perr)
	}
	if parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return Options{}, fmt.Errorf("server URL %q must be an absolute http or https URL", serverURL)
	}
	if !safeSlugPattern.MatchString(projectSlug) {
		return Options{}, fmt.Errorf("project slug %q does not match %s", projectSlug, safeSlugPattern)
	}
	return Options{ServerURL: serverURL, ProjectSlug: projectSlug}, nil
}

// SyncReport is the per-file outcome of a Sync. The two fields are
// always populated except in the projectDir-level early-error case where
// Agents carries the projectDir error and Cursor is the zero value.
type SyncReport struct {
	Agents SyncResult
	Cursor SyncResult
}

// IsErr reports whether either pointer file failed.
func (r *SyncReport) IsErr() bool {
	return r.Agents.Action == ActionError || r.Cursor.Action == ActionError
}

// Sync reconciles all pointer files for the project. Returns a SyncReport
// with one SyncResult per pointer file.
//
// If projectDir is missing, is not a directory, or is itself a symlink,
// Sync short-circuits: report.Agents carries the projectDir-level error
// (Path == projectDir, Action == ActionError) and report.Cursor is the
// zero value.
//
// A failure on one pointer file is reported via the corresponding
// SyncResult.Err with Action == ActionError; the other file is still
// processed. This differs from mcpconfigs.Sync which aborts on first
// error. The init caller reconciles by running mcpconfigs first and only
// invoking pointers.Sync if mcpconfigs succeeded.
func Sync(projectDir string, opts Options) SyncReport {
	info, err := os.Stat(projectDir)
	if err != nil || !info.IsDir() {
		msg := fmt.Errorf("projectDir %q is not a directory", projectDir)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			msg = fmt.Errorf("stat %s: %w", projectDir, err)
		}
		return SyncReport{Agents: errResult(projectDir, msg)}
	}
	li, lerr := os.Lstat(projectDir)
	if lerr != nil {
		return SyncReport{Agents: errResult(projectDir, fmt.Errorf("lstat %s: %w", projectDir, lerr))}
	}
	if li.Mode()&os.ModeSymlink != 0 {
		return SyncReport{Agents: errResult(projectDir, fmt.Errorf("%w: %s", ErrSymlinkRejected, projectDir))}
	}
	return SyncReport{
		Agents: syncAgents(projectDir, opts),
		Cursor: syncCursor(projectDir, opts),
	}
}

func errResult(path string, err error) SyncResult {
	return SyncResult{Path: path, Action: ActionError, Err: err}
}

func okResult(path string, action Action, purged, skipped int) SyncResult {
	return SyncResult{Path: path, Action: action, LegacyBlocksPurged: purged, LegacyBlocksSkippedMalformed: skipped}
}

func noopResult(path string) SyncResult {
	return SyncResult{Path: path, Action: ActionNoOp}
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
			return fmt.Errorf("%w: %s", ErrSymlinkRejected, cur)
		}
	}
	return nil
}

// atomicWrite writes data to <fullPath>.tmp.<random> in the same directory,
// fsyncs, then renames over fullPath. The directory is fsynced after the
// rename so the rename itself is durable across power loss.
//
// The temp file is chmod'd to mode. On a fresh write, callers pass 0o600.
// On an update, callers should read the existing file's mode and pass it so
// user-set permissions (e.g. 0o644) are preserved.
//
// All cleanup errors are joined to the original error via errors.Join so
// the caller sees both the proximate failure and any stranded-tempfile
// fallout.
func atomicWrite(fullPath string, data []byte, mode os.FileMode) error {
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
		return errors.Join(
			fmt.Errorf("write temp: %w", werr),
			tmp.Close(),
			os.Remove(tmpName),
		)
	}
	if cerr := tmp.Chmod(mode); cerr != nil {
		return errors.Join(
			fmt.Errorf("chmod temp: %w", cerr),
			tmp.Close(),
			os.Remove(tmpName),
		)
	}
	if serr := tmp.Sync(); serr != nil {
		return errors.Join(
			fmt.Errorf("fsync temp: %w", serr),
			tmp.Close(),
			os.Remove(tmpName),
		)
	}
	if cerr := tmp.Close(); cerr != nil {
		return errors.Join(
			fmt.Errorf("close temp: %w", cerr),
			os.Remove(tmpName),
		)
	}
	if rerr := os.Rename(tmpName, fullPath); rerr != nil {
		return errors.Join(
			fmt.Errorf("rename: %w", rerr),
			os.Remove(tmpName),
		)
	}
	if dirF, derr := os.Open(dir); derr == nil {
		_ = dirF.Sync()  //nolint:errcheck // best-effort dir fsync; not fatal if unsupported
		_ = dirF.Close() //nolint:errcheck // best-effort cleanup
	}
	return nil
}
