// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
)

func TestConfigStore_ResolveAPIKey(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Test Key", Role: "admin"},
		},
	}
	store, err := auth.NewConfigStore(cfg)
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
	store, err := auth.NewConfigStore(cfg)
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	_, err = store.ResolveAPIKey(context.Background(), "wrong_key")
	if !errors.Is(err, auth.ErrUnknownKey) {
		t.Errorf("err = %v, want ErrUnknownKey", err)
	}
}

func TestConfigStore_HasKeys(t *testing.T) {
	empty, err := auth.NewConfigStore(config.AuthConfig{})
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
	})
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
	_, err := auth.NewConfigStore(cfg)
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
	_, err := auth.NewConfigStore(cfg)
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
	_, err := auth.NewConfigStore(cfg)
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
	_, err := auth.NewConfigStore(cfg)
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
	_, err := auth.NewConfigStore(cfg)
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
	store, err := auth.NewConfigStore(cfg)
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
	store, err := auth.NewConfigStore(cfg)
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
