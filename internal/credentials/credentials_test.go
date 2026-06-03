// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package credentials_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/credentials"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.yaml")

	f := &credentials.File{}
	f.Upsert("https://api.example.com", credentials.ServerCreds{
		Token: "tok-123",
		Label: "prod",
	})

	if err := f.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("perm = %o, want 0600", perm)
	}

	loaded, err := credentials.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := loaded.TokenFor("https://api.example.com"); got != "tok-123" {
		t.Fatalf("TokenFor = %q, want tok-123", got)
	}
}

func TestUpsertPreservesOtherServers(t *testing.T) {
	f := &credentials.File{}
	f.Upsert("https://a.example.com", credentials.ServerCreds{Token: "tok-a"})
	f.Upsert("https://b.example.com", credentials.ServerCreds{Token: "tok-b"})

	// Overwrite a without disturbing b.
	f.Upsert("https://a.example.com", credentials.ServerCreds{Token: "tok-a2"})

	if got := f.TokenFor("https://a.example.com"); got != "tok-a2" {
		t.Fatalf("TokenFor(a) = %q, want tok-a2", got)
	}
	if got := f.TokenFor("https://b.example.com"); got != "tok-b" {
		t.Fatalf("TokenFor(b) = %q, want tok-b", got)
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.yaml")

	f, err := credentials.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if f == nil {
		t.Fatal("Load returned nil File")
	}
	if got := f.TokenFor("https://anything"); got != "" {
		t.Fatalf("TokenFor on empty = %q, want empty", got)
	}
}

func TestLoadOldShapeYieldsNoServers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.yaml")

	old := "api_keys:\n  - id: foo\n    key: sg_oldkey\n"
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	f, err := credentials.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := f.TokenFor("https://anything"); got != "" {
		t.Fatalf("TokenFor on old-shape = %q, want empty", got)
	}
}

func TestTokenForNormalizesTrailingSlash(t *testing.T) {
	f := &credentials.File{}
	f.Upsert("https://api.example.com/", credentials.ServerCreds{Token: "tok-x"})

	if got := f.TokenFor("https://api.example.com"); got != "tok-x" {
		t.Fatalf("TokenFor(no slash) = %q, want tok-x", got)
	}
	if got := f.TokenFor("https://api.example.com/"); got != "tok-x" {
		t.Fatalf("TokenFor(slash) = %q, want tok-x", got)
	}
}

func TestCheckPermissions(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.yaml")
	require.NoError(t, os.WriteFile(ok, []byte("servers: {}\n"), 0o600))
	require.Empty(t, credentials.CheckPermissions(ok), "0600 is fine")

	loose := filepath.Join(dir, "loose.yaml")
	require.NoError(t, os.WriteFile(loose, []byte("servers: {}\n"), 0o644)) //nolint:gosec // intentionally loose perms to exercise the warning
	require.NotEmpty(t, credentials.CheckPermissions(loose), "group/other-readable must warn")

	require.Empty(t, credentials.CheckPermissions(filepath.Join(dir, "absent.yaml")), "missing file: no warning")

	// stricter owner-only modes are accepted, not flagged
	strict := filepath.Join(dir, "strict.yaml")
	require.NoError(t, os.WriteFile(strict, []byte("servers: {}\n"), 0o400))
	require.Empty(t, credentials.CheckPermissions(strict), "0400 (owner read-only) is fine")
}
