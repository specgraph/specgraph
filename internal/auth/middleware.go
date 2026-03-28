// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"net/http"
	"strings"
)

// RequireAuth returns HTTP middleware that enforces API key authentication.
// When the store has no keys configured, requests pass through with a local
// identity (matching the ConnectRPC interceptor behavior). When keys are
// configured, the Authorization header must carry a valid Bearer token.
func RequireAuth(store IdentityStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := authenticate(r.Context(), store, r.Header.Get("Authorization"))
			if !ok {
				http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}

// authenticate resolves the caller identity from an Authorization header value.
// Returns the identity and true on success, or nil and false on failure.
func authenticate(ctx context.Context, store IdentityStore, authHeader string) (*Identity, bool) {
	if authHeader == "" {
		if store.AllowUnauthenticated() {
			return localIdentity(), true
		}
		return nil, false
	}

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
