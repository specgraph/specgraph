// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package scanner_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/seanb4t/specgraph/internal/scanner"
	"github.com/stretchr/testify/require"
)

// TestTier1Scan_MalformedGoFile verifies that a .go file with invalid syntax
// does not panic and is recorded in SkippedFiles rather than causing an error.
func TestTier1Scan_MalformedGoFile(t *testing.T) {
	dir := t.TempDir()

	// Write a Go file with a syntax error (missing braces after func).
	require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package main\nfunc broken {"), 0o600))

	result, err := scanner.ScanTier1(dir)
	require.NoError(t, err, "ScanTier1 must not return an error for malformed files")
	require.NotNil(t, result)
	require.Len(t, result.SkippedFiles, 1, "malformed file should appear in SkippedFiles")
	require.Equal(t, filepath.Join(dir, "broken.go"), result.SkippedFiles[0].Path)
	require.NotEmpty(t, result.SkippedFiles[0].Reason, "SkippedFile must have a non-empty Reason")
}

// TestTier1Scan_SymlinkToGoFile verifies that the scanner handles a symlink
// pointing to a valid Go file without panicking or returning an error.
// The scanner may either follow the symlink (recording the package) or skip it —
// both outcomes are acceptable as long as there is no crash or error.
func TestTier1Scan_SymlinkToGoFile(t *testing.T) {
	dir := t.TempDir()

	// Create the real Go file.
	realFile := filepath.Join(dir, "real.go")
	require.NoError(t, os.WriteFile(realFile, []byte("package main\n"), 0o600))

	// Create a symlink pointing to the real file.
	linkFile := filepath.Join(dir, "link.go")
	if err := os.Symlink(realFile, linkFile); err != nil {
		t.Skipf("cannot create symlink (OS restriction): %v", err)
	}

	result, err := scanner.ScanTier1(dir)
	require.NoError(t, err, "ScanTier1 must not error on a directory containing symlinks")
	require.NotNil(t, result)
	// The scanner processes .go files by path suffix; both the real file and the
	// symlink have the suffix, so the package may appear once or twice depending
	// on deduplication. Either way, no panic and no error is the key assertion.
}

// TestTier1Scan_UnreadableSubdirectory verifies that the scanner skips a
// subdirectory it cannot read and continues scanning the rest of the tree.
// Skipped on Windows where chmod 000 has no effect.
func TestTier1Scan_UnreadableSubdirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 is not effective on Windows")
	}

	dir := t.TempDir()

	// Create a readable package in the root dir.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "good.go"), []byte("package root\n"), 0o600))

	// Create a subdirectory we'll make unreadable.
	secretDir := filepath.Join(dir, "secret")
	require.NoError(t, os.MkdirAll(secretDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(secretDir, "hidden.go"), []byte("package secret\n"), 0o600))

	// Make the subdirectory unreadable/untraversable.
	require.NoError(t, os.Chmod(secretDir, 0o000))
	t.Cleanup(func() {
		// Restore permissions so t.TempDir() cleanup can remove the directory.
		_ = os.Chmod(secretDir, 0o750) //nolint:gosec // restoring permissions for temp dir cleanup
	})

	result, err := scanner.ScanTier1(dir)
	require.NoError(t, err, "ScanTier1 must not error when a subdirectory is unreadable")
	require.NotNil(t, result)

	// The root package should still be found.
	var foundRoot bool
	for _, pkg := range result.Packages {
		if pkg.Name == "root" {
			foundRoot = true
		}
		// The secret package must not appear since the directory is unreadable.
		require.NotEqual(t, "secret", pkg.Name, "secret package must not be scanned through unreadable dir")
	}
	require.True(t, foundRoot, "root package from readable file must still be found")
}
