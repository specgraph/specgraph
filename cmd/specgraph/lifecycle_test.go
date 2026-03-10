// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
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
		if scope == "" {
			continue // empty string maps to UNSPECIFIED; it is not a CLI scope
		}
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

func TestRunDrift_InvalidScope(t *testing.T) {
	old := driftScope
	driftScope = "bogus"
	t.Cleanup(func() { driftScope = old })
	err := runDrift(nil, []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid scope")
}

func TestRunDriftAck_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runDriftAck(nil, []string{"my-spec"})
	require.Error(t, err)
}

// fakeAckHandler implements only AcknowledgeDrift; all other methods return Unimplemented.
type fakeAckHandler struct {
	specgraphv1connect.UnimplementedLifecycleServiceHandler
}

func (fakeAckHandler) AcknowledgeDrift(_ context.Context, _ *connect.Request[specv1.DriftAcknowledgeRequest]) (*connect.Response[specv1.DriftAcknowledgeResponse], error) {
	return connect.NewResponse(&specv1.DriftAcknowledgeResponse{
		Report: &specv1.DriftReport{
			SpecSlug:     "stale-spec",
			Acknowledged: true,
			ItemsStale:   true,
		},
	}), nil
}

func TestRunDriftAck_ItemsStale_ExitsWithCode2(t *testing.T) {
	// Stand up a ConnectRPC server with a fake handler returning ItemsStale=true.
	mux := http.NewServeMux()
	path, handler := specgraphv1connect.NewLifecycleServiceHandler(fakeAckHandler{})
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// Write a config file pointing at the test server.
	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(fmt.Sprintf("server:\n  remote: %s\n", srv.URL)), 0o600))
	old := cfgFile
	cfgFile = cfgPath
	t.Cleanup(func() { cfgFile = old })

	// Capture exitFunc calls.
	var exitCode int
	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code; exitCalled = true }
	t.Cleanup(func() { exitFunc = oldExit })

	err := runDriftAck(nil, []string{"stale-spec"})
	require.NoError(t, err)
	assert.True(t, exitCalled, "expected exitFunc to be called")
	assert.Equal(t, 2, exitCode, "expected exit code 2 for stale items")
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
