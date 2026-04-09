// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeHealthHandler returns a successful Health response.
type fakeHealthHandler struct {
	specgraphv1connect.UnimplementedServerServiceHandler
}

func (fakeHealthHandler) Health(_ context.Context, _ *connect.Request[specv1.HealthRequest]) (*connect.Response[specv1.HealthResponse], error) {
	return connect.NewResponse(&specv1.HealthResponse{
		Status:  "ok",
		Version: "1.0.0",
	}), nil
}

// fakeHealthErrorHandler returns a permission-denied RPC error (not a net
// error), so runStatus should propagate it rather than printing "not running".
type fakeHealthErrorHandler struct {
	specgraphv1connect.UnimplementedServerServiceHandler
}

func (fakeHealthErrorHandler) Health(_ context.Context, _ *connect.Request[specv1.HealthRequest]) (*connect.Response[specv1.HealthResponse], error) {
	return nil, connect.NewError(connect.CodePermissionDenied, nil)
}

func TestRunStatus_HappyPath(t *testing.T) {
	startFakeServerServiceServer(t, fakeHealthHandler{})

	old := statusJSON
	statusJSON = false
	t.Cleanup(func() { statusJSON = old })

	err := runStatus(newCmdWithCtx(), nil)
	require.NoError(t, err)
}

func TestRunStatus_HappyPath_JSON(t *testing.T) {
	startFakeServerServiceServer(t, fakeHealthHandler{})

	old := statusJSON
	statusJSON = true
	t.Cleanup(func() { statusJSON = old })

	err := runStatus(newCmdWithCtx(), nil)
	require.NoError(t, err)
}

func TestRunStatus_ClientError(t *testing.T) {
	setMissingConfig(t)

	err := runStatus(newCmdWithCtx(), nil)
	require.Error(t, err)
}

func TestRunStatus_NotRunning(t *testing.T) {
	// Point at a URL where nothing is listening — triggers the net.Error branch.
	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte("server:\n  remote: http://127.0.0.1:1\n"), 0o600))
	old := cfgFile
	cfgFile = cfgPath
	t.Cleanup(func() { cfgFile = old })

	oldJSON := statusJSON
	statusJSON = false
	t.Cleanup(func() { statusJSON = oldJSON })

	// Should NOT return an error — prints "not running" instead.
	err := runStatus(newCmdWithCtx(), nil)
	require.NoError(t, err)
}

func TestRunStatus_RPCError(t *testing.T) {
	startFakeServerServiceServer(t, fakeHealthErrorHandler{})

	old := statusJSON
	statusJSON = false
	t.Cleanup(func() { statusJSON = old })

	err := runStatus(newCmdWithCtx(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "health check")
}
