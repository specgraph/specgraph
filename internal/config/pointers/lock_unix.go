// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build !windows

package pointers

import (
	"fmt"
	"log/slog"
	"os"
	"syscall"
)

// acquireFileLock acquires an exclusive advisory lock on a sibling file
// <path>.lock. Returns an unlock function that must be called to release.
// The lock file is intentionally never removed: deleting it between unlock
// and a concurrent open creates a new inode and breaks mutual exclusion.
func acquireFileLock(path string) (func(), error) {
	lockPath := path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create lock file: %w", err)
	}
	fd := int(lockFile.Fd())
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		lockFile.Close() //nolint:errcheck // lock acquisition failed
		return nil, fmt.Errorf("acquire file lock: %w", err)
	}
	return func() {
		if uerr := syscall.Flock(fd, syscall.LOCK_UN); uerr != nil {
			slog.Error("failed to release file lock", "path", path, "error", uerr)
		}
		lockFile.Close() //nolint:errcheck // best-effort close after unlock
	}, nil
}
