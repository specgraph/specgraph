// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRejectSymlinkComponents_NoSymlinks(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := rejectSymlinkComponents(dir, "nested/foo.txt"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRejectSymlinkComponents_DetectsSymlinkInPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires admin on Windows")
	}
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	if err := os.Mkdir(realDir, 0o750); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatal(err)
	}

	err := rejectSymlinkComponents(dir, "link/foo.txt")
	if !errors.Is(err, ErrSymlinkRejected) {
		t.Errorf("got %v, want ErrSymlinkRejected", err)
	}
}

func TestRejectSymlinkComponents_NonExistentComponentsAllowed(t *testing.T) {
	// Allow paths whose terminal components don't exist yet — init writes
	// new files, so non-existence at the leaf is normal.
	dir := t.TempDir()
	if err := rejectSymlinkComponents(dir, "does/not/exist.txt"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
