// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWarnLooseCredentialFile_LoosePermsWarns asserts a group/other-readable
// credentials file produces a warning naming the path.
func TestWarnLooseCredentialFile_LoosePermsWarns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.yaml")
	//nolint:gosec // intentionally group/other-readable to exercise the warning path
	if err := os.WriteFile(path, []byte("servers: {}\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var buf bytes.Buffer
	warnLooseCredentialFile(&buf, path)

	out := buf.String()
	if !strings.Contains(out, path) {
		t.Errorf("expected warning naming %q, got %q", path, out)
	}
	if !strings.Contains(out, "0600") {
		t.Errorf("expected warning to recommend 0600, got %q", out)
	}
}

// TestWarnLooseCredentialFile_SecurePermsSilent asserts a 0600 file produces
// no output.
func TestWarnLooseCredentialFile_SecurePermsSilent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.yaml")
	if err := os.WriteFile(path, []byte("servers: {}\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var buf bytes.Buffer
	warnLooseCredentialFile(&buf, path)

	if out := buf.String(); out != "" {
		t.Errorf("expected no warning for 0600 file, got %q", out)
	}
}

// TestWarnLooseCredentialFile_MissingFileSilent asserts a nonexistent file
// produces no output (the common case: no credentials yet).
func TestWarnLooseCredentialFile_MissingFileSilent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	var buf bytes.Buffer
	warnLooseCredentialFile(&buf, path)

	if out := buf.String(); out != "" {
		t.Errorf("expected no warning for missing file, got %q", out)
	}
}
