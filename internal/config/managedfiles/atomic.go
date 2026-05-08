// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

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
