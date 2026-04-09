// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
)

// credFileYAML builds a credentials.yaml string for test use.
func credFileYAML(id, key, name, role string) string {
	return fmt.Sprintf("api_keys:\n  - id: %s\n    key: %s\n    name: %s\n    role: %s\n", id, key, name, role)
}

func TestConfigStore_ResolveAPIKey(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Test Key", Role: "admin"},
		},
	}
	store, err := auth.NewConfigStore(cfg, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	id, err := store.ResolveAPIKey(context.Background(), "spgr_sk_abc")
	if err != nil {
		t.Fatalf("ResolveAPIKey: %v", err)
	}
	if id.Subject != "apikey:k1" {
		t.Errorf("subject = %q, want apikey:k1", id.Subject)
	}
	if id.Role != auth.RoleAdmin {
		t.Errorf("role = %q, want admin", id.Role)
	}
	if id.Source != "apikey" {
		t.Errorf("source = %q, want apikey", id.Source)
	}
}

func TestConfigStore_UnknownKey(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Test Key", Role: "admin"},
		},
	}
	store, err := auth.NewConfigStore(cfg, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	_, err = store.ResolveAPIKey(context.Background(), "wrong_key")
	if !errors.Is(err, auth.ErrUnknownKey) {
		t.Errorf("err = %v, want ErrUnknownKey", err)
	}
}

func TestConfigStore_HasKeys(t *testing.T) {
	empty, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	if empty.HasKeys() {
		t.Error("HasKeys() = true for empty config")
	}
	withKey, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Key", Role: "reader"},
		},
	}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	if !withKey.HasKeys() {
		t.Error("HasKeys() = false for config with keys")
	}
}

func TestConfigStore_DuplicateKeyID(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Key 1", Role: "admin"},
			{ID: "k1", Key: "spgr_sk_def", Name: "Key 2", Role: "reader"},
		},
	}
	_, err := auth.NewConfigStore(cfg, "")
	if err == nil {
		t.Fatal("expected error for duplicate key ID")
	}
}

func TestConfigStore_DuplicateKeyValue(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_same", Name: "Key 1", Role: "admin"},
			{ID: "k2", Key: "spgr_sk_same", Name: "Key 2", Role: "reader"},
		},
	}
	_, err := auth.NewConfigStore(cfg, "")
	if err == nil {
		t.Fatal("expected error for duplicate key value")
	}
}

func TestConfigStore_UnknownRole(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Key", Role: "nonexistent"},
		},
	}
	_, err := auth.NewConfigStore(cfg, "")
	if err == nil {
		t.Fatal("expected error for unknown role")
	}
}

func TestConfigStore_BlankID(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "", Key: "spgr_sk_abc", Name: "Key", Role: "admin"},
		},
	}
	_, err := auth.NewConfigStore(cfg, "")
	if err == nil {
		t.Fatal("expected error for blank key ID")
	}
}

func TestConfigStore_BlankKey(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "", Name: "Key", Role: "admin"},
		},
	}
	_, err := auth.NewConfigStore(cfg, "")
	if err == nil {
		t.Fatal("expected error for blank key value")
	}
}

func TestConfigStore_CustomRole(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "CI", Role: "ci-readonly"},
		},
		Roles: map[string]config.RoleConfig{
			"ci-readonly": {Permissions: []string{"spec:read", "decision:read"}},
		},
	}
	store, err := auth.NewConfigStore(cfg, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	id, err := store.ResolveAPIKey(context.Background(), "spgr_sk_abc")
	if err != nil {
		t.Fatalf("ResolveAPIKey: %v", err)
	}
	if !auth.HasPermission(id.Permissions, "spec:read") {
		t.Error("expected spec:read permission")
	}
	if auth.HasPermission(id.Permissions, "spec:write") {
		t.Error("unexpected spec:write permission")
	}
}

func TestConfigStore_BuiltinRolePermissions(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_r", Name: "Reader", Role: "reader"},
			{ID: "k2", Key: "spgr_sk_w", Name: "Writer", Role: "writer"},
			{ID: "k3", Key: "spgr_sk_a", Name: "Admin", Role: "admin"},
		},
	}
	store, err := auth.NewConfigStore(cfg, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	reader, _ := store.ResolveAPIKey(context.Background(), "spgr_sk_r")
	if !auth.HasPermission(reader.Permissions, "spec:read") {
		t.Error("reader should have spec:read")
	}
	if auth.HasPermission(reader.Permissions, "spec:write") {
		t.Error("reader should not have spec:write")
	}
	writer, _ := store.ResolveAPIKey(context.Background(), "spgr_sk_w")
	if !auth.HasPermission(writer.Permissions, "spec:write") {
		t.Error("writer should have spec:write")
	}
	if auth.HasPermission(writer.Permissions, "graph:delete") {
		t.Error("writer should not have graph:delete")
	}
	admin, _ := store.ResolveAPIKey(context.Background(), "spgr_sk_a")
	if !auth.HasPermission(admin.Permissions, "graph:delete") {
		t.Error("admin should have graph:delete via *:*")
	}
}

func TestConfigStore_DifferentLengthKeyRejected(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Test Key", Role: "admin"},
		},
	}
	store, err := auth.NewConfigStore(cfg, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	// Shorter key — different length, must not match
	_, err = store.ResolveAPIKey(context.Background(), "spgr_sk_ab")
	if !errors.Is(err, auth.ErrUnknownKey) {
		t.Errorf("shorter key: err = %v, want ErrUnknownKey", err)
	}

	// Longer key — different length, must not match
	_, err = store.ResolveAPIKey(context.Background(), "spgr_sk_abcd")
	if !errors.Is(err, auth.ErrUnknownKey) {
		t.Errorf("longer key: err = %v, want ErrUnknownKey", err)
	}
}

func TestConfigStore_ResolveJWT_ReturnsErrNoOIDC(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	_, err = store.ResolveJWT(context.Background(), "header.payload.signature")
	if !errors.Is(err, auth.ErrNoOIDC) {
		t.Errorf("ResolveJWT error = %v, want ErrNoOIDC", err)
	}
}

func TestConfigStore_HasAuth_NoKeys(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	if store.HasAuth() {
		t.Error("HasAuth() = true, want false with no keys")
	}
}

func TestConfigStore_HasAuth_WithKeys(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Test", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	if !store.HasAuth() {
		t.Error("HasAuth() = false, want true with keys")
	}
}

func TestConfigStore_SameContentKeyMatches(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_exactmatch", Name: "Test Key", Role: "admin"},
		},
	}
	store, err := auth.NewConfigStore(cfg, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	id, err := store.ResolveAPIKey(context.Background(), "spgr_sk_exactmatch")
	if err != nil {
		t.Fatalf("exact match: unexpected error: %v", err)
	}
	if id.Subject != "apikey:k1" {
		t.Errorf("subject = %q, want apikey:k1", id.Subject)
	}
}

func TestConfigStore_LoadsCredentialKeys(t *testing.T) {
	dir := t.TempDir()
	credPath := filepath.Join(dir, "credentials.yaml")
	credYAML := credFileYAML("cred-key", "spgr_sk_from_creds", "Credential Key", "admin")
	if err := os.WriteFile(credPath, []byte(credYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := auth.NewConfigStore(config.AuthConfig{}, credPath)
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	id, err := store.ResolveAPIKey(context.Background(), "spgr_sk_from_creds")
	if err != nil {
		t.Fatalf("ResolveAPIKey: %v", err)
	}
	if id.Subject != "apikey:cred-key" {
		t.Errorf("subject = %q, want apikey:cred-key", id.Subject)
	}
}

func TestConfigStore_ConfigKeyOverridesCredential(t *testing.T) {
	dir := t.TempDir()
	credPath := filepath.Join(dir, "credentials.yaml")
	credYAML := credFileYAML("shared-id", "spgr_sk_cred_version", "Credential Version", "reader")
	if err := os.WriteFile(credPath, []byte(credYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "shared-id", Key: "spgr_sk_config_version", Name: "Config Version", Role: "admin"},
		},
	}, credPath)
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	// Config key should win.
	id, err := store.ResolveAPIKey(context.Background(), "spgr_sk_config_version")
	if err != nil {
		t.Fatalf("ResolveAPIKey for config key: %v", err)
	}
	if id.Role != auth.RoleAdmin {
		t.Errorf("role = %q, want admin", id.Role)
	}

	// Credential key with same ID should NOT be loaded.
	_, err = store.ResolveAPIKey(context.Background(), "spgr_sk_cred_version")
	if !errors.Is(err, auth.ErrUnknownKey) {
		t.Errorf("expected ErrUnknownKey for overridden credential key, got %v", err)
	}
}

func TestConfigStore_MissingCredentialFile(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Test", Role: "admin"},
		},
	}, "/nonexistent/credentials.yaml")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	// Should still work with just config keys.
	id, err := store.ResolveAPIKey(context.Background(), "spgr_sk_test")
	if err != nil {
		t.Fatalf("ResolveAPIKey: %v", err)
	}
	if id.Subject != "apikey:k1" {
		t.Errorf("subject = %q, want apikey:k1", id.Subject)
	}
}

func TestConfigStore_EmptyCredentialFilePath(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test2", Name: "Test", Role: "reader"},
		},
	}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	id, err := store.ResolveAPIKey(context.Background(), "spgr_sk_test2")
	if err != nil {
		t.Fatalf("ResolveAPIKey: %v", err)
	}
	if id.Subject != "apikey:k1" {
		t.Errorf("subject = %q, want apikey:k1", id.Subject)
	}
}

func TestConfigStore_InvalidCredentialYAML(t *testing.T) {
	dir := t.TempDir()
	credPath := filepath.Join(dir, "credentials.yaml")
	if err := os.WriteFile(credPath, []byte("not: [valid yaml"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := auth.NewConfigStore(config.AuthConfig{}, credPath)
	if err == nil {
		t.Fatal("expected error for invalid credential YAML")
	}
}
