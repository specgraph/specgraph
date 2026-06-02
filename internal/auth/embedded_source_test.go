// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"strings"
	"testing"

	cedar "github.com/cedar-policy/cedar-go"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
)

func TestEmbeddedPolicySource_LoadsBasePolicies(t *testing.T) {
	docs, err := auth.NewEmbeddedPolicySource().Load(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, docs, "embedded base policies must be present")

	var combined strings.Builder
	for _, d := range docs {
		require.True(t, strings.HasPrefix(d.Source, "embedded:"), "source tag: %s", d.Source)
		combined.WriteString(d.Text)
		combined.WriteString("\n")
	}

	// The embedded text must parse as valid Cedar.
	_, err = cedar.NewPolicySetFromBytes("embedded:test", []byte(combined.String()))
	require.NoError(t, err, "embedded base policies must parse")

	// Behavior anchor: the three verb groups are referenced.
	text := combined.String()
	require.Contains(t, text, `action in SpecGraph::Action::"read"`)
	require.Contains(t, text, `action in SpecGraph::Action::"write"`)
	require.Contains(t, text, `action in SpecGraph::Action::"delete"`)
	require.Contains(t, text, `principal.role == "reader"`)
	require.Contains(t, text, `principal.role == "writer"`)
	require.Contains(t, text, `principal.role == "admin"`)
}

func TestEmbeddedPolicySource_Name(t *testing.T) {
	require.Equal(t, "embedded", auth.NewEmbeddedPolicySource().Name())
}
