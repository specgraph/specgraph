// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/config"
)

func rawArr(vals ...string) json.RawMessage {
	b, _ := json.Marshal(vals)
	return b
}

func TestResolveLoginRole(t *testing.T) {
	adminRule := []config.ClaimMapping{{Claim: "roles", Value: "specgraph.admin", Role: "admin"}}
	claimsAdmin := map[string]json.RawMessage{"roles": rawArr("specgraph.admin")}
	claimsNone := map[string]json.RawMessage{"roles": rawArr("specgraph.other")}

	cases := []struct {
		name        string
		mappings    []config.ClaimMapping
		claims      map[string]json.RawMessage
		current     string
		defaultRole string
		wantRole    string
		wantChanged bool
	}{
		{"rule1 no mappings -> unchanged", nil, claimsNone, "admin", "reader", "admin", false},
		{"rule1 empty slice -> unchanged", []config.ClaimMapping{}, claimsNone, "writer", "reader", "writer", false},
		{"rule2 match", adminRule, claimsAdmin, "reader", "reader", "admin", true},
		{"rule2 match no change -> changed=false", adminRule, claimsAdmin, "admin", "reader", "admin", false},
		{"rule3 no match -> default_role", adminRule, claimsNone, "admin", "reader", "reader", true},
		{"rule3 default unset -> reader", adminRule, claimsNone, "admin", "", "reader", true},
		{"rule3 numeric claim never matches", adminRule, map[string]json.RawMessage{"roles": json.RawMessage(`[1]`)}, "admin", "reader", "reader", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			role, changed := resolveLoginRole(tc.mappings, tc.claims, tc.current, tc.defaultRole)
			require.Equal(t, tc.wantRole, role)
			require.Equal(t, tc.wantChanged, changed)
		})
	}
}

func TestIsPromotion(t *testing.T) {
	cases := []struct {
		current, next string
		want          bool
	}{
		{"reader", "writer", true},
		{"writer", "admin", true},
		{"reader", "admin", true},
		{"admin", "writer", false},     // demotion
		{"writer", "writer", false},    // equal
		{"reader", "auditor", false},   // builtin -> custom (incomparable)
		{"auditor", "admin", false},    // custom -> builtin
		{"auditor", "releaser", false}, // custom -> custom
	}
	for _, tc := range cases {
		require.Equalf(t, tc.want, isPromotion(tc.current, tc.next), "%s->%s", tc.current, tc.next)
	}
}
