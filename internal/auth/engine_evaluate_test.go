// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
)

func newMatrixEngine(t *testing.T) auth.PolicyEngine {
	t.Helper()
	eng, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{baseSource()}, testActions())
	require.NoError(t, err)
	return eng
}

func evalRole(t *testing.T, eng auth.PolicyEngine, role auth.Role, action string) bool {
	t.Helper()
	dec, err := eng.Evaluate(context.Background(), auth.EvalRequest{
		Identity: &auth.Identity{UserID: "u1", EffectiveRole: role, Role: role},
		Action:   action,
		Resource: auth.ResourceRef{Type: "spec"},
	})
	require.NoError(t, err)
	return dec.Allowed
}

func TestEvaluate_RoleVerbMatrix(t *testing.T) {
	eng := newMatrixEngine(t)
	cases := []struct {
		role    auth.Role
		action  string
		allowed bool
	}{
		{auth.RoleReader, "spec.read", true},
		{auth.RoleReader, "spec.write", false},
		{auth.RoleReader, "graph.delete", false},
		{auth.RoleWriter, "spec.read", true},
		{auth.RoleWriter, "spec.write", true},
		{auth.RoleWriter, "graph.delete", false},
		{auth.RoleAdmin, "spec.read", true},
		{auth.RoleAdmin, "spec.write", true},
		{auth.RoleAdmin, "graph.delete", true},
	}
	for _, c := range cases {
		require.Equalf(t, c.allowed, evalRole(t, eng, c.role, c.action),
			"role=%s action=%s", c.role, c.action)
	}
}

func TestEvaluate_UnknownRoleDenied(t *testing.T) {
	eng := newMatrixEngine(t)
	require.False(t, evalRole(t, eng, auth.Role("auditor"), "spec.read"),
		"a role with no matching policy is denied by default")
}

func TestEvaluate_AllowCitesPolicy(t *testing.T) {
	eng := newMatrixEngine(t)
	dec, err := eng.Evaluate(context.Background(), auth.EvalRequest{
		Identity: &auth.Identity{UserID: "u1", EffectiveRole: auth.RoleAdmin, Role: auth.RoleAdmin},
		Action:   "graph.delete",
		Resource: auth.ResourceRef{Type: "graph"},
	})
	require.NoError(t, err)
	require.True(t, dec.Allowed)
	require.NotEmpty(t, dec.MatchedPolicies, "an allow must cite the matching policy id")
}

func TestEvaluate_DowngradedRoleIsEnforced(t *testing.T) {
	eng := newMatrixEngine(t)
	// A writer token downgraded to reader (EffectiveRole != Role) must be
	// authorized on the DOWNGRADED role: the write is denied.
	dec, err := eng.Evaluate(context.Background(), auth.EvalRequest{
		Identity: &auth.Identity{
			UserID:        "u1",
			Role:          auth.RoleWriter, // raw assignment
			EffectiveRole: auth.RoleReader, // downgraded via API-key RoleDowngrade
		},
		Action:   "spec.write",
		Resource: auth.ResourceRef{Type: "spec"},
	})
	require.NoError(t, err)
	require.False(t, dec.Allowed, "downgraded writer must not be allowed to write")

	// And the downgraded role's own permission still works (read allowed).
	require.True(t, evalRole(t, eng, auth.RoleReader, "spec.read"))
}

const basePoliciesWithManage = basePolicies + `
permit (principal, action in SpecGraph::Action::"manage", resource)
when { principal has role && principal.role == "admin" };
`

func TestEvaluate_ManageVerbAdminOnly(t *testing.T) {
	eng, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{stubSource{name: "test", docs: []auth.PolicyDocument{{Source: "test:base.cedar", Text: basePoliciesWithManage}}}},
		[]string{"spec.read", "spec.write", "graph.delete", "user.manage"})
	require.NoError(t, err)

	check := func(role auth.Role) bool {
		dec, evalErr := eng.Evaluate(context.Background(), auth.EvalRequest{
			Identity: &auth.Identity{UserID: "u1", EffectiveRole: role, Role: role},
			Action:   "user.manage",
			Resource: auth.ResourceRef{Type: "user"},
		})
		require.NoError(t, evalErr)
		return dec.Allowed
	}
	require.True(t, check(auth.RoleAdmin), "admin allowed manage")
	require.False(t, check(auth.RoleWriter), "writer denied manage")
	require.False(t, check(auth.RoleReader), "reader denied manage")
}
