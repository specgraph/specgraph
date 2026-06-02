// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
)

func TestStaticTableAuthorizer_AllowOnMatch(t *testing.T) {
	a := auth.NewStaticTableAuthorizer(map[auth.Role][]string{
		auth.RoleReader: {"*:read"},
	})
	id := &auth.Identity{Role: auth.RoleReader, EffectiveRole: auth.RoleReader}
	d, err := a.Authorize(context.Background(), id,
		specgraphv1connect.SpecServiceGetSpecProcedure, nil)
	require.NoError(t, err)
	require.True(t, d.Allowed)
	require.Contains(t, d.Reason, "static-table-allow")
}

func TestStaticTableAuthorizer_DenyOnRoleMismatch(t *testing.T) {
	a := auth.NewStaticTableAuthorizer(map[auth.Role][]string{
		auth.RoleReader: {"*:read"},
	})
	id := &auth.Identity{Role: auth.RoleReader, EffectiveRole: auth.RoleReader}
	d, err := a.Authorize(context.Background(), id,
		specgraphv1connect.SpecServiceCreateSpecProcedure, nil)
	require.NoError(t, err)
	require.False(t, d.Allowed)
	require.Contains(t, d.Reason, "static-table-deny")
}

func TestStaticTableAuthorizer_DenyOnUnknownRole(t *testing.T) {
	// Authorizer is built WITHOUT the identity's role (RoleWriter) loaded,
	// so the role lookup misses entirely — distinct from the
	// permission-insufficient path exercised above.
	a := auth.NewStaticTableAuthorizer(map[auth.Role][]string{
		auth.RoleReader: {"*:read"},
	})
	id := &auth.Identity{Role: auth.RoleWriter, EffectiveRole: auth.RoleWriter}
	d, err := a.Authorize(context.Background(), id,
		specgraphv1connect.SpecServiceGetSpecProcedure, nil)
	require.NoError(t, err)
	require.False(t, d.Allowed)
	require.Contains(t, d.Reason, "static-table-deny:unknown-role:")
}

func TestStaticTableAuthorizer_ErrorOnUnconfigured(t *testing.T) {
	a := auth.NewStaticTableAuthorizer(nil)
	id := &auth.Identity{Role: auth.RoleAdmin, EffectiveRole: auth.RoleAdmin}
	_, err := a.Authorize(context.Background(), id, "/unconfigured/procedure", nil)
	require.Error(t, err)
}

func TestStaticTableAuthorizer_AdminWildcard(t *testing.T) {
	a := auth.NewStaticTableAuthorizer(map[auth.Role][]string{
		auth.RoleAdmin: {"*:*"},
	})
	id := &auth.Identity{Role: auth.RoleAdmin, EffectiveRole: auth.RoleAdmin}
	d, err := a.Authorize(context.Background(), id,
		specgraphv1connect.SpecServiceCreateSpecProcedure, nil)
	require.NoError(t, err)
	require.True(t, d.Allowed)
}
