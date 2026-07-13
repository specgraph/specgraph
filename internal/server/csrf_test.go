// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	csrfTestProcedure = "/specgraph.v1.IdentityService/CreateMyAPIKey"
	csrfTestToken     = "deadbeefcafef00ddeadbeefcafef00ddeadbeefcafef00ddeadbeefcafef00d"
)

// okHandler records that the wrapped handler was reached and returns 200.
func okHandler(reached *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*reached = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestCSRFValidate_MatchingTokenPasses(t *testing.T) {
	reached := false
	req := httptest.NewRequest(http.MethodPost, csrfTestProcedure, nil)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfTestToken}) //nolint:gosec // G124: test request cookie
	req.Header.Set(csrfHeaderName, csrfTestToken)

	w := httptest.NewRecorder()
	csrfValidate(okHandler(&reached)).ServeHTTP(w, req)

	if !reached {
		t.Fatalf("expected downstream handler reached on matching token")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on matching token, got %d", w.Code)
	}
}

func TestCSRFValidate_MissingHeaderRejected(t *testing.T) {
	reached := false
	req := httptest.NewRequest(http.MethodPost, csrfTestProcedure, nil)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfTestToken}) //nolint:gosec // G124: test request cookie
	// No X-CSRF-Token header.

	w := httptest.NewRecorder()
	csrfValidate(okHandler(&reached)).ServeHTTP(w, req)

	if reached {
		t.Fatalf("handler must NOT be reached when the CSRF header is missing")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on missing header, got %d", w.Code)
	}
}

func TestCSRFValidate_MismatchedTokenRejected(t *testing.T) {
	reached := false
	req := httptest.NewRequest(http.MethodPost, csrfTestProcedure, nil)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfTestToken}) //nolint:gosec // G124: test request cookie
	req.Header.Set(csrfHeaderName, "0000000000000000000000000000000000000000000000000000000000000000")

	w := httptest.NewRecorder()
	csrfValidate(okHandler(&reached)).ServeHTTP(w, req)

	if reached {
		t.Fatalf("handler must NOT be reached when the token mismatches")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on mismatched token, got %d", w.Code)
	}
}

func TestCSRFValidate_BearerRequestExempt(t *testing.T) {
	reached := false
	// Bearer-authenticated (CLI/MCP) request to the same self-key procedure:
	// NOT CSRF-able, so it must pass without any CSRF cookie/header.
	req := httptest.NewRequest(http.MethodPost, csrfTestProcedure, nil)
	req.Header.Set("Authorization", "Bearer spgr_some_api_key")

	w := httptest.NewRecorder()
	csrfValidate(okHandler(&reached)).ServeHTTP(w, req)

	if !reached {
		t.Fatalf("Bearer-authenticated request must be exempt and reach the handler")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for exempt Bearer request, got %d", w.Code)
	}
}

func TestCSRFValidate_UnprotectedPathPasses(t *testing.T) {
	reached := false
	// A cookie-authed POST to a non-self-key path is not enforced.
	req := httptest.NewRequest(http.MethodPost, "/specgraph.v1.IdentityService/CreateAPIKey", nil)

	w := httptest.NewRecorder()
	csrfValidate(okHandler(&reached)).ServeHTTP(w, req)

	if !reached {
		t.Fatalf("non-protected procedure must pass through untouched")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on unprotected path, got %d", w.Code)
	}
}

func TestCSRFIssue_SetsNonHTTPOnlyCookieWhenAbsent(t *testing.T) {
	reached := false
	req := httptest.NewRequest(http.MethodGet, "/api/auth/whoami", nil)
	w := httptest.NewRecorder()

	csrfIssue(okHandler(&reached)).ServeHTTP(w, req)

	if !reached {
		t.Fatalf("csrfIssue must delegate to the next handler")
	}
	var got *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == csrfCookieName {
			got = c
		}
	}
	if got == nil {
		t.Fatalf("expected a %s cookie to be issued when absent", csrfCookieName)
	}
	if got.Value == "" {
		t.Fatalf("issued CSRF cookie must carry a non-empty token")
	}
	if got.HttpOnly {
		t.Fatalf("CSRF cookie must be non-HttpOnly so the web client can echo it")
	}
	if got.SameSite != http.SameSiteLaxMode {
		t.Fatalf("CSRF cookie must be SameSite=Lax, got %v", got.SameSite)
	}
}

func TestCSRFIssue_DoesNotOverwriteExistingCookie(t *testing.T) {
	reached := false
	req := httptest.NewRequest(http.MethodGet, "/api/auth/whoami", nil)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfTestToken}) //nolint:gosec // G124: test request cookie
	w := httptest.NewRecorder()

	csrfIssue(okHandler(&reached)).ServeHTTP(w, req)

	for _, c := range w.Result().Cookies() {
		if c.Name == csrfCookieName {
			t.Fatalf("csrfIssue must NOT re-issue an existing CSRF cookie")
		}
	}
}
