// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/specgraph/specgraph/internal/auth"
	"gopkg.in/yaml.v3"
)

func TestBootstrap_GeneratesKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.yaml")

	key, err := auth.Bootstrap(path)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if key == "" {
		t.Fatal("Bootstrap returned empty key")
	}
	if len(key) != 40 {
		t.Errorf("key length = %d, want 40", len(key))
	}
	if key[:8] != "spgr_sk_" {
		t.Errorf("key prefix = %q, want spgr_sk_", key[:8])
	}

	// Verify file exists with correct permissions.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat credentials: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("permissions = %o, want 600", info.Mode().Perm())
	}

	// Verify file content is valid YAML with the key.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read credentials: %v", err)
	}
	var creds auth.CredentialsFile
	if err := yaml.Unmarshal(data, &creds); err != nil {
		t.Fatalf("unmarshal credentials: %v", err)
	}
	if len(creds.APIKeys) != 1 {
		t.Fatalf("api_keys count = %d, want 1", len(creds.APIKeys))
	}
	if creds.APIKeys[0].Key != key {
		t.Errorf("stored key = %q, want %q", creds.APIKeys[0].Key, key)
	}
	if creds.APIKeys[0].Role != "admin" {
		t.Errorf("role = %q, want admin", creds.APIKeys[0].Role)
	}
}

func TestBootstrap_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.yaml")

	key1, err := auth.Bootstrap(path)
	if err != nil {
		t.Fatalf("Bootstrap first: %v", err)
	}

	key2, err := auth.Bootstrap(path)
	if err != nil {
		t.Fatalf("Bootstrap second: %v", err)
	}

	if key1 != key2 {
		t.Errorf("second call returned different key: %q vs %q", key1, key2)
	}
}

func TestBootstrap_PermissionWarning(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses permission checks")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.yaml")

	_, err := auth.Bootstrap(path)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	// Widen permissions.
	if err := os.Chmod(path, 0o644); err != nil { //nolint:gosec // intentional for test
		t.Fatalf("chmod: %v", err)
	}

	warning := auth.CheckCredentialPermissions(path)
	if warning == "" {
		t.Error("expected warning for open permissions")
	}
}

func TestBootstrap_ReadOnlyDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses permission checks")
	}

	dir := t.TempDir()
	readOnlyDir := filepath.Join(dir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0o555); err != nil { //nolint:gosec // intentional for test
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0o750) }) //nolint:gosec // restore perms for cleanup

	credPath := filepath.Join(readOnlyDir, "credentials.yaml")
	_, err := auth.Bootstrap(credPath)
	if err == nil {
		t.Fatal("expected error when writing to read-only directory")
	}
}

func TestBootstrap_EmptyExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.yaml")

	// Write a credentials file with no keys.
	if err := os.WriteFile(path, []byte("api_keys: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// File already exists, so O_EXCL fails → readExistingBootstrapKey is called → no keys → error.
	_, err := auth.Bootstrap(path)
	if err == nil {
		t.Fatal("expected error for empty existing credentials")
	}
}

func TestReadDefaultKey_ReturnsKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.yaml")

	key, err := auth.Bootstrap(path)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	got, err := auth.ReadDefaultKey(path)
	if err != nil {
		t.Fatalf("ReadDefaultKey: %v", err)
	}
	if got != key {
		t.Errorf("ReadDefaultKey = %q, want %q", got, key)
	}
}

func TestReadDefaultKey_MissingFile(t *testing.T) {
	got, err := auth.ReadDefaultKey("/nonexistent/credentials.yaml")
	if err != nil {
		t.Fatalf("ReadDefaultKey: %v", err)
	}
	if got != "" {
		t.Errorf("ReadDefaultKey = %q, want empty for missing file", got)
	}
}

func TestCheckCredentialPermissions_NoFile(t *testing.T) {
	warning := auth.CheckCredentialPermissions("/nonexistent/file")
	if warning != "" {
		t.Errorf("expected empty warning for missing file, got %q", warning)
	}
}

func TestCheckCredentialPermissions_CorrectPerms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.yaml")
	if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	warning := auth.CheckCredentialPermissions(path)
	if warning != "" {
		t.Errorf("expected empty warning for 0600 file, got %q", warning)
	}
}
