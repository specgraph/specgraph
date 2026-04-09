// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/specgraph/specgraph/internal/auth"
)

const sessionCookieName = "specgraph_session"

// RegisterAuthHandlers registers login, logout, and whoami endpoints.
// authMW is applied to protected routes (whoami). It must not be nil.
func RegisterAuthHandlers(mux *http.ServeMux, store auth.IdentityStore, authMW func(http.Handler) http.Handler) {
	if authMW == nil {
		panic("RegisterAuthHandlers: authMW must not be nil")
	}

	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		handleLogin(w, r, store)
	})

	mux.HandleFunc("/api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		handleLogout(w, r)
	})

	// whoami: GET only, translate session cookie → Authorization header, then apply authMW.
	mux.Handle("/api/auth/whoami", authMW(cookieToAuthHeader(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		handleWhoami(w, r)
	}))))
}

// handleLogin validates the API key and sets a session cookie on success.
func handleLogin(w http.ResponseWriter, r *http.Request, store auth.IdentityStore) {
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

	id, err := store.ResolveAPIKey(r.Context(), req.Key)
	if err != nil {
		if errors.Is(err, auth.ErrUnknownKey) {
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

// handleLogout clears the session cookie.
func handleLogout(w http.ResponseWriter, r *http.Request) {
	c := sessionCookie("", r)
	c.MaxAge = -1
	http.SetCookie(w, c)
	w.WriteHeader(http.StatusNoContent)
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

// sessionCookie returns a configured session cookie with the given value.
// Secure is set dynamically: true when the request arrived over TLS or via a
// reverse proxy signaling HTTPS via X-Forwarded-Proto, false otherwise (local HTTP dev).
func sessionCookie(value string, r *http.Request) *http.Cookie {
	return &http.Cookie{ // nosemgrep: go.lang.security.audit.net.cookie-missing-secure.cookie-missing-secure
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
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
