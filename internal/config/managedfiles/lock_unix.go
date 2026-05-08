// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build !windows

package managedfiles

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// acquireFileLock acquires an exclusive advisory lock on a sibling file
// <path>.lock. Returns an Unlocker that must be called to release.
// The lock file is intentionally never removed: deleting it between unlock
// and a concurrent open creates a new inode and breaks mutual exclusion.
func acquireFileLock(path string) (Unlocker, error) {
	lockPath := path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create lock file: %w", err)
	}
	fd := int(lockFile.Fd())
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		return nil, errors.Join(
			fmt.Errorf("acquire file lock: %w", err),
			lockFile.Close(),
		)
	}
	return func() error {
		uerr := syscall.Flock(fd, syscall.LOCK_UN)
		cerr := lockFile.Close()
		if uerr != nil || cerr != nil {
			return errors.Join(uerr, cerr)
		}
		return nil
	}, nil
}
