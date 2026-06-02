// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import "context"

// Resolver dispatches an authentication token to an Identity. Successor
// to the legacy IdentityStore interface (which routed across multiple
// stores). The interceptor depends on Resolver after the Phase B cutover.
type Resolver interface {
	// Resolve returns the Identity for the given bearer token.
	//
	// Returns ErrUnauthenticated for any credential failure (missing,
	// malformed, expired, revoked, soft-deleted, JIT-rate-limited,
	// allowlist mismatch). Returns ErrTransient (wrapping the cause) for
	// backend failures (DB down, pool exhausted, etc.). Propagates
	// context.Canceled / context.DeadlineExceeded unwrapped.
	Resolve(ctx context.Context, token string) (*Identity, error)

	// HasAuth reports whether any non-bootstrap, non-deleted user
	// exists. Used by warnIfNoAuthOnPublicListen at startup.
	HasAuth(ctx context.Context) (bool, error)
}
