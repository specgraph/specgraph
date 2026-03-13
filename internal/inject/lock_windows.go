// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build windows

package inject

// acquireFileLock is a no-op on Windows. Advisory file locking is not available
// via syscall.Flock on this platform. Concurrent inject calls on Windows are
// not protected against TOCTOU races.
func acquireFileLock(_ string) (func(), error) {
	return func() {}, nil
}
