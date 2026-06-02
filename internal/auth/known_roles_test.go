// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
)

func TestKnownRolesFrom_BuiltinsPlusCustom(t *testing.T) {
	known := auth.KnownRolesFrom([]string{"auditor", "", "releaser"})
	require.True(t, known[auth.RoleAdmin])
	require.True(t, known[auth.RoleWriter])
	require.True(t, known[auth.RoleReader])
	require.True(t, known[auth.Role("auditor")])
	require.True(t, known[auth.Role("releaser")])
	require.False(t, known[auth.Role("nope")])
	require.Len(t, known, 5, "empty custom role name must be dropped (3 builtins + 2 custom)")
}

func TestKnownRolesFrom_NilCustom(t *testing.T) {
	known := auth.KnownRolesFrom(nil)
	require.Len(t, known, 3)
	require.True(t, known[auth.RoleAdmin])
}
