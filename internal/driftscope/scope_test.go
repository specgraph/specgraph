// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package driftscope

import "testing"

func TestIsValid(t *testing.T) {
	t.Parallel()

	validCases := []struct {
		name  string
		scope string
	}{
		{"empty string (all scopes)", ""},
		{"deps", "deps"},
		{"interfaces", "interfaces"},
		{"verify", "verify"},
	}

	for _, tc := range validCases {
		t.Run("valid/"+tc.name, func(t *testing.T) {
			t.Parallel()
			if !IsValid(tc.scope) {
				t.Errorf("IsValid(%q) = false, want true", tc.scope)
			}
		})
	}

	invalidCases := []struct {
		name  string
		scope string
	}{
		{"unknown", "unknown"},
		{"all", "all"},
		{"dependency", "dependency"},
	}

	for _, tc := range invalidCases {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			t.Parallel()
			if IsValid(tc.scope) {
				t.Errorf("IsValid(%q) = true, want false", tc.scope)
			}
		})
	}
}
