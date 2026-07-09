// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
)

func TestActionForProcedure_KnownProcedure(t *testing.T) {
	action, ok := auth.ActionForProcedure(specgraphv1connect.SpecServiceGetSpecProcedure)
	require.True(t, ok)
	require.Equal(t, "spec.read", action)
}

func TestActionForProcedure_DeleteProcedure(t *testing.T) {
	action, ok := auth.ActionForProcedure(specgraphv1connect.GraphServiceRemoveEdgeProcedure)
	require.True(t, ok)
	require.Equal(t, "graph.delete", action)
}

func TestActionForProcedure_Unconfigured(t *testing.T) {
	_, ok := auth.ActionForProcedure("/no.such/Procedure")
	require.False(t, ok)
}

func TestActionNames_AllParseToKnownVerb(t *testing.T) {
	names := auth.ActionNames()
	require.NotEmpty(t, names)
	for _, n := range names {
		idx := strings.LastIndex(n, ".")
		require.Greater(t, idx, 0, "action %q must be domain.verb", n)
		verb := n[idx+1:]
		require.Contains(t, []string{"read", "write", "delete", "manage", "self"}, verb, "action %q", n)
	}
}

// TestActionNames_SelfVerbConfinedToAPIKey is a drift guard: the "self" verb
// must appear ONLY on apikey.* actions. It keeps base.cedar's broad
// "any authenticated role" self permit from silently leaking onto some other
// domain's action (an elevation-of-privilege regression, T-02-13).
func TestActionNames_SelfVerbConfinedToAPIKey(t *testing.T) {
	sawSelf := false
	for _, n := range auth.ActionNames() {
		idx := strings.LastIndex(n, ".")
		require.Greater(t, idx, 0, "action %q must be domain.verb", n)
		if n[idx+1:] != "self" {
			continue
		}
		sawSelf = true
		require.Equalf(t, "apikey", n[:idx],
			"self verb must be confined to apikey.* actions; found %q", n)
	}
	require.True(t, sawSelf, "expected at least one apikey.self action to exist")
}

func TestActionNames_DecoupledFromMethodNames(t *testing.T) {
	for _, n := range auth.ActionNames() {
		require.NotContains(t, n, "Service", "action %q leaks an RPC service name", n)
	}
}

func TestActionForProcedure_Identity(t *testing.T) {
	cases := map[string]string{
		specgraphv1connect.IdentityServiceWhoamiProcedure:               "identity.read",
		specgraphv1connect.IdentityServiceListUsersProcedure:            "user.manage",
		specgraphv1connect.IdentityServiceGetUserProcedure:              "user.manage",
		specgraphv1connect.IdentityServiceUpdateUserRoleProcedure:       "user.manage",
		specgraphv1connect.IdentityServiceSoftDeleteUserProcedure:       "user.manage",
		specgraphv1connect.IdentityServicePurgeUserProcedure:            "user.manage",
		specgraphv1connect.IdentityServiceCreateServiceAccountProcedure: "serviceaccount.manage",
		specgraphv1connect.IdentityServiceCreateAPIKeyProcedure:         "apikey.manage",
		specgraphv1connect.IdentityServiceRevokeAPIKeyProcedure:         "apikey.manage",
		specgraphv1connect.IdentityServiceRotateAPIKeyProcedure:         "apikey.manage",
		specgraphv1connect.IdentityServiceListAPIKeysProcedure:          "apikey.manage",
		specgraphv1connect.IdentityServiceListOIDCBindingsProcedure:     "oidc.manage",
		specgraphv1connect.IdentityServiceUnbindOIDCProcedure:           "oidc.manage",
		specgraphv1connect.IdentityServiceCreateMyAPIKeyProcedure:       "apikey.self",
		specgraphv1connect.IdentityServiceListMyAPIKeysProcedure:        "apikey.self",
		specgraphv1connect.IdentityServiceRotateMyAPIKeyProcedure:       "apikey.self",
		specgraphv1connect.IdentityServiceRevokeMyAPIKeyProcedure:       "apikey.self",
		specgraphv1connect.IdentityServiceResyncUserRoleProcedure:       "user.manage",
	}
	for proc, want := range cases {
		got, ok := auth.ActionForProcedure(proc)
		require.Truef(t, ok, "procedure %s must map", proc)
		require.Equalf(t, want, got, "procedure %s", proc)
	}
}

func TestActionNames_SortedAndUnique(t *testing.T) {
	// ActionNames documents a distinct, sorted contract: strict ordering
	// asserts both (no duplicates AND ascending).
	names := auth.ActionNames()
	for i := 1; i < len(names); i++ {
		require.Less(t, names[i-1], names[i], "ActionNames must be sorted and unique")
	}
}
