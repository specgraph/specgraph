// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"net/http"
)

type projectKey struct{}

// ProjectFromContext extracts the project slug from the request context.
func ProjectFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(projectKey{}).(string); ok {
		return v
	}
	return ""
}

// ProjectMiddleware extracts X-Specgraph-Project header and adds to context.
func ProjectMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		project := r.Header.Get("X-Specgraph-Project")
		if project != "" {
			r = r.WithContext(context.WithValue(r.Context(), projectKey{}, project))
		}
		next.ServeHTTP(w, r)
	})
}
