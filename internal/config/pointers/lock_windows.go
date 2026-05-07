// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build windows

package pointers

import "log/slog"

// acquireFileLock is a no-op on Windows. ADR posture: native Windows is
// best-effort; concurrent specgraph init runs on Windows are not serialized
// at the lock layer. The atomic-rename in atomicWrite still prevents a
// partially-purged file on disk; the worst case is "last writer wins".
func acquireFileLock(path string) (func(), error) {
	slog.Warn("file locking is not implemented on Windows; concurrent specgraph init runs may race", "path", path)
	return func() {}, nil
}
