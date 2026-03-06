// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package scanner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanb4t/specgraph/internal/scanner"
	"github.com/stretchr/testify/require"
)

func TestTier2Scan(t *testing.T) {
	dir := t.TempDir()

	svcDir := filepath.Join(dir, "internal", "svc")
	require.NoError(t, os.MkdirAll(svcDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(svcDir, "handler.go"), []byte(`package svc

import (
	"context"
	"fmt"
)

func HandleRequest(ctx context.Context, id string) (string, error) {
	return fmt.Sprintf("handled %s", id), nil
}

func helperFunc(x int) int {
	return x + 1
}
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(svcDir, "handler_test.go"), []byte(`package svc_test

import "testing"

func TestHandleRequest(t *testing.T) {}
`), 0o600))

	result, err := scanner.ScanTier2(dir, []string{filepath.Join("internal", "svc")})
	require.NoError(t, err)

	// Verify functions found.
	require.NotEmpty(t, result.Functions)
	var foundExported, foundUnexported bool
	for _, fn := range result.Functions {
		if fn.Name == "HandleRequest" {
			foundExported = true
			require.True(t, fn.IsExported)
			require.Equal(t, "svc", fn.Package)
			require.Equal(t, filepath.Join("internal", "svc", "handler.go"), fn.File)
			require.Len(t, fn.Params, 2)
		}
		if fn.Name == "helperFunc" {
			foundUnexported = true
			require.False(t, fn.IsExported)
		}
	}
	require.True(t, foundExported, "expected to find exported function 'HandleRequest'")
	require.True(t, foundUnexported, "expected to find unexported function 'helperFunc'")

	// Verify test files found.
	require.Len(t, result.TestFiles, 1)
	require.Equal(t, filepath.Join("internal", "svc", "handler_test.go"), result.TestFiles[0])

	// Verify imports found.
	require.NotEmpty(t, result.Imports)
	var foundCtx, foundFmt bool
	for _, imp := range result.Imports {
		if imp.Path == "context" {
			foundCtx = true
		}
		if imp.Path == "fmt" {
			foundFmt = true
		}
	}
	require.True(t, foundCtx, "expected to find import 'context'")
	require.True(t, foundFmt, "expected to find import 'fmt'")
}

func TestTier2Scan_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	emptyDir := filepath.Join(dir, "empty")
	require.NoError(t, os.MkdirAll(emptyDir, 0o750))

	result, err := scanner.ScanTier2(dir, []string{"empty"})
	require.NoError(t, err)
	require.Empty(t, result.Functions)
	require.Empty(t, result.TestFiles)
	require.Empty(t, result.Imports)
}

func TestTier2Scan_NonExistentDir(t *testing.T) {
	root := t.TempDir()

	// "missing" does not exist under root; walkGoFiles will receive an error
	// from WalkDir on the first call and record it in SkippedFiles.
	result, err := scanner.ScanTier2(root, []string{"missing"})
	require.NoError(t, err, "non-existent dir should be recorded in SkippedFiles, not return an error")
	require.Empty(t, result.Functions)
	require.Empty(t, result.TestFiles)
	require.Empty(t, result.Imports)
	require.Len(t, result.SkippedFiles, 1, "expected missing dir to appear in SkippedFiles")
	require.Contains(t, result.SkippedFiles[0].Path, "missing")
}

func TestTier2Scan_PathTraversalRejected(t *testing.T) {
	root := t.TempDir()

	// "../../etc" escapes root; ScanTier2 should return an error.
	_, err := scanner.ScanTier2(root, []string{"../../etc"})
	require.Error(t, err, "traversal dir should be rejected")
	require.Contains(t, err.Error(), "escapes root")
}

func TestTier2Scan_NonExistentRoot(t *testing.T) {
	_, err := scanner.ScanTier2(filepath.Join(t.TempDir(), "does-not-exist"), []string{})
	require.Error(t, err, "non-existent root should return an error")
}
