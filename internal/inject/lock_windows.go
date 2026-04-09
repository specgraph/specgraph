// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build windows

package inject

import "log/slog"

// acquireFileLock is a no-op on Windows. Advisory file locking is not available
// via syscall.Flock on this platform. Concurrent inject calls on Windows are
// not protected against TOCTOU races.
func acquireFileLock(path string) (func(), error) {
	slog.Warn("file locking not available on Windows — concurrent inject calls are unprotected",
		"path", path)
	return func() {}, nil
}
