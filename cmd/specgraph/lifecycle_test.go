// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/driftscope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDriftScopeToProto(t *testing.T) {
	tests := []struct {
		input string
		want  specv1.DriftScope
	}{
		{"deps", specv1.DriftScope_DRIFT_SCOPE_DEPS},
		{"interfaces", specv1.DriftScope_DRIFT_SCOPE_INTERFACES},
		{"verify", specv1.DriftScope_DRIFT_SCOPE_VERIFY},
		{"", specv1.DriftScope_DRIFT_SCOPE_UNSPECIFIED},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := driftScopeToProto(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDriftScopeToProto_UnknownReturnsError(t *testing.T) {
	_, err := driftScopeToProto("bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid scope")
	assert.Contains(t, err.Error(), "bogus")
}

func TestDriftScopeToProtoMap_Completeness(t *testing.T) {
	expected := []string{"", "deps", "interfaces", "verify"}
	for _, scope := range expected {
		_, ok := driftScopeToProtoMap[scope]
		assert.True(t, ok, "expected scope %q in driftScopeToProtoMap", scope)
	}
}

func TestDriftScopeToProtoMap_SyncWithDriftscope(t *testing.T) {
	for scope := range driftScopeToProtoMap {
		assert.True(t, driftscope.IsValid(scope),
			"CLI scope %q not recognized by driftscope.IsValid — tables out of sync", scope)
	}
}
