// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/auth"
)

// mockStore implements auth.IdentityStore for tests.
// "valid-test-key" resolves to a known identity; everything else returns ErrUnknownKey.
type mockStore struct{}

func (m *mockStore) ResolveAPIKey(_ context.Context, key string) (*auth.Identity, error) {
	if key == "valid-test-key" {
		return &auth.Identity{
			Subject:     "apikey:test",
			DisplayName: "Test User",
			Role:        auth.RoleReader,
		}, nil
	}
	return nil, auth.ErrUnknownKey
}

func (m *mockStore) ResolveJWT(_ context.Context, _ string) (*auth.Identity, error) {
	return nil, auth.ErrNoOIDC
}

func (m *mockStore) HasAuth() bool { return true }

// noopMW is a pass-through auth middleware used for routes that pre-populate the context.
func noopMW(next http.Handler) http.Handler { return next }

// identityMW wraps a handler and injects the given identity into every request context.
func identityMW(id *auth.Identity) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(auth.WithIdentity(r.Context(), id)))
		})
	}
}

func newTestMux(store auth.IdentityStore, authMW func(http.Handler) http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	RegisterAuthHandlers(mux, store, authMW)
	return mux
}

func TestHandleLogin_ValidKey(t *testing.T) {
	mux := newTestMux(&mockStore{}, noopMW)

	body := `{"key":"valid-test-key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var got identityResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Identity.Subject != "apikey:test" {
		t.Errorf("subject = %q, want %q", got.Identity.Subject, "apikey:test")
	}
	if got.Identity.DisplayName != "Test User" {
		t.Errorf("display_name = %q, want %q", got.Identity.DisplayName, "Test User")
	}
	if got.Identity.Role != string(auth.RoleReader) {
		t.Errorf("role = %q, want %q", got.Identity.Role, auth.RoleReader)
	}

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected Set-Cookie header, got none")
	}
	var session *http.Cookie
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			session = c
			break
		}
	}
	if session == nil {
		t.Fatalf("no %q cookie in response", sessionCookieName)
	}
	if session.Value != "valid-test-key" {
		t.Errorf("cookie value = %q, want %q", session.Value, "valid-test-key")
	}
}

func TestHandleLogin_InvalidKey(t *testing.T) {
	mux := newTestMux(&mockStore{}, noopMW)

	body := `{"key":"wrong-key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHandleLogin_MissingBody(t *testing.T) {
	mux := newTestMux(&mockStore{}, noopMW)

	// Empty JSON body — key field absent → empty string → treated as missing.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleLogin_WrongContentType(t *testing.T) {
	mux := newTestMux(&mockStore{}, noopMW)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("key=valid-test-key"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", w.Code)
	}
}

func TestHandleLogout(t *testing.T) {
	mux := newTestMux(&mockStore{}, noopMW)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	var session *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookieName {
			session = c
			break
		}
	}
	if session == nil {
		t.Fatalf("no %q cookie in logout response", sessionCookieName)
	}
	if session.MaxAge >= 0 {
		t.Errorf("expected MaxAge < 0 (clear cookie), got %d", session.MaxAge)
	}
}

func TestHandleWhoami_WithIdentity(t *testing.T) {
	id := &auth.Identity{
		Subject:     "apikey:test",
		DisplayName: "Test User",
		Role:        auth.RoleReader,
	}
	mux := newTestMux(&mockStore{}, identityMW(id))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/whoami", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var got identityResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Identity.Subject != id.Subject {
		t.Errorf("subject = %q, want %q", got.Identity.Subject, id.Subject)
	}
	if got.Identity.DisplayName != id.DisplayName {
		t.Errorf("display_name = %q, want %q", got.Identity.DisplayName, id.DisplayName)
	}
}

func TestHandleWhoami_NoIdentity(t *testing.T) {
	mux := newTestMux(&mockStore{}, noopMW)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/whoami", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
