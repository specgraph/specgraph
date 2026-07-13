// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package auth provides authentication and authorization for SpecGraph RPCs.
package auth

// Role represents a named authorization role.
type Role string

// Built-in roles.
const (
	RoleAdmin  Role = "admin"
	RoleWriter Role = "writer"
	RoleReader Role = "reader"
)

// Identity represents an authenticated principal. Produced by Resolver.Resolve;
// consumed by the interceptor and by Authorizer implementations.
type Identity struct {
	UserID        string // uuid (storage.User.ID)
	EffectiveRole Role   // min(Role, key.RoleDowngrade); equals Role for OIDC
	Email         string // from User row
	Subject       string // "apikey:<id>" | "oidc:<sub>"
	DisplayName   string // human-friendly name
	Role          Role   // role name (built-in or custom)
	Source        string // "apikey" | "oidc"
	// Issuer carries the verified iss claim (OIDC) or the synthetic provider
	// id (oauth2) that authenticated this identity (D-09). Empty for static
	// credentials (API keys, sessions resolved by token). Threaded into
	// web_sessions.issuer at session-mint time (AUTH-05) and available for
	// future RP-initiated logout to target the correct end_session_endpoint.
	Issuer string
}
