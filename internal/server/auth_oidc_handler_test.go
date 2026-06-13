// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/storage"
)

type fakeProvider struct {
	id          string
	exchangeErr error
	idToken     string
}

func (f *fakeProvider) ID() string          { return f.id }
func (f *fakeProvider) DisplayName() string { return "Fake" }
func (f *fakeProvider) AuthCodeURL(state, _, _, _ string) string {
	return "https://idp/authorize?state=" + state
}
func (f *fakeProvider) Exchange(_ context.Context, _, _, _, _ string) (string, error) {
	return f.idToken, f.exchangeErr
}

type fakeWA struct {
	flows      map[string]*storage.LoginFlow
	sessions   map[string]*storage.Session
	createErr  error
	consumeErr error // when set, ConsumeLoginFlow returns it (for the existing flow id)
}

func (f *fakeWA) CreateSession(_ context.Context, s *storage.Session) (*storage.Session, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.sessions == nil {
		f.sessions = map[string]*storage.Session{}
	}
	s.ID = "s1"
	f.sessions[string(s.TokenHash)] = s
	return s, nil
}
func (f *fakeWA) LookupSessionByHash(context.Context, []byte) (*storage.Session, error) {
	return nil, storage.ErrSessionNotFound
}
func (f *fakeWA) RevokeSession(context.Context, []byte) error          { return nil }
func (f *fakeWA) DeleteExpiredSessions(context.Context) (int64, error) { return 0, nil }
func (f *fakeWA) CreateLoginFlow(_ context.Context, fl *storage.LoginFlow) (string, error) {
	if f.flows == nil {
		f.flows = map[string]*storage.LoginFlow{}
	}
	fl.ID = "flow-1"
	f.flows[fl.ID] = fl
	return fl.ID, nil
}
func (f *fakeWA) ConsumeLoginFlow(_ context.Context, id string) (*storage.LoginFlow, error) {
	if f.consumeErr != nil {
		return nil, f.consumeErr
	}
	fl, ok := f.flows[id]
	if !ok {
		return nil, storage.ErrLoginFlowNotFound
	}
	delete(f.flows, id)
	return fl, nil
}
func (f *fakeWA) DeleteExpiredLoginFlows(context.Context) (int64, error) { return 0, nil }

type fakeResolver struct {
	id  *auth.Identity
	err error
}

func (f *fakeResolver) Resolve(context.Context, string) (*auth.Identity, error) {
	return f.id, f.err
}
func (f *fakeResolver) HasAuth(context.Context) (bool, error) { return true, nil }

func newTestOIDCMux(provs []auth.LoginProvider, wa storage.WebAuthStore, res auth.Resolver) *http.ServeMux {
	mux := http.NewServeMux()
	RegisterOIDCLoginHandlers(mux, OIDCLoginConfig{
		Providers: provs, Resolver: res, WebAuth: wa,
		SessionTTL: time.Hour, FlowTTL: time.Minute,
		Limiter: newIPRateLimiter(1000, 1000, false),
	})
	return mux
}

func TestOIDCStart_RedirectsAndSetsCookie(t *testing.T) {
	wa := &fakeWA{}
	mux := newTestOIDCMux([]auth.LoginProvider{&fakeProvider{id: "entra"}}, wa, &fakeResolver{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/entra/start", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "https://idp/authorize") {
		t.Fatalf("Location = %q, want prefix https://idp/authorize", loc)
	}
	if len(wa.flows) != 1 {
		t.Fatalf("flows created = %d, want 1", len(wa.flows))
	}
	var found bool
	for _, sc := range rec.Header().Values("Set-Cookie") {
		if strings.Contains(sc, txCookieName) {
			found = true
		}
	}
	if !found {
		t.Fatalf("Set-Cookie missing %q: %v", txCookieName, rec.Header().Values("Set-Cookie"))
	}
}

func TestOIDCStart_UnknownProvider404(t *testing.T) {
	mux := newTestOIDCMux([]auth.LoginProvider{&fakeProvider{id: "entra"}}, &fakeWA{}, &fakeResolver{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/nope/start", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestOIDCCallback_HappyPath(t *testing.T) {
	wa := &fakeWA{
		flows: map[string]*storage.LoginFlow{
			"flow-1": {ID: "flow-1", State: "S", ProviderID: "entra", Nonce: "n", CodeVerifier: "v"},
		},
	}
	res := &fakeResolver{id: &auth.Identity{UserID: "u1", Subject: "oidc:sub-1"}}
	mux := newTestOIDCMux([]auth.LoginProvider{&fakeProvider{id: "entra", idToken: "tok"}}, wa, res)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state=S&code=abc", nil)
	req.AddCookie(&http.Cookie{Name: txCookieName, Value: "flow-1"}) //nolint:gosec // G124: test request cookie
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Fatalf("Location = %q, want /", loc)
	}
	if len(wa.sessions) != 1 {
		t.Fatalf("sessions created = %d, want 1", len(wa.sessions))
	}
	var sessionVal string
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == sessionCookieName {
			sessionVal = ck.Value
		}
	}
	if !strings.HasPrefix(sessionVal, "spgr_ws_") {
		t.Fatalf("session cookie value = %q, want prefix spgr_ws_", sessionVal)
	}
	// Verify subject prefix stripped for storage.
	for _, s := range wa.sessions {
		if s.OIDCSubject != "sub-1" {
			t.Fatalf("OIDCSubject = %q, want sub-1", s.OIDCSubject)
		}
	}
}

func TestOIDCCallback_Failures(t *testing.T) {
	tests := []struct {
		name      string
		withTx    bool
		txValue   string
		query     string
		wantError string
	}{
		{name: "missing-tx", withTx: false, query: "state=S&code=abc", wantError: "expired"},
		{name: "unknown-flow", withTx: true, txValue: "no-such", query: "state=S&code=abc", wantError: "expired"},
		{name: "idp-error", withTx: true, txValue: "flow-1", query: "error=access_denied", wantError: "denied"},
		{name: "state-mismatch", withTx: true, txValue: "flow-1", query: "state=WRONG&code=abc", wantError: "state"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wa := &fakeWA{
				flows: map[string]*storage.LoginFlow{
					"flow-1": {ID: "flow-1", State: "S", ProviderID: "entra", Nonce: "n", CodeVerifier: "v"},
				},
			}
			res := &fakeResolver{id: &auth.Identity{UserID: "u1", Subject: "oidc:sub-1"}}
			mux := newTestOIDCMux([]auth.LoginProvider{&fakeProvider{id: "entra", idToken: "tok"}}, wa, res)

			req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?"+tc.query, nil)
			if tc.withTx {
				req.AddCookie(&http.Cookie{Name: txCookieName, Value: tc.txValue}) //nolint:gosec // G124: test request cookie
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusFound {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
			}
			loc := rec.Header().Get("Location")
			if !strings.Contains(loc, "auth_error="+tc.wantError) {
				t.Fatalf("Location = %q, want auth_error=%s", loc, tc.wantError)
			}
		})
	}
}

func TestOIDCCallback_ConsumeTransient(t *testing.T) {
	wa := &fakeWA{
		flows: map[string]*storage.LoginFlow{
			"flow-1": {ID: "flow-1", State: "S", ProviderID: "entra"},
		},
		consumeErr: errors.New("db down"),
	}
	res := &fakeResolver{id: &auth.Identity{UserID: "u1", Subject: "oidc:sub-1"}}
	mux := newTestOIDCMux([]auth.LoginProvider{&fakeProvider{id: "entra", idToken: "tok"}}, wa, res)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state=S&code=abc", nil)
	req.AddCookie(&http.Cookie{Name: txCookieName, Value: "flow-1"}) //nolint:gosec // G124: test request cookie
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "auth_error=temporary") {
		t.Fatalf("Location = %q, want auth_error=temporary", loc)
	}
}

func TestOIDCCallback_Unauthorized(t *testing.T) {
	wa := &fakeWA{
		flows: map[string]*storage.LoginFlow{
			"flow-1": {ID: "flow-1", State: "S", ProviderID: "entra", Nonce: "n", CodeVerifier: "v"},
		},
	}
	res := &fakeResolver{err: auth.ErrUnauthenticated}
	mux := newTestOIDCMux([]auth.LoginProvider{&fakeProvider{id: "entra", idToken: "tok"}}, wa, res)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state=S&code=abc", nil)
	req.AddCookie(&http.Cookie{Name: txCookieName, Value: "flow-1"}) //nolint:gosec // G124: test request cookie
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "auth_error=unauthorized") {
		t.Fatalf("Location = %q, want auth_error=unauthorized", loc)
	}
	if len(wa.sessions) != 0 {
		t.Fatalf("sessions created = %d, want 0", len(wa.sessions))
	}
}

func TestOIDCCallback_DeletesTxCookie(t *testing.T) {
	wa := &fakeWA{}
	mux := newTestOIDCMux([]auth.LoginProvider{&fakeProvider{id: "entra"}}, wa, &fakeResolver{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state=S", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var found bool
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == txCookieName && ck.MaxAge < 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %q cookie with MaxAge<0 (deletion): %v", txCookieName, rec.Result().Cookies())
	}
}

// callbackWithFlow drives a callback against a freshly-seeded flow-1 and
// returns the auth_error reason from the redirect Location (empty for success).
func callbackWithFlow(t *testing.T, prov auth.LoginProvider, res auth.Resolver, wa *fakeWA) *httptest.ResponseRecorder {
	t.Helper()
	if wa.flows == nil {
		wa.flows = map[string]*storage.LoginFlow{
			"flow-1": {ID: "flow-1", State: "S", ProviderID: "entra", Nonce: "n", CodeVerifier: "v"},
		}
	}
	mux := newTestOIDCMux([]auth.LoginProvider{prov}, wa, res)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?state=S&code=abc", nil)
	req.AddCookie(&http.Cookie{Name: txCookieName, Value: "flow-1"}) //nolint:gosec // G124: test request cookie
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestOIDCCallback_ResolverTransient(t *testing.T) {
	wa := &fakeWA{}
	res := &fakeResolver{err: auth.ErrTransient}
	rec := callbackWithFlow(t, &fakeProvider{id: "entra", idToken: "tok"}, res, wa)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "auth_error=temporary") {
		t.Fatalf("Location = %q, want auth_error=temporary", loc)
	}
	if len(wa.sessions) != 0 {
		t.Fatalf("no session must be minted on transient resolve, got %d", len(wa.sessions))
	}
}

func TestOIDCCallback_ExchangeFailure(t *testing.T) {
	wa := &fakeWA{}
	prov := &fakeProvider{id: "entra", exchangeErr: errors.New("token endpoint 500")}
	rec := callbackWithFlow(t, prov, &fakeResolver{}, wa)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "auth_error=exchange") {
		t.Fatalf("Location = %q, want auth_error=exchange", loc)
	}
	if len(wa.sessions) != 0 {
		t.Fatalf("no session must be minted on exchange failure, got %d", len(wa.sessions))
	}
}

func TestOIDCCallback_ProviderRemovedMidFlow(t *testing.T) {
	// The flow references "ghost", but only "entra" is registered.
	wa := &fakeWA{
		flows: map[string]*storage.LoginFlow{
			"flow-1": {ID: "flow-1", State: "S", ProviderID: "ghost", Nonce: "n", CodeVerifier: "v"},
		},
	}
	rec := callbackWithFlow(t, &fakeProvider{id: "entra", idToken: "tok"}, &fakeResolver{}, wa)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "auth_error=exchange") {
		t.Fatalf("Location = %q, want auth_error=exchange", loc)
	}
}
