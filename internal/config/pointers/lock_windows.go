// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build windows

package pointers

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// acquireFileLock acquires an exclusive lock on a sibling file <path>.lock
// via LockFileEx. Returns an Unlocker that calls UnlockFileEx and closes
// the handle. The lock file is intentionally not removed (see lock_unix.go
// for the rationale).
func acquireFileLock(path string) (Unlocker, error) {
	lockPath := path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create lock file: %w", err)
	}
	handle := windows.Handle(lockFile.Fd())
	var ol windows.Overlapped
	if err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &ol); err != nil {
		return nil, errors.Join(
			fmt.Errorf("acquire file lock: %w", err),
			lockFile.Close(),
		)
	}
	return func() error {
		var ol windows.Overlapped
		uerr := windows.UnlockFileEx(handle, 0, 1, 0, &ol)
		cerr := lockFile.Close()
		if uerr != nil || cerr != nil {
			return errors.Join(uerr, cerr)
		}
		return nil
	}, nil
}
