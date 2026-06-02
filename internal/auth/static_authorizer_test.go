// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
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

// TestLoadRolePerms_RedefineBuiltInMerges verifies that redefining a built-in
// role (e.g. admin with one extra perm) MERGES rather than replaces: the
// result still contains admin's default "*:*" AND the custom perm.
func TestLoadRolePerms_RedefineBuiltInMerges(t *testing.T) {
	perms := auth.LoadRolePerms(map[string]config.RoleConfig{
		"admin": {Permissions: []string{"audit:export"}},
	})
	adminPerms := perms[auth.RoleAdmin]
	require.Contains(t, adminPerms, "*:*",
		"redefining admin must retain its built-in *:* permission")
	require.Contains(t, adminPerms, "audit:export",
		"redefining admin must include the custom permission")
}

// TestLoadRolePerms_NewCustomRoleGetsOwnPerms verifies that a brand-new
// custom role name gets exactly its own configured permissions (no pollution
// from built-ins).
func TestLoadRolePerms_NewCustomRoleGetsOwnPerms(t *testing.T) {
	perms := auth.LoadRolePerms(map[string]config.RoleConfig{
		"auditor": {Permissions: []string{"audit:read", "logs:read"}},
	})
	auditorPerms := perms[auth.Role("auditor")]
	require.ElementsMatch(t, []string{"audit:read", "logs:read"}, auditorPerms,
		"new custom role must contain exactly its own permissions")
}

// TestLoadRolePerms_RedefineBuiltInNoDuplicates verifies that re-adding a
// permission already in the built-in set does not produce duplicates.
func TestLoadRolePerms_RedefineBuiltInNoDuplicates(t *testing.T) {
	perms := auth.LoadRolePerms(map[string]config.RoleConfig{
		"writer": {Permissions: []string{"*:read"}}, // "*:read" is already in writer
	})
	writerPerms := perms[auth.RoleWriter]
	count := 0
	for _, p := range writerPerms {
		if p == "*:read" {
			count++
		}
	}
	require.Equal(t, 1, count, "duplicate permissions must be deduplicated on merge")
}
