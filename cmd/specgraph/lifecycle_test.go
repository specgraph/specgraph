// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/assert"
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
			got := driftScopeToProto(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDriftScopeToProto_UnknownFallback(t *testing.T) {
	got := driftScopeToProto("bogus")
	assert.Equal(t, specv1.DriftScope_DRIFT_SCOPE_UNSPECIFIED, got,
		"unknown scope should fall back to UNSPECIFIED")
}

func TestDriftScopeToProtoMap_Completeness(t *testing.T) {
	expected := []string{"", "deps", "interfaces", "verify"}
	for _, scope := range expected {
		_, ok := driftScopeToProtoMap[scope]
		assert.True(t, ok, "expected scope %q in driftScopeToProtoMap", scope)
	}
}
