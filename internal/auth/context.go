// SPDX-License-Identifier: MIT
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
