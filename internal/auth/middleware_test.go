// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/specgraph/specgraph/internal/auth"
)

func TestRequireAuth_NoToken_Returns401(t *testing.T) {
	resolver := &fakeResolver{
		resolve: func(_ context.Context, _ string) (*auth.Identity, error) {
			return nil, auth.ErrUnauthenticated
		},
	}
	handler := auth.RequireAuth(resolver)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestRequireAuth_ValidBearerToken_PassesThrough(t *testing.T) {
	id := &auth.Identity{Subject: "apikey:k1", Role: auth.RoleAdmin, EffectiveRole: auth.RoleAdmin}
	resolver := &fakeResolver{
		resolve: func(_ context.Context, token string) (*auth.Identity, error) {
			if token == "spgr_sk_validtoken" { //nolint:gosec // G101: test fixture token; not a real credential
				return id, nil
			}
			return nil, auth.ErrUnauthenticated
		},
	}
	called := false
	handler := auth.RequireAuth(resolver)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer spgr_sk_validtoken")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !called {
		t.Fatal("handler not called with valid bearer token")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRequireAuth_ErrTransient_Returns503(t *testing.T) {
	resolver := &fakeResolver{
		resolve: func(_ context.Context, _ string) (*auth.Identity, error) {
			return nil, fmt.Errorf("%w: db down", auth.ErrTransient)
		},
	}
	handler := auth.RequireAuth(resolver)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer spgr_sk_sometoken")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
}

func TestRequireAuth_SessionCookie_ValidToken_PassesThrough(t *testing.T) {
	id := &auth.Identity{Subject: "apikey:k1", Role: auth.RoleAdmin, EffectiveRole: auth.RoleAdmin}
	resolver := &fakeResolver{
		resolve: func(_ context.Context, token string) (*auth.Identity, error) {
			if token == "spgr_sk_cookievalue" { //nolint:gosec // G101: test fixture token; not a real credential
				return id, nil
			}
			return nil, auth.ErrUnauthenticated
		},
	}
	called := false
	handler := auth.RequireAuth(resolver)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.AddCookie(&http.Cookie{Name: "specgraph_session", Value: "spgr_sk_cookievalue", HttpOnly: true, SameSite: http.SameSiteStrictMode}) //nolint:gosec // G124: test cookie; Secure omitted for plain-HTTP httptest server
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if !called {
		t.Fatal("handler not called with valid session cookie")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRequireAuth_InvalidToken_Returns401(t *testing.T) {
	resolver := &fakeResolver{
		resolve: func(_ context.Context, _ string) (*auth.Identity, error) {
			return nil, auth.ErrUnauthenticated
		},
	}
	handler := auth.RequireAuth(resolver)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer bad_token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestRequireAuth_ContextCanceled_NoAuthError(t *testing.T) {
	resolver := &fakeResolver{
		resolve: func(_ context.Context, _ string) (*auth.Identity, error) {
			return nil, context.Canceled
		},
	}
	called := false
	handler := auth.RequireAuth(resolver)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer spgr_sk_sometoken")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Fatal("next handler must not be invoked when the request is canceled")
	}
	// A canceled/timed-out request has no meaningful response: the middleware
	// returns without writing, so it must NOT report a 401 auth failure.
	if rec.Code == http.StatusUnauthorized {
		t.Errorf("status = %d; canceled request must not be reported as 401", rec.Code)
	}
}
