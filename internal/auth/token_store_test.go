// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/client/transport"
)

func TestFileTokenStore_SaveAndGet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")
	store := NewFileTokenStore(path)

	token := &transport.Token{
		AccessToken:  "access-123",
		TokenType:    "Bearer",
		RefreshToken: "refresh-456",
	}

	ctx := context.Background()
	if err := store.SaveToken(ctx, token); err != nil {
		t.Fatalf("SaveToken error: %v", err)
	}

	got, err := store.GetToken(ctx)
	if err != nil {
		t.Fatalf("GetToken error: %v", err)
	}
	if got.AccessToken != "access-123" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "access-123")
	}
	if got.RefreshToken != "refresh-456" {
		t.Errorf("RefreshToken = %q, want %q", got.RefreshToken, "refresh-456")
	}
}

func TestFileTokenStore_GetToken_NoFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")
	store := NewFileTokenStore(path)

	_, err := store.GetToken(context.Background())
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFileTokenStore_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.json")
	store := NewFileTokenStore(path)

	token := &transport.Token{AccessToken: "secret"}
	if err := store.SaveToken(context.Background(), token); err != nil {
		t.Fatalf("SaveToken error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat error: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}
