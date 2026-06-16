// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNameFromClaims(t *testing.T) {
	raw := func(s string) json.RawMessage { return json.RawMessage(`"` + s + `"`) }
	cases := []struct {
		name   string
		claims map[string]json.RawMessage
		want   string
	}{
		{"name preferred", map[string]json.RawMessage{"name": raw("Ada Lovelace"), "preferred_username": raw("ada@x.io")}, "Ada Lovelace"},
		{"name only", map[string]json.RawMessage{"name": raw("Ada Lovelace")}, "Ada Lovelace"},
		{"falls back to preferred_username", map[string]json.RawMessage{"preferred_username": raw("ada@x.io")}, "ada@x.io"},
		{"both absent", map[string]json.RawMessage{}, ""},
		{"empty name falls through", map[string]json.RawMessage{"name": raw(""), "preferred_username": raw("ada@x.io")}, "ada@x.io"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, nameFromClaims(tc.claims))
		})
	}
}
