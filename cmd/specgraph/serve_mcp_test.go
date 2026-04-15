// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 SpecGraph Contributors

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
)

// newMCPAuthStore returns a ConfigStore pre-loaded with a single admin API key
// for use in MCP endpoint auth tests.
func newMCPAuthStore(t *testing.T) *auth.ConfigStore {
	t.Helper()
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "test-admin", Key: "spgr_sk_valid_mcp_key", Name: "Test Admin", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	return store
}

// mcpHandler wraps a simple OK handler in RequireAuth to simulate the /mcp/ endpoint.
func mcpHandler(store auth.IdentityStore) http.Handler {
	return auth.RequireAuth(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
}

func TestMCPEndpoint_Unauthenticated(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "test-admin", Key: "spgr_sk_valid_mcp_key", Name: "Test Admin", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	handler := mcpHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/mcp/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMCPEndpoint_ValidAPIKey(t *testing.T) {
	store := newMCPAuthStore(t)

	var gotIdentity *auth.Identity
	handler := auth.RequireAuth(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := auth.IdentityFromContext(r.Context())
		if !ok {
			t.Error("no identity in context after successful auth")
		}
		gotIdentity = id
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp/", nil)
	req.Header.Set("Authorization", "Bearer spgr_sk_valid_mcp_key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotIdentity == nil {
		t.Fatal("identity is nil after successful auth")
	}
	if gotIdentity.Subject != "apikey:test-admin" {
		t.Errorf("identity.Subject = %q, want %q", gotIdentity.Subject, "apikey:test-admin")
	}
	if gotIdentity.Role != auth.RoleAdmin {
		t.Errorf("identity.Role = %q, want %q", gotIdentity.Role, auth.RoleAdmin)
	}
}

func TestMCPEndpoint_InvalidToken(t *testing.T) {
	store := newMCPAuthStore(t)
	handler := mcpHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/mcp/", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
