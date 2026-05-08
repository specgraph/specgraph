// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build windows

package pointers

import "log/slog"

// acquireFileLock is a no-op on Windows. Real implementation arrives in a
// follow-up commit using LockFileEx from golang.org/x/sys/windows.
func acquireFileLock(path string) (Unlocker, error) {
	slog.Warn("file locking is not implemented on Windows; concurrent specgraph init runs may race", "path", path)
	return func() error { return nil }, nil
}
