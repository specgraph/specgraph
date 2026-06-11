// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/specgraph/specgraph/internal/reqctx"
)

// RequireAuth returns HTTP middleware that authenticates requests via
// Bearer header or session cookie using a Resolver.
func RequireAuth(resolver Resolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := authenticate(r.Context(), resolver, r.Header)
			if err != nil {
				switch {
				case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
					return // client gone / deadline; nothing to write
				case errors.Is(err, ErrTransient):
					http.Error(w, `{"error":"transient"}`, http.StatusServiceUnavailable)
				default:
					http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
				}
				return
			}
			if info := reqctx.FromContext(r.Context()); info != nil {
				info.Identity = id.Subject
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}
