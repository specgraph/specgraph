// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 SpecGraph Contributors

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/specgraph/specgraph/internal/auth"
)

// stubResolver is a simple Resolver for MCP endpoint auth tests.
// It accepts exactly one valid token (validToken) and rejects everything else.
type stubResolver struct {
	validToken string
	identity   *auth.Identity
}

func (r *stubResolver) Resolve(_ context.Context, token string) (*auth.Identity, error) {
	if token == r.validToken {
		return r.identity, nil
	}
	return nil, auth.ErrUnauthenticated
}

func (r *stubResolver) HasAuth(_ context.Context) (bool, error) { return true, nil }

// newMCPAuthResolver returns a Resolver pre-loaded with a single admin identity.
func newMCPAuthResolver() *stubResolver {
	return &stubResolver{ //nolint:gosec // G101: test fixture struct; validToken is a test-only placeholder, not a real credential
		validToken: "spgr_sk_valid_mcp_key",
		identity: &auth.Identity{
			Subject:       "apikey:test-admin",
			DisplayName:   "Test Admin",
			Role:          auth.RoleAdmin,
			EffectiveRole: auth.RoleAdmin,
			Source:        "apikey",
		},
	}
}

func TestMCPEndpoint_Unauthenticated(t *testing.T) {
	resolver := newMCPAuthResolver()
	handler := auth.RequireAuth(resolver)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMCPEndpoint_ValidAPIKey(t *testing.T) {
	resolver := newMCPAuthResolver()

	var gotIdentity *auth.Identity
	handler := auth.RequireAuth(resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	resolver := newMCPAuthResolver()
	handler := auth.RequireAuth(resolver)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp/", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
