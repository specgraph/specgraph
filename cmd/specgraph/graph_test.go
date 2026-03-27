// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fake handlers: deps (direct) ---

type fakeGetDepsHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeGetDepsHandler) GetDependencies(_ context.Context, _ *connect.Request[specv1.GetDependenciesRequest]) (*connect.Response[specv1.GetDependenciesResponse], error) {
	return connect.NewResponse(&specv1.GetDependenciesResponse{
		Dependencies: []*specv1.NodeRef{
			{Slug: "dep-1", Label: "Spec", Stage: "spark"},
			{Slug: "dep-2", Label: "Spec", Stage: "shape"},
		},
	}), nil
}

type fakeGetDepsEmptyHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeGetDepsEmptyHandler) GetDependencies(_ context.Context, _ *connect.Request[specv1.GetDependenciesRequest]) (*connect.Response[specv1.GetDependenciesResponse], error) {
	return connect.NewResponse(&specv1.GetDependenciesResponse{}), nil
}

type fakeGetDepsErrHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeGetDepsErrHandler) GetDependencies(_ context.Context, _ *connect.Request[specv1.GetDependenciesRequest]) (*connect.Response[specv1.GetDependenciesResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, errors.New("deps exploded"))
}

// --- fake handlers: deps (transitive) ---

type fakeGetTransitiveDepsHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeGetTransitiveDepsHandler) GetTransitiveDeps(_ context.Context, _ *connect.Request[specv1.GetTransitiveDepsRequest]) (*connect.Response[specv1.GetTransitiveDepsResponse], error) {
	return connect.NewResponse(&specv1.GetTransitiveDepsResponse{
		Dependencies: []*specv1.NodeRef{
			{Slug: "tdep-1", Label: "Spec", Stage: "specify"},
			{Slug: "tdep-2", Label: "Decision", Stage: "approved"},
		},
	}), nil
}

type fakeGetTransitiveDepsErrHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeGetTransitiveDepsErrHandler) GetTransitiveDeps(_ context.Context, _ *connect.Request[specv1.GetTransitiveDepsRequest]) (*connect.Response[specv1.GetTransitiveDepsResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, errors.New("transitive deps exploded"))
}

// --- fake handlers: ready ---

type fakeGetReadyHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeGetReadyHandler) GetReady(_ context.Context, _ *connect.Request[specv1.GetReadyRequest]) (*connect.Response[specv1.GetReadyResponse], error) {
	return connect.NewResponse(&specv1.GetReadyResponse{
		Ready: []*specv1.NodeRef{
			{Slug: "ready-1", Label: "Spec", Stage: "specify"},
		},
	}), nil
}

type fakeGetReadyEmptyHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeGetReadyEmptyHandler) GetReady(_ context.Context, _ *connect.Request[specv1.GetReadyRequest]) (*connect.Response[specv1.GetReadyResponse], error) {
	return connect.NewResponse(&specv1.GetReadyResponse{}), nil
}

type fakeGetReadyErrHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeGetReadyErrHandler) GetReady(_ context.Context, _ *connect.Request[specv1.GetReadyRequest]) (*connect.Response[specv1.GetReadyResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, errors.New("ready exploded"))
}

// --- fake handlers: critical-path ---

type fakeGetCriticalPathHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeGetCriticalPathHandler) GetCriticalPath(_ context.Context, _ *connect.Request[specv1.GetCriticalPathRequest]) (*connect.Response[specv1.GetCriticalPathResponse], error) {
	return connect.NewResponse(&specv1.GetCriticalPathResponse{
		Path: []*specv1.NodeRef{
			{Slug: "cp-1", Label: "Spec", Stage: "spark"},
			{Slug: "cp-2", Label: "Spec", Stage: "shape"},
			{Slug: "cp-3", Label: "Spec", Stage: "specify"},
		},
	}), nil
}

type fakeGetCriticalPathErrHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeGetCriticalPathErrHandler) GetCriticalPath(_ context.Context, _ *connect.Request[specv1.GetCriticalPathRequest]) (*connect.Response[specv1.GetCriticalPathResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, errors.New("critical path exploded"))
}

// --- fake handlers: impact ---

type fakeGetImpactHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeGetImpactHandler) GetImpact(_ context.Context, _ *connect.Request[specv1.GetImpactRequest]) (*connect.Response[specv1.GetImpactResponse], error) {
	return connect.NewResponse(&specv1.GetImpactResponse{
		Impacted: []*specv1.NodeRef{
			{Slug: "impacted-1", Label: "Spec", Stage: "spark"},
			{Slug: "impacted-2", Label: "Spec", Stage: "decompose"},
		},
	}), nil
}

type fakeGetImpactErrHandler struct {
	specgraphv1connect.UnimplementedGraphServiceHandler
}

func (fakeGetImpactErrHandler) GetImpact(_ context.Context, _ *connect.Request[specv1.GetImpactRequest]) (*connect.Response[specv1.GetImpactResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, errors.New("impact exploded"))
}

// --- happy-path tests: deps ---

func TestRunDeps_HappyPath_Direct(t *testing.T) {
	startFakeGraphServer(t, fakeGetDepsHandler{})

	old := depsTransitive
	depsTransitive = false
	t.Cleanup(func() { depsTransitive = old })

	oldJSON := depsJSON
	depsJSON = false
	t.Cleanup(func() { depsJSON = oldJSON })

	err := runDeps(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunDeps_HappyPath_Transitive(t *testing.T) {
	startFakeGraphServer(t, fakeGetTransitiveDepsHandler{})

	old := depsTransitive
	depsTransitive = true
	t.Cleanup(func() { depsTransitive = old })

	oldJSON := depsJSON
	depsJSON = false
	t.Cleanup(func() { depsJSON = oldJSON })

	err := runDeps(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunDeps_HappyPath_JSON(t *testing.T) {
	startFakeGraphServer(t, fakeGetDepsHandler{})

	old := depsTransitive
	depsTransitive = false
	t.Cleanup(func() { depsTransitive = old })

	oldJSON := depsJSON
	depsJSON = true
	t.Cleanup(func() { depsJSON = oldJSON })

	cmd := newCmdWithCtx()
	err := runDeps(cmd, []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunDeps_EmptyResults(t *testing.T) {
	startFakeGraphServer(t, fakeGetDepsEmptyHandler{})

	old := depsTransitive
	depsTransitive = false
	t.Cleanup(func() { depsTransitive = old })

	oldJSON := depsJSON
	depsJSON = false
	t.Cleanup(func() { depsJSON = oldJSON })

	err := runDeps(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

// --- happy-path tests: ready ---

func TestRunReady_HappyPath(t *testing.T) {
	startFakeGraphServer(t, fakeGetReadyHandler{})

	old := readyJSON
	readyJSON = false
	t.Cleanup(func() { readyJSON = old })

	err := runReady(newCmdWithCtx(), nil)
	require.NoError(t, err)
}

func TestRunReady_HappyPath_JSON(t *testing.T) {
	startFakeGraphServer(t, fakeGetReadyHandler{})

	old := readyJSON
	readyJSON = true
	t.Cleanup(func() { readyJSON = old })

	cmd := newCmdWithCtx()
	err := runReady(cmd, nil)
	require.NoError(t, err)
}

func TestRunReady_EmptyResults(t *testing.T) {
	startFakeGraphServer(t, fakeGetReadyEmptyHandler{})

	old := readyJSON
	readyJSON = false
	t.Cleanup(func() { readyJSON = old })

	err := runReady(newCmdWithCtx(), nil)
	require.NoError(t, err)
}

// --- happy-path tests: critical-path ---

func TestRunCriticalPath_HappyPath(t *testing.T) {
	startFakeGraphServer(t, fakeGetCriticalPathHandler{})

	old := criticalPathJSON
	criticalPathJSON = false
	t.Cleanup(func() { criticalPathJSON = old })

	err := runCriticalPath(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunCriticalPath_HappyPath_JSON(t *testing.T) {
	startFakeGraphServer(t, fakeGetCriticalPathHandler{})

	old := criticalPathJSON
	criticalPathJSON = true
	t.Cleanup(func() { criticalPathJSON = old })

	cmd := newCmdWithCtx()
	err := runCriticalPath(cmd, []string{"my-spec"})
	require.NoError(t, err)
}

// --- happy-path tests: impact ---

func TestRunImpact_HappyPath(t *testing.T) {
	startFakeGraphServer(t, fakeGetImpactHandler{})

	old := impactJSON
	impactJSON = false
	t.Cleanup(func() { impactJSON = old })

	err := runImpact(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunImpact_HappyPath_JSON(t *testing.T) {
	startFakeGraphServer(t, fakeGetImpactHandler{})

	old := impactJSON
	impactJSON = true
	t.Cleanup(func() { impactJSON = old })

	cmd := newCmdWithCtx()
	err := runImpact(cmd, []string{"my-spec"})
	require.NoError(t, err)
}

// --- RPC error tests ---

func TestRunDeps_RPCError(t *testing.T) {
	startFakeGraphServer(t, fakeGetDepsErrHandler{})

	old := depsTransitive
	depsTransitive = false
	t.Cleanup(func() { depsTransitive = old })

	err := runDeps(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get dependencies")
}

func TestRunDeps_Transitive_RPCError(t *testing.T) {
	startFakeGraphServer(t, fakeGetTransitiveDepsErrHandler{})

	old := depsTransitive
	depsTransitive = true
	t.Cleanup(func() { depsTransitive = old })

	err := runDeps(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get transitive deps")
}

func TestRunReady_RPCError(t *testing.T) {
	startFakeGraphServer(t, fakeGetReadyErrHandler{})

	old := readyJSON
	readyJSON = false
	t.Cleanup(func() { readyJSON = old })

	err := runReady(newCmdWithCtx(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get ready")
}

func TestRunCriticalPath_RPCError(t *testing.T) {
	startFakeGraphServer(t, fakeGetCriticalPathErrHandler{})

	old := criticalPathJSON
	criticalPathJSON = false
	t.Cleanup(func() { criticalPathJSON = old })

	err := runCriticalPath(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get critical path")
}

func TestRunImpact_RPCError(t *testing.T) {
	startFakeGraphServer(t, fakeGetImpactErrHandler{})

	old := impactJSON
	impactJSON = false
	t.Cleanup(func() { impactJSON = old })

	err := runImpact(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get impact")
}

// --- cobra args validation tests ---

func TestDepsCmd_RequiresSlug(t *testing.T) {
	err := depsCmd.Args(depsCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestCriticalPathCmd_RequiresSlug(t *testing.T) {
	err := criticalPathCmd.Args(criticalPathCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestImpactCmd_RequiresSlug(t *testing.T) {
	err := impactCmd.Args(impactCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestReadyCmd_AcceptsNoArgs(t *testing.T) {
	// readyCmd has no Args validator — nil means cobra allows any args.
	assert.Nil(t, readyCmd.Args, "readyCmd should not restrict args")
}
