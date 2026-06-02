// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
)

// ErrUnknownKey is returned when an API key is not recognized.
var ErrUnknownKey = errors.New("unknown API key")

// ErrUnauthenticated indicates a credential failure: missing, malformed,
// expired, revoked, soft-deleted user, JIT-rate-limited, allowlist
// mismatch, or any other "this principal isn't allowed to authenticate"
// condition. The interceptor maps this to connect.CodeUnauthenticated.
//
// Produced by the new Resolver impl (pgIdentityStore). The legacy
// IdentityStore methods still produce ErrUnknownKey etc. until Phase C.
var ErrUnauthenticated = errors.New("auth: unauthenticated")

// ErrTransient indicates a backend failure unrelated to the credential:
// database unavailable, pool exhausted, network timeout. The interceptor
// maps this to connect.CodeUnavailable so callers know to retry.
//
// Errors of this kind wrap the underlying cause; tests use errors.Is to
// detect ErrTransient.
var ErrTransient = errors.New("auth: transient backend error")

// ErrNoOIDC is returned by stores that don't support OIDC token resolution.
var ErrNoOIDC = errors.New("OIDC not configured")

// ErrUnknownIssuer is returned when a JWT's issuer doesn't match any configured provider.
var ErrUnknownIssuer = errors.New("unknown token issuer")

// ErrInvalidToken is returned when a JWT fails verification (expired, bad signature, wrong audience).
var ErrInvalidToken = errors.New("invalid token")

// IdentityStore resolves authentication tokens to identities.
type IdentityStore interface {
	// ResolveAPIKey returns the identity for the given API key.
	// Returns ErrUnknownKey if the key is not recognized.
	ResolveAPIKey(ctx context.Context, key string) (*Identity, error)

	// ResolveJWT validates a JWT and returns the identity.
	// Returns ErrNoOIDC if the store doesn't support OIDC.
	// Returns ErrUnknownIssuer if the token's issuer doesn't match any provider.
	ResolveJWT(ctx context.Context, token string) (*Identity, error)

	// HasAuth reports whether any authentication is configured (keys or OIDC providers).
	HasAuth() bool
}
