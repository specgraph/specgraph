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

// Identity represents an authenticated principal.
type Identity struct {
	Subject     string          // "local:<user>" | "apikey:<id>" | "oidc:<sub>"
	DisplayName string          // human-friendly name
	Role        Role            // role name (built-in or custom)
	Permissions map[string]bool // raw entries from role definition
	Source      string          // "local" | "apikey" | "oidc"
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
