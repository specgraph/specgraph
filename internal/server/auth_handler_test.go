// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/storage"
)

// mockResolver implements auth.Resolver for tests.
// "valid-test-key" resolves to a known identity; everything else returns ErrUnauthenticated.
type mockResolver struct{}

func (m *mockResolver) Resolve(_ context.Context, key string) (*auth.Identity, error) {
	if key == "valid-test-key" {
		return &auth.Identity{
			Subject:     "apikey:test",
			DisplayName: "Test User",
			Role:        auth.RoleReader,
		}, nil
	}
	return nil, auth.ErrUnauthenticated
}

func (m *mockResolver) HasAuth(_ context.Context) (bool, error) { return true, nil }

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

func newTestMux(resolver auth.Resolver, authMW func(http.Handler) http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	RegisterAuthHandlers(mux, resolver, nil, authMW)
	return mux
}

func TestHandleLogin_ValidKey(t *testing.T) {
	mux := newTestMux(&mockResolver{}, noopMW)

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
	mux := newTestMux(&mockResolver{}, noopMW)

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
	mux := newTestMux(&mockResolver{}, noopMW)

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
	mux := newTestMux(&mockResolver{}, noopMW)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader("key=valid-test-key"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", w.Code)
	}
}

func TestHandleLogout(t *testing.T) {
	mux := newTestMux(&mockResolver{}, noopMW)

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
	mux := newTestMux(&mockResolver{}, identityMW(id))

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
	mux := newTestMux(&mockResolver{}, noopMW)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/whoami", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

// logoutFakeWA is a minimal storage.WebAuthStore for logout-revocation tests.
// Only RevokeSession has a meaningful body; the rest return zero values.
type logoutFakeWA struct {
	onRevoke     func()
	gotRevoked   []byte // the token hash passed to the most recent RevokeSession
	revokeCalled bool   // true once RevokeSession has been invoked
}

var _ storage.WebAuthStore = (*logoutFakeWA)(nil)

func (f *logoutFakeWA) RevokeSession(_ context.Context, tokenHash []byte) error {
	f.gotRevoked = tokenHash
	f.revokeCalled = true
	if f.onRevoke != nil {
		f.onRevoke()
	}
	return nil
}

// revokedWith reports whether the most recent RevokeSession received hash.
func (f *logoutFakeWA) revokedWith(hash []byte) bool {
	return bytes.Equal(f.gotRevoked, hash)
}

func (f *logoutFakeWA) CreateSession(_ context.Context, _ *storage.Session) (*storage.Session, error) {
	return nil, nil
}

func (f *logoutFakeWA) LookupSessionByHash(_ context.Context, _ []byte) (*storage.Session, error) {
	return nil, nil
}

func (f *logoutFakeWA) DeleteExpiredSessions(_ context.Context) (int64, error) { return 0, nil }

func (f *logoutFakeWA) CreateLoginFlow(_ context.Context, _ *storage.LoginFlow) (string, error) {
	return "", nil
}

func (f *logoutFakeWA) ConsumeLoginFlow(_ context.Context, _ string) (*storage.LoginFlow, error) {
	return nil, nil
}

func (f *logoutFakeWA) DeleteExpiredLoginFlows(_ context.Context) (int64, error) { return 0, nil }
func (f *logoutFakeWA) CreateCLICode(_ context.Context, _ []byte, _, _, _ string, _ time.Time) error {
	return nil
}
func (f *logoutFakeWA) ExchangeCLICode(_ context.Context, _ []byte, _ *storage.Session, _ string) (*storage.Session, error) {
	return nil, nil
}
func (f *logoutFakeWA) DeleteExpiredCLICodes(_ context.Context) (int64, error) { return 0, nil }

func TestLogout_RevokesSession(t *testing.T) {
	revoked := false
	wa := &logoutFakeWA{onRevoke: func() { revoked = true }}
	mux := http.NewServeMux()
	RegisterAuthHandlers(mux, &mockResolver{}, wa, noopMW)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "spgr_ws_abc"}) //nolint:gosec // G124: test request cookie
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rec.Code)
	}
	if !revoked {
		t.Fatal("expected RevokeSession to be called")
	}
	want := sha256.Sum256([]byte("spgr_ws_abc"))
	if !bytes.Equal(wa.gotRevoked, want[:]) {
		t.Fatalf("RevokeSession got wrong hash: %x want %x", wa.gotRevoked, want[:])
	}
}

func TestLogout_NonSessionCookie_NoRevoke(t *testing.T) {
	revoked := false
	wa := &logoutFakeWA{onRevoke: func() { revoked = true }}
	mux := http.NewServeMux()
	RegisterAuthHandlers(mux, &mockResolver{}, wa, noopMW)

	// A legacy API-key cookie value must never be hashed/revoked.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "spgr_sk_legacy"}) //nolint:gosec // G124: test request cookie
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rec.Code)
	}
	if revoked {
		t.Fatal("expected RevokeSession NOT to be called for non-spgr_ws_ cookie")
	}
}

func TestHandleLogout_BearerSession(t *testing.T) {
	t.Parallel()
	wa := &logoutFakeWA{}
	mux := http.NewServeMux()
	RegisterAuthHandlers(mux, &mockResolver{}, wa, noopMW)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer spgr_ws_abc")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	want := sha256.Sum256([]byte("spgr_ws_abc"))
	if !wa.revokedWith(want[:]) {
		t.Fatal("expected RevokeSession for the bearer session token")
	}
}

func TestHandleLogout_BearerAPIKeyIgnored(t *testing.T) {
	t.Parallel()
	wa := &logoutFakeWA{}
	mux := http.NewServeMux()
	RegisterAuthHandlers(mux, &mockResolver{}, wa, noopMW)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer spgr_sk_key")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if wa.revokeCalled {
		t.Fatal("RevokeSession must NOT be called for a non-spgr_ws_ bearer")
	}
}
