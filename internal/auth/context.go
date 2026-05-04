// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import "context"

type contextKey struct{}

// WithIdentity returns a new context carrying the given identity.
// A nil identity is treated as absent — the original context is returned unchanged.
func WithIdentity(ctx context.Context, id *Identity) context.Context {
	if id == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, id)
}

// IdentityFromContext extracts the identity from the context.
// Returns nil, false if no identity is present (e.g., exempt RPCs).
func IdentityFromContext(ctx context.Context) (*Identity, bool) {
	id, ok := ctx.Value(contextKey{}).(*Identity)
	if !ok || id == nil {
		return nil, false
	}
	return id, true
}

type bearerTokenKey struct{}

// WithBearerToken returns a new context carrying the raw bearer token.
// An empty token is treated as absent — the original context is returned unchanged.
func WithBearerToken(ctx context.Context, token string) context.Context {
	if token == "" {
		return ctx
	}
	return context.WithValue(ctx, bearerTokenKey{}, token)
}

// BearerTokenFromContext extracts the raw bearer token from the context.
// Returns "", false if no token is present.
func BearerTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(bearerTokenKey{}).(string)
	if !ok || token == "" {
		return "", false
	}
	return token, true
}

type projectKey struct{}

// WithProject returns a new context carrying the project slug.
// An empty slug is treated as absent — the original context is returned unchanged.
func WithProject(ctx context.Context, slug string) context.Context {
	if slug == "" {
		return ctx
	}
	return context.WithValue(ctx, projectKey{}, slug)
}

// ProjectFromContext extracts the project slug from the context.
// Returns "", false if no project slug is present.
func ProjectFromContext(ctx context.Context) (string, bool) {
	slug, ok := ctx.Value(projectKey{}).(string)
	if !ok || slug == "" {
		return "", false
	}
	return slug, true
}
