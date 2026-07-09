// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/storage"
)

const sessionCookieName = "specgraph_session"

// RegisterAuthHandlers registers login, logout, and whoami endpoints.
// authMW is applied to protected routes (whoami). It must not be nil.
// resolver validates credentials at login. webAuth (may be nil) is used to
// revoke the server session on logout.
func RegisterAuthHandlers(mux *http.ServeMux, resolver auth.Resolver, webAuth storage.WebAuthStore, authMW func(http.Handler) http.Handler) {
	if authMW == nil {
		panic("RegisterAuthHandlers: authMW must not be nil")
	}

	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		handleLogin(w, r, resolver)
	})

	mux.HandleFunc("/api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		handleLogout(w, r, webAuth)
	})

	// whoami: GET only, translate session cookie → Authorization header, then apply authMW.
	// csrfIssue wraps the GET so a dashboard load bootstraps the specgraph_csrf
	// double-submit cookie before the first self-key mutation (cursor #1).
	mux.Handle("/api/auth/whoami", csrfIssue(authMW(cookieToAuthHeader(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		handleWhoami(w, r)
	})))))
}

// handleLogin validates the API key and sets a session cookie on success.
func handleLogin(w http.ResponseWriter, r *http.Request, resolver auth.Resolver) {
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		writeJSONError(w, http.StatusUnsupportedMediaType, "unsupported media type")
		return
	}

	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" {
		writeJSONError(w, http.StatusBadRequest, "missing key")
		return
	}

	id, err := resolver.Resolve(r.Context(), req.Key)
	if err != nil {
		if errors.Is(err, auth.ErrUnauthenticated) {
			writeJSONError(w, http.StatusUnauthorized, "invalid API key")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}

	// Store the raw API key in the cookie so the auth middleware can resolve it.
	http.SetCookie(w, sessionCookie(req.Key, r))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newIdentityResponse(id)) //nolint:errcheck // best-effort write to http.ResponseWriter
}

// handleLogout revokes the server session (if the cookie holds a spgr_ws_
// token) and clears the session cookie. A legacy API-key cookie value is
// never hashed/looked-up.
func handleLogout(w http.ResponseWriter, r *http.Request, webAuth storage.WebAuthStore) {
	if webAuth != nil {
		if tok := bearerSessionToken(r); tok != "" {
			sum := sha256.Sum256([]byte(tok))
			if revErr := webAuth.RevokeSession(r.Context(), sum[:]); revErr != nil {
				slog.LogAttrs(r.Context(), slog.LevelWarn, "logout: revoke session", slog.Any("error", revErr))
			}
		}
	}
	c := sessionCookie("", r) //nolint:gosec // G124: sessionCookie sets HttpOnly/SameSite/dynamic Secure
	c.MaxAge = -1
	http.SetCookie(w, c)
	w.WriteHeader(http.StatusNoContent)
}

// bearerSessionToken returns a spgr_ws_ session token from the cookie or an
// Authorization: Bearer header. The auth scheme is matched case-insensitively
// (RFC 7235), consistent with auth.extractBearerToken. Non-session values
// (e.g. API keys) yield "".
func bearerSessionToken(r *http.Request) string {
	if c, err := r.Cookie(sessionCookieName); err == nil && strings.HasPrefix(c.Value, "spgr_ws_") {
		return c.Value
	}
	scheme, tok, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if ok && strings.EqualFold(scheme, "Bearer") {
		if tok = strings.TrimSpace(tok); strings.HasPrefix(tok, "spgr_ws_") {
			return tok
		}
	}
	return ""
}

// handleWhoami returns the identity from the request context (set by authMW).
func handleWhoami(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.IdentityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newIdentityResponse(id)) //nolint:errcheck // best-effort write to http.ResponseWriter
}

// cookieToAuthHeader is middleware that promotes a session cookie to an
// Authorization: Bearer header so the downstream authMW can resolve it.
// It does not overwrite an existing Authorization header.
func cookieToAuthHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
				// Clone the request so we don't mutate the original.
				r2 := r.Clone(r.Context())
				r2.Header.Set("Authorization", "Bearer "+c.Value)
				r = r2
			}
		}
		next.ServeHTTP(w, r)
	})
}

// writeJSONError writes a JSON error response with the given HTTP status code.
func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck // best-effort write to http.ResponseWriter
}

// establishSession sets the session cookie to the given token value. It is the
// single seam that seats a web session (OIDC flow uses it). MUST reuse
// sessionCookie()'s exact name and Path so a callback's Set-Cookie
// deterministically overwrites any pre-existing session cookie and logout's
// clear always deletes it.
func establishSession(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, sessionCookie(token, r))
}

// sessionCookie returns a configured session cookie with the given value.
// Secure is set dynamically: true when the request arrived over TLS or via a
// reverse proxy signaling HTTPS via X-Forwarded-Proto, false otherwise (local HTTP dev).
// SameSite=Lax so the cookie is sent on the post-IdP redirect top-level
// navigation; safe because no GET endpoint mutates state. This relaxation is
// intentionally global: it applies to the legacy API-key session cookie too,
// which is acceptable under the same "no GET mutates state" invariant.
func sessionCookie(value string, r *http.Request) *http.Cookie {
	return &http.Cookie{ //nolint:gosec // G124: Secure is dynamic via r.TLS / X-Forwarded-Proto for dev/prod parity; HttpOnly+SameSite are set below // nosemgrep: go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	}
}

// identityResponse builds the JSON-serializable identity map.
type identityResponse struct {
	Identity struct {
		Subject     string `json:"subject"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
	} `json:"identity"`
}

func newIdentityResponse(id *auth.Identity) identityResponse {
	var resp identityResponse
	resp.Identity.Subject = id.Subject
	resp.Identity.DisplayName = id.DisplayName
	resp.Identity.Role = string(id.Role)
	return resp
}
