// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSupersedesGuardedDelete_NoFile(t *testing.T) {
	dir := t.TempDir()
	// Old path doesn't exist — guarded delete is a no-op success.
	if err := supersedesGuardedDelete(dir, "missing.md", "anyhash"); err != nil {
		t.Errorf("expected nil for missing file, got %v", err)
	}
}

func TestSupersedesGuardedDelete_HashMatches_DeletesFile(t *testing.T) {
	dir := t.TempDir()
	oldPath := "old.md"
	full := filepath.Join(dir, oldPath)
	content := "old canonical"
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	expected := HashExcludingSentinel(CommentNone, []byte(content))
	if err := supersedesGuardedDelete(dir, oldPath, expected); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if _, err := os.Stat(full); !os.IsNotExist(err) {
		t.Errorf("file should be deleted, stat err = %v", err)
	}
}

func TestSupersedesGuardedDelete_HashMismatch_LeavesFile(t *testing.T) {
	dir := t.TempDir()
	oldPath := "old.md"
	full := filepath.Join(dir, oldPath)
	if err := os.WriteFile(full, []byte("user-edited content"), 0o600); err != nil {
		t.Fatal(err)
	}

	expected := "0000000000000000000000000000000000000000000000000000000000000000" // wrong
	err := supersedesGuardedDelete(dir, oldPath, expected)
	if !errors.Is(err, ErrPriorCanonicalMismatch) {
		t.Errorf("got %v, want ErrPriorCanonicalMismatch", err)
	}
	if _, statErr := os.Stat(full); statErr != nil {
		t.Errorf("file should NOT be deleted on mismatch, stat err = %v", statErr)
	}
}
