// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"

	"connectrpc.com/connect"
	"github.com/specgraph/specgraph/internal/storage"
)

// validProjectSlug matches kebab-case identifiers: alphanumeric + hyphens,
// no slashes/dots, 2–128 chars. Single-char slugs are rejected.
var validProjectSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,126}[a-z0-9]$`)

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

// TestInjectProject injects a project slug into the context for testing.
// This is exported for tests that call handler methods directly (bypassing HTTP).
func TestInjectProject(ctx context.Context, project string) context.Context {
	return context.WithValue(ctx, projectKey{}, project)
}

// scopeStore extracts the project slug from the request context and returns
// a project-scoped storage backend. Returns a connect error if the project
// header is missing.
func scopeStore(ctx context.Context, scoper storage.Scoper) (storage.ScopedBackend, error) {
	project := ProjectFromContext(ctx)
	if project == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("X-Specgraph-Project header required"))
	}
	if !validProjectSlug.MatchString(project) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("invalid project slug: must be kebab-case (a-z0-9 and hyphens, 2-128 chars)"))
	}
	store, err := scoper.Scoped(ctx, project)
	if err != nil {
		slog.Error("scopeStore: failed to scope storage", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal,
			errors.New("internal error"))
	}
	return store, nil
}
