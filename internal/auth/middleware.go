// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"net/http"
	"strings"
)

// RequireAuth returns HTTP middleware that enforces API key authentication.
// Requests must provide a valid Bearer token in the Authorization header or
// a valid session token in the specgraph_session cookie.
func RequireAuth(store IdentityStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := authenticate(r.Context(), store, r)
			if !ok {
				http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}

// authenticate resolves the caller identity from the request's Authorization header
// or specgraph_session cookie. Returns the identity and true on success, or nil and
// false on failure.
func authenticate(ctx context.Context, store IdentityStore, r *http.Request) (*Identity, bool) {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		scheme, token, ok := strings.Cut(authHeader, " ")
		token = strings.TrimSpace(token)
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
			return nil, false
		}
		id, err := store.ResolveAPIKey(ctx, token)
		if err != nil {
			return nil, false
		}
		return id, true
	}

	// Fallback: session cookie.
	cookie, err := r.Cookie("specgraph_session")
	if err == nil && cookie.Value != "" {
		id, storeErr := store.ResolveAPIKey(ctx, cookie.Value)
		if storeErr != nil {
			return nil, false
		}
		return id, true
	}

	return nil, false
}
