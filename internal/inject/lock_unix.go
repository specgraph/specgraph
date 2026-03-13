// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build !windows

package inject

import (
	"fmt"
	"log/slog"
	"os"
	"syscall"
)

// acquireFileLock acquires an exclusive advisory lock on a lock file adjacent to path.
// Returns an unlock function that must be called to release the lock.
func acquireFileLock(path string) (func(), error) {
	lockPath := path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create lock file: %w", err)
	}
	fd := int(lockFile.Fd()) //nolint:gosec // G115: Fd() returns a valid file descriptor; overflow is not possible on supported platforms
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		lockFile.Close() //nolint:errcheck // lock acquisition failed; close is best-effort
		return nil, fmt.Errorf("acquire file lock: %w", err)
	}
	return func() {
		if unlockErr := syscall.Flock(fd, syscall.LOCK_UN); unlockErr != nil {
			slog.Error("failed to release AGENTS.md lock", "path", path, "error", unlockErr)
		}
		lockFile.Close() //nolint:errcheck // best-effort close after unlock
	}, nil
}
