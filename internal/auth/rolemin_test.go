// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
)

func TestRoleMin(t *testing.T) {
	cases := []struct {
		name string
		a    auth.Role
		b    auth.Role
		want auth.Role
	}{
		// Ranked min cases, both argument orders.
		{"admin,reader", auth.RoleAdmin, auth.RoleReader, auth.RoleReader},
		{"reader,admin", auth.RoleReader, auth.RoleAdmin, auth.RoleReader},
		{"writer,admin", auth.RoleWriter, auth.RoleAdmin, auth.RoleWriter},
		{"admin,writer", auth.RoleAdmin, auth.RoleWriter, auth.RoleWriter},
		{"writer,reader", auth.RoleWriter, auth.RoleReader, auth.RoleReader},
		{"reader,writer", auth.RoleReader, auth.RoleWriter, auth.RoleReader},
		{"reader,reader", auth.RoleReader, auth.RoleReader, auth.RoleReader},
		{"writer,writer", auth.RoleWriter, auth.RoleWriter, auth.RoleWriter},
		{"admin,admin", auth.RoleAdmin, auth.RoleAdmin, auth.RoleAdmin},

		// Fail-closed: an unranked/empty role on either side yields RoleReader,
		// never the fuller role (mirrors clampedRole's spgr-rjrt.9 guarantee).
		{"empty,admin", auth.Role(""), auth.RoleAdmin, auth.RoleReader},
		{"admin,empty", auth.RoleAdmin, auth.Role(""), auth.RoleReader},
		{"unknown,writer", auth.Role("superuser"), auth.RoleWriter, auth.RoleReader},
		{"writer,unknown", auth.RoleWriter, auth.Role("superuser"), auth.RoleReader},
		{"empty,empty", auth.Role(""), auth.Role(""), auth.RoleReader},
		{"unknown,unknown", auth.Role("a"), auth.Role("b"), auth.RoleReader},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, auth.RoleMin(tc.a, tc.b))
		})
	}
}
