// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import "context"

// Resolver dispatches an authentication token to an Identity.
// The interceptor and HTTP middleware depend on Resolver.
type Resolver interface {
	// Resolve returns the Identity for the given bearer token.
	//
	// Returns ErrUnauthenticated for any credential failure (missing,
	// malformed, expired, revoked, soft-deleted, JIT-rate-limited,
	// allowlist mismatch). Returns ErrTransient (wrapping the cause) for
	// backend failures (DB down, pool exhausted, etc.). Propagates
	// context.Canceled / context.DeadlineExceeded unwrapped.
	Resolve(ctx context.Context, token string) (*Identity, error)

	// ResolveLogin materializes an Identity from already-verified OIDC/oauth2
	// claims. It is the interactive-login entrypoint: the callback handler
	// obtains *OIDCClaims from LoginProvider.Exchange and passes them here,
	// bypassing Resolve's token-shape dispatch entirely. It is NEVER used for
	// bearer-JWT, API-key, session, or MCP resolution — those keep flowing
	// through Resolve unchanged (D-08). Error discipline matches Resolve:
	// ErrUnauthenticated for credential failure, ErrTransient for backend
	// failure.
	ResolveLogin(ctx context.Context, claims *OIDCClaims) (*Identity, error)

	// HasAuth reports whether any non-bootstrap, non-deleted user
	// exists. Used by warnIfNoAuthOnPublicListen at startup.
	HasAuth(ctx context.Context) (bool, error)
}
