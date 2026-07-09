// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
)

// csrfCookieName is the non-HttpOnly double-submit CSRF cookie. It is
// deliberately readable by the web client so it can echo the value back in the
// csrfHeaderName header on mutating requests.
const csrfCookieName = "specgraph_csrf"

// csrfHeaderName is the request header the web client echoes the csrfCookieName
// value into for the double-submit comparison.
const csrfHeaderName = "X-CSRF-Token"

// csrfProtectedProcedures is the set of Connect IdentityService self-key
// procedures guarded by the double-submit CSRF check. These are the
// cookie-authenticated self-mint/list/rotate/revoke mutations reachable from the
// dashboard. ListMyAPIKeys is included for defense-in-depth (cursor #6): it is a
// POST in Connect and, while metadata-only, is cheap to protect.
var csrfProtectedProcedures = map[string]bool{
	"/specgraph.v1.IdentityService/CreateMyAPIKey": true,
	"/specgraph.v1.IdentityService/ListMyAPIKeys":  true,
	"/specgraph.v1.IdentityService/RotateMyAPIKey": true,
	"/specgraph.v1.IdentityService/RevokeMyAPIKey": true,
}

// hasBearerToken reports whether the request carries an Authorization: Bearer
// header (scheme matched case-insensitively per RFC 7235, consistent with
// auth.extractBearerToken). Bearer-authenticated callers (CLI/MCP) are not
// CSRF-able and are exempt from the double-submit check (cursor #2).
func hasBearerToken(r *http.Request) bool {
	scheme, tok, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return false
	}
	return strings.TrimSpace(tok) != ""
}

// csrfValidate is middleware that enforces a double-submit CSRF token on
// cookie-authenticated, mutating (POST) requests to the self-key Connect
// procedures. It ENFORCES only when ALL of the following hold: the method is
// POST, the request is NOT Bearer-authenticated (i.e. cookie-authenticated), and
// the path is one of csrfProtectedProcedures. When enforced, the request must
// carry an X-CSRF-Token header whose value equals the specgraph_csrf cookie,
// compared in constant time; otherwise it is rejected with HTTP 403. All other
// requests pass through untouched.
//
// It is mounted on the Connect IdentityService handler in Plan 05; the cookie it
// validates is issued on the safe /api/auth/whoami GET (see csrfIssue).
func csrfValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !csrfProtectedProcedures[r.URL.Path] || hasBearerToken(r) {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(csrfCookieName)
		header := r.Header.Get(csrfHeaderName)
		if err != nil || cookie.Value == "" || header == "" ||
			subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
			writeJSONError(w, http.StatusForbidden, "invalid or missing CSRF token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// issueCSRFCookie sets the non-HttpOnly specgraph_csrf double-submit cookie when
// it is absent, generating a fresh crypto/rand token. An existing cookie is left
// untouched so a page reload does not rotate a token an in-flight form is about
// to echo. The cookie mirrors sessionCookie's SameSite=Lax + dynamic Secure
// attributes but is intentionally NOT HttpOnly so the web client can read and
// echo it in the X-CSRF-Token header.
func issueCSRFCookie(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(csrfCookieName); err == nil && c.Value != "" {
		return
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failure is catastrophic and non-recoverable; skip issuing
		// rather than set a predictable token. The validator fails closed when
		// the cookie is absent, so no CSRF weakening results.
		return
	}
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: Secure is dynamic via r.TLS / X-Forwarded-Proto for dev/prod parity; non-HttpOnly is required for the double-submit echo // nosemgrep: go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
		Name:     csrfCookieName,
		Value:    hex.EncodeToString(buf),
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	})
}

// csrfIssue is middleware that issues the specgraph_csrf cookie (when absent)
// before delegating to next. Wrap it around a safe GET the dashboard already
// calls on load (whoami) so the double-submit token exists before the first
// mutation (cursor #1 — token bootstrap).
func csrfIssue(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issueCSRFCookie(w, r)
		next.ServeHTTP(w, r)
	})
}
