// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
)

func TestIsExempt_Health(t *testing.T) {
	require.True(t, auth.IsExempt(specgraphv1connect.ServerServiceHealthProcedure))
}

func TestIsExempt_NormalProcedureNotExempt(t *testing.T) {
	require.False(t, auth.IsExempt(specgraphv1connect.SpecServiceGetSpecProcedure))
}
