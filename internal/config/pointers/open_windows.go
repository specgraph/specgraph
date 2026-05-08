// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build windows

package pointers

import "os"

// readFileNoFollow falls back to os.ReadFile on Windows; symlink rejection
// is handled by the rejectSymlinkComponents walk only. Documented in doc.go
// as best-effort, not a security boundary.
func readFileNoFollow(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// noFollowIsSymlink is always false on Windows.
func noFollowIsSymlink(_ error) bool { return false }
