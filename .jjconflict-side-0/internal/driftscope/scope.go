// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package driftscope defines valid drift scope values shared between the
// server handler layer and the drift engine.
package driftscope

// validScopes is the set of recognized drift scope strings.
// Empty string means "all scopes".
var validScopes = map[string]bool{
	"":           true,
	"deps":       true,
	"interfaces": true,
	"verify":     true,
}

// IsValid reports whether scope is a recognized drift scope value.
func IsValid(scope string) bool {
	return validScopes[scope]
}
