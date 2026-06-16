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

type interactiveLoginKey struct{}

// WithInteractiveLogin marks the context as originating from the interactive
// OIDC login callback. It is the SOLE gate distinguishing a login event from a
// per-request bearer JWT: jitResolve uses it to bypass the per-issuer JIT rate
// limiter, and login-sync (applyLoginSync — metadata + role re-evaluation) fires
// only when it is set.
//
// SECURITY: set ONLY by the OIDC callback handler
// (internal/server/auth_oidc_handler.go). Do NOT call it on any
// per-request/bearer/API-key/MCP path — a single misplaced caller would let an
// ordinary request mutate a user's role.
func WithInteractiveLogin(ctx context.Context) context.Context {
	return context.WithValue(ctx, interactiveLoginKey{}, true)
}

// InteractiveLoginFromContext reports whether the context was marked by
// WithInteractiveLogin.
func InteractiveLoginFromContext(ctx context.Context) bool {
	v, ok := ctx.Value(interactiveLoginKey{}).(bool)
	return ok && v
}
