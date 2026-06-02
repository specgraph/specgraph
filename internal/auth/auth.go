// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package auth provides authentication and authorization for SpecGraph RPCs.
package auth

import "strings"

// Role represents a named authorization role.
type Role string

// Built-in roles.
const (
	RoleAdmin  Role = "admin"
	RoleWriter Role = "writer"
	RoleReader Role = "reader"
)

// Identity represents an authenticated principal. Produced by Resolver.Resolve
// (or, until the Phase B cutover, by the legacy IdentityStore methods);
// consumed by the interceptor and by Authorizer implementations.
//
// Subject keeps its original namespacing format ("apikey:<id>", "oidc:<sub>",
// historically "local:<user>") for log-filter and dashboard compatibility.
// After Phase B the "local:" prefix is no longer produced; the constant is
// retained as a historical comment only.
type Identity struct {
	// New fields (populated by Resolver.Resolve from Task 12 onward).
	UserID        string // uuid (storage.User.ID); empty for legacy paths
	EffectiveRole Role   // min(Role, key.RoleDowngrade); equals Role for OIDC
	Email         string // from User row; populated by new resolver only

	// Existing fields.
	Subject     string          // "apikey:<id>" | "oidc:<sub>" | (legacy) "local:<user>"
	DisplayName string          // human-friendly name
	Role        Role            // role name (built-in or custom)
	Permissions map[string]bool // raw entries from role definition (legacy; removed in Phase C)
	Source      string          // "local" | "apikey" | "oidc" ("local" removed in Phase C)
}

// HasPermission checks whether perms satisfies the required permission.
// Supports wildcards: "*:*" (full), "*:read" (action), "spec:*" (service).
func HasPermission(perms map[string]bool, required string) bool {
	if len(perms) == 0 {
		return false
	}
	if perms["*:*"] {
		return true
	}
	if perms[required] {
		return true
	}
	parts := strings.SplitN(required, ":", 2)
	if len(parts) != 2 {
		return false
	}
	service, action := parts[0], parts[1]
	if perms[service+":*"] {
		return true
	}
	if perms["*:"+action] {
		return true
	}
	return false
}
