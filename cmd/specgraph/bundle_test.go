// SPDX-License-Identifier: MIT
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

// --- fake handlers ---

type fakeBundleHandler struct {
	specgraphv1connect.UnimplementedExecutionServiceHandler
}

func (fakeBundleHandler) GenerateBundle(_ context.Context, _ *connect.Request[specv1.GenerateBundleRequest]) (*connect.Response[specv1.GenerateBundleResponse], error) {
	return connect.NewResponse(&specv1.GenerateBundleResponse{
		Bundle: &specv1.Bundle{BundleContent: "---\nversion: 2\nslug: test\n---\n\n# Execution Bundle: test\n"},
	}), nil
}

type fakeBundleErrorHandler struct {
	specgraphv1connect.UnimplementedExecutionServiceHandler
}

func (fakeBundleErrorHandler) GenerateBundle(_ context.Context, _ *connect.Request[specv1.GenerateBundleRequest]) (*connect.Response[specv1.GenerateBundleResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, nil)
}

// --- tests ---

func TestRunBundle_HappyPath_Stdout(t *testing.T) {
	startFakeExecutionServer(t, fakeBundleHandler{})

	old := bundleOutput
	bundleOutput = ""
	t.Cleanup(func() { bundleOutput = old })

	err := runBundle(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunBundle_HappyPath_File(t *testing.T) {
	startFakeExecutionServer(t, fakeBundleHandler{})

	dir := t.TempDir()
	outPath := filepath.Join(dir, "bundle.md")

	old := bundleOutput
	bundleOutput = outPath
	t.Cleanup(func() { bundleOutput = old })

	err := runBundle(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "# Execution Bundle: test")
}

func TestRunBundle_RPCError(t *testing.T) {
	startFakeExecutionServer(t, fakeBundleErrorHandler{})

	old := bundleOutput
	bundleOutput = ""
	t.Cleanup(func() { bundleOutput = old })

	err := runBundle(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "generate bundle")
}

func TestRunBundle_ClientError(t *testing.T) {
	setMissingConfig(t)

	err := runBundle(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
}

func TestBundleCmd_RequiresSlug(t *testing.T) {
	err := bundleCmd.Args(bundleCmd, []string{})
	require.Error(t, err)
}
