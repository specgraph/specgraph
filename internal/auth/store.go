// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
)

// ErrUnknownKey is returned when an API key is not recognized.
var ErrUnknownKey = errors.New("unknown API key")

// IdentityStore resolves authentication tokens to identities.
type IdentityStore interface {
	// ResolveAPIKey returns the identity for the given API key.
	// Returns ErrUnknownKey if the key is not recognized.
	ResolveAPIKey(ctx context.Context, key string) (*Identity, error)

	// HasKeys reports whether any API keys are configured.
	// When false, unauthenticated requests fall back to the implicit local identity.
	HasKeys() bool
}
