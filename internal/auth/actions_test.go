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
		require.Contains(t, []string{"read", "write", "delete"}, verb, "action %q", n)
	}
}

func TestActionNames_DecoupledFromMethodNames(t *testing.T) {
	for _, n := range auth.ActionNames() {
		require.NotContains(t, n, "Service", "action %q leaks an RPC service name", n)
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
