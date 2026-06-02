// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package auth_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
)

// newRealCedarAuthorizer builds the production authorizer: embedded policies,
// real engine, real action map.
func newRealCedarAuthorizer(t *testing.T) auth.Authorizer {
	t.Helper()
	engine, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{auth.NewEmbeddedPolicySource()}, auth.ActionNames())
	require.NoError(t, err)
	return auth.NewCedarAuthorizer(engine)
}

func TestIntegration_CedarAuthorizer_EmbeddedPolicies(t *testing.T) {
	a := newRealCedarAuthorizer(t)
	ctx := context.Background()

	cases := []struct {
		role      auth.Role
		procedure string
		allowed   bool
	}{
		{auth.RoleReader, specgraphv1connect.SpecServiceGetSpecProcedure, true},
		{auth.RoleReader, specgraphv1connect.SpecServiceCreateSpecProcedure, false},
		{auth.RoleReader, specgraphv1connect.GraphServiceRemoveEdgeProcedure, false},
		{auth.RoleWriter, specgraphv1connect.SpecServiceCreateSpecProcedure, true},
		{auth.RoleWriter, specgraphv1connect.GraphServiceRemoveEdgeProcedure, false},
		{auth.RoleAdmin, specgraphv1connect.GraphServiceRemoveEdgeProcedure, true},
	}
	for _, c := range cases {
		id := &auth.Identity{UserID: "u1", EffectiveRole: c.role, Role: c.role, Subject: "apikey:k1"}
		d, err := a.Authorize(ctx, id, c.procedure, nil)
		require.NoErrorf(t, err, "role=%s proc=%s", c.role, c.procedure)
		require.Equalf(t, c.allowed, d.Allowed, "role=%s proc=%s reason=%s", c.role, c.procedure, d.Reason)
	}
}

func TestIntegration_HealthIsExempt(t *testing.T) {
	// Exempt procedures never reach the authorizer; assert the interceptor's
	// gate directly.
	require.True(t, auth.IsExempt(specgraphv1connect.ServerServiceHealthProcedure))
}

// TestIntegration_DiscretePolicyLayering shows a directory-sourced ownership
// policy granting a reader access to a write action on their OWN resource —
// something the role-only base policy denies — while a non-owner reader is
// still denied. New behavior arrives as a new policy file, no code change.
func TestIntegration_DiscretePolicyLayering(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	// Owner may rotate their own resource regardless of role.
	ownerPolicy := `permit (
		principal,
		action in SpecGraph::Action::"write",
		resource
	) when {
		resource has owner_user_id && principal has id && resource.owner_user_id == principal.id
	};`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ownership.cedar"), []byte(ownerPolicy), 0o600))

	// Guard: the base policy ALONE must deny the owner's write — this proves
	// the layered ownership policy below is genuinely load-bearing.
	baseOnly, err := auth.NewCedarEngine(ctx,
		[]auth.PolicySource{auth.NewEmbeddedPolicySource()}, auth.ActionNames())
	require.NoError(t, err)
	baseDec, err := baseOnly.Evaluate(ctx, auth.EvalRequest{
		Identity: &auth.Identity{UserID: "u1", EffectiveRole: auth.RoleReader},
		Action:   "spec.write",
		Resource: auth.ResourceRef{Type: "spec", ID: "s1", Attributes: map[string]string{"owner_user_id": "u1"}},
	})
	require.NoError(t, err)
	require.False(t, baseDec.Allowed, "base policy alone must deny — confirms the ownership policy is load-bearing")

	engine, err := auth.NewCedarEngine(ctx,
		[]auth.PolicySource{auth.NewEmbeddedPolicySource(), auth.NewDirectoryPolicySource(dir)},
		auth.ActionNames())
	require.NoError(t, err)

	// Reader who owns the resource: base denies write, ownership permit allows.
	ownerDec, err := engine.Evaluate(ctx, auth.EvalRequest{
		Identity: &auth.Identity{UserID: "u1", EffectiveRole: auth.RoleReader},
		Action:   "spec.write",
		Resource: auth.ResourceRef{Type: "spec", ID: "s1", Attributes: map[string]string{"owner_user_id": "u1"}},
	})
	require.NoError(t, err)
	require.True(t, ownerDec.Allowed, "owner reader should be allowed by the layered ownership policy")

	// Reader who does NOT own the resource: neither base nor ownership matches.
	otherDec, err := engine.Evaluate(ctx, auth.EvalRequest{
		Identity: &auth.Identity{UserID: "u2", EffectiveRole: auth.RoleReader},
		Action:   "spec.write",
		Resource: auth.ResourceRef{Type: "spec", ID: "s1", Attributes: map[string]string{"owner_user_id": "u1"}},
	})
	require.NoError(t, err)
	require.False(t, otherDec.Allowed, "non-owner reader stays denied — layering is additive, not blanket")
}
