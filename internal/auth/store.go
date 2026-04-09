// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
)

// ErrUnknownKey is returned when an API key is not recognized.
var ErrUnknownKey = errors.New("unknown API key")

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
