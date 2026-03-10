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

// --- lifecycle CLI run function tests ---

func TestRunAmend_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runAmend(nil, []string{"my-spec"})
	require.Error(t, err)
}

func TestRunSupersede_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runSupersede(nil, []string{"my-spec"})
	require.Error(t, err)
}

func TestRunAbandon_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runAbandon(nil, []string{"my-spec"})
	require.Error(t, err)
}

func TestRunDrift_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runDrift(nil, []string{"my-spec"})
	require.Error(t, err)
}

func TestRunDriftAck_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runDriftAck(nil, []string{"my-spec"})
	require.Error(t, err)
}

func TestRunLint_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runLint(nil, nil)
	require.Error(t, err)
}

func TestAmendCmd_RequiresSlug(t *testing.T) {
	err := amendCmd.Args(amendCmd, []string{})
	require.Error(t, err)
}

func TestSupersedeCmd_RequiresSlug(t *testing.T) {
	err := supersedeCmd.Args(supersedeCmd, []string{})
	require.Error(t, err)
}

func TestAbandonCmd_RequiresSlug(t *testing.T) {
	err := abandonCmd.Args(abandonCmd, []string{})
	require.Error(t, err)
}

func TestDriftCmd_AcceptsNoArgs(t *testing.T) {
	err := driftCmd.Args(driftCmd, []string{})
	require.NoError(t, err)
}

func TestDriftCmd_AcceptsOneArg(t *testing.T) {
	err := driftCmd.Args(driftCmd, []string{"my-spec"})
	require.NoError(t, err)
}

func TestDriftAckCmd_RequiresSlug(t *testing.T) {
	err := driftAckCmd.Args(driftAckCmd, []string{})
	require.Error(t, err)
}

func TestLintCmd_AcceptsNoArgs(t *testing.T) {
	err := lintCmd.Args(lintCmd, []string{})
	require.NoError(t, err)
}

func TestLintCmd_AcceptsOneArg(t *testing.T) {
	err := lintCmd.Args(lintCmd, []string{"my-spec"})
	require.NoError(t, err)
}
