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
	"github.com/specgraph/specgraph/internal/config"
)

func TestRequireAuth_NoKeys_NoToken_Returns401(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatal(err)
	}

	handler := auth.RequireAuth(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestRequireAuth_WithKeys_NoToken_Returns401(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Admin", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatal(err)
	}

	handler := auth.RequireAuth(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestRequireAuth_WithKeys_ValidToken_PassesThrough(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Admin", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatal(err)
	}

	called := false
	handler := auth.RequireAuth(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer spgr_sk_test")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("handler not called with valid token")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRequireAuth_WithKeys_InvalidToken_Returns401(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Admin", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatal(err)
	}

	handler := auth.RequireAuth(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer wrong_key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestRequireAuth_MalformedAuthHeader_Returns401(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Admin", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatal(err)
	}

	handler := auth.RequireAuth(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestRequireAuth_SessionCookie_ValidKey_Returns200(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Admin", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatal(err)
	}

	called := false
	handler := auth.RequireAuth(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.AddCookie(&http.Cookie{Name: "specgraph_session", Value: "spgr_sk_test", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("handler not called with valid session cookie")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRequireAuth_HeaderTakesPrecedenceOverCookie(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_valid", Name: "Admin", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatal(err)
	}

	called := false
	handler := auth.RequireAuth(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// Valid header + invalid cookie: header wins → success.
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer spgr_sk_valid")
	req.AddCookie(&http.Cookie{Name: "specgraph_session", Value: "invalid_cookie_token", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("handler not called when valid header present with invalid cookie")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// --- V2 middleware tests (RequireAuthV2) ---

func TestRequireAuthV2_NoToken_Returns401(t *testing.T) {
	resolver := &fakeResolver{
		resolve: func(_ context.Context, _ string) (*auth.Identity, error) {
			return nil, auth.ErrUnauthenticated
		},
	}
	handler := auth.RequireAuthV2(resolver)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestRequireAuthV2_ValidBearerToken_PassesThrough(t *testing.T) {
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
	handler := auth.RequireAuthV2(resolver)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

func TestRequireAuthV2_ErrTransient_Returns503(t *testing.T) {
	resolver := &fakeResolver{
		resolve: func(_ context.Context, _ string) (*auth.Identity, error) {
			return nil, fmt.Errorf("%w: db down", auth.ErrTransient)
		},
	}
	handler := auth.RequireAuthV2(resolver)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

func TestRequireAuthV2_SessionCookie_ValidToken_PassesThrough(t *testing.T) {
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
	handler := auth.RequireAuthV2(resolver)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

func TestRequireAuthV2_InvalidToken_Returns401(t *testing.T) {
	resolver := &fakeResolver{
		resolve: func(_ context.Context, _ string) (*auth.Identity, error) {
			return nil, auth.ErrUnauthenticated
		},
	}
	handler := auth.RequireAuthV2(resolver)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

func TestRequireAuthV2_ContextCanceled_NoAuthError(t *testing.T) {
	resolver := &fakeResolver{
		resolve: func(_ context.Context, _ string) (*auth.Identity, error) {
			return nil, context.Canceled
		},
	}
	called := false
	handler := auth.RequireAuthV2(resolver)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
