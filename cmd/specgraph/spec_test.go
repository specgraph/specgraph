// SPDX-License-Identifier: Apache-2.0
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

// --- fake handlers ---

type fakeCreateSpecHandler struct {
	specgraphv1connect.UnimplementedSpecServiceHandler
}

func (fakeCreateSpecHandler) CreateSpec(_ context.Context, req *connect.Request[specv1.CreateSpecRequest]) (*connect.Response[specv1.CreateSpecResponse], error) {
	return connect.NewResponse(&specv1.CreateSpecResponse{
		Spec: &specv1.Spec{
			Id:   "spec-01ABC",
			Slug: req.Msg.GetSlug(),
		},
	}), nil
}

type fakeCreateSpecErrHandler struct {
	specgraphv1connect.UnimplementedSpecServiceHandler
}

func (fakeCreateSpecErrHandler) CreateSpec(_ context.Context, _ *connect.Request[specv1.CreateSpecRequest]) (*connect.Response[specv1.CreateSpecResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, errors.New("db exploded"))
}

type fakeUpdateSpecHandler struct {
	specgraphv1connect.UnimplementedSpecServiceHandler
}

func (fakeUpdateSpecHandler) UpdateSpec(_ context.Context, req *connect.Request[specv1.UpdateSpecRequest]) (*connect.Response[specv1.UpdateSpecResponse], error) {
	return connect.NewResponse(&specv1.UpdateSpecResponse{
		Spec: &specv1.Spec{
			Slug:    req.Msg.GetSlug(),
			Version: 3,
		},
	}), nil
}

type fakeUpdateSpecErrHandler struct {
	specgraphv1connect.UnimplementedSpecServiceHandler
}

func (fakeUpdateSpecErrHandler) UpdateSpec(_ context.Context, _ *connect.Request[specv1.UpdateSpecRequest]) (*connect.Response[specv1.UpdateSpecResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, errors.New("update failed"))
}

type fakeListSpecsHandler struct {
	specgraphv1connect.UnimplementedSpecServiceHandler
}

func (fakeListSpecsHandler) ListSpecs(_ context.Context, _ *connect.Request[specv1.ListSpecsRequest]) (*connect.Response[specv1.ListSpecsResponse], error) {
	return connect.NewResponse(&specv1.ListSpecsResponse{
		Specs: []*specv1.Spec{
			{Slug: "spec-a", Stage: "spark"},
			{Slug: "spec-b", Stage: "shape"},
		},
	}), nil
}

type fakeListSpecsEmptyHandler struct {
	specgraphv1connect.UnimplementedSpecServiceHandler
}

func (fakeListSpecsEmptyHandler) ListSpecs(_ context.Context, _ *connect.Request[specv1.ListSpecsRequest]) (*connect.Response[specv1.ListSpecsResponse], error) {
	return connect.NewResponse(&specv1.ListSpecsResponse{}), nil
}

type fakeListSpecsErrHandler struct {
	specgraphv1connect.UnimplementedSpecServiceHandler
}

func (fakeListSpecsErrHandler) ListSpecs(_ context.Context, _ *connect.Request[specv1.ListSpecsRequest]) (*connect.Response[specv1.ListSpecsResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, errors.New("list failed"))
}

type fakeGetSpecHandler struct {
	specgraphv1connect.UnimplementedSpecServiceHandler
}

func (fakeGetSpecHandler) GetSpec(_ context.Context, req *connect.Request[specv1.GetSpecRequest]) (*connect.Response[specv1.GetSpecResponse], error) {
	return connect.NewResponse(&specv1.GetSpecResponse{
		Spec: &specv1.Spec{
			Id:       "spec-01XYZ",
			Slug:     req.Msg.GetSlug(),
			Intent:   "do something",
			Stage:    "spark",
			Priority: "p1",
			Version:  1,
		},
	}), nil
}

type fakeGetSpecErrHandler struct {
	specgraphv1connect.UnimplementedSpecServiceHandler
}

func (fakeGetSpecErrHandler) GetSpec(_ context.Context, _ *connect.Request[specv1.GetSpecRequest]) (*connect.Response[specv1.GetSpecResponse], error) {
	return nil, connect.NewError(connect.CodeNotFound, errors.New("spec not found"))
}

// --- happy-path tests ---

func TestRunCreate_HappyPath(t *testing.T) {
	startFakeSpecServer(t, fakeCreateSpecHandler{})
	err := runCreate(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunUpdate_HappyPath(t *testing.T) {
	startFakeSpecServer(t, fakeUpdateSpecHandler{})

	cmd := newCmdWithCtx()
	cmd.Flags().String("intent", "", "")
	cmd.Flags().String("stage", "", "")
	cmd.Flags().String("priority", "", "")
	cmd.Flags().String("complexity", "", "")
	cmd.Flags().String("notes", "", "")

	old := updateIntent
	updateIntent = "new intent"
	t.Cleanup(func() { updateIntent = old })
	require.NoError(t, cmd.Flags().Set("intent", "new intent"))

	err := runUpdate(cmd, []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunList_HappyPath(t *testing.T) {
	startFakeSpecServer(t, fakeListSpecsHandler{})

	old := listJSON
	listJSON = false
	t.Cleanup(func() { listJSON = old })

	err := runList(newCmdWithCtx(), nil)
	require.NoError(t, err)
}

func TestRunList_HappyPath_JSON(t *testing.T) {
	startFakeSpecServer(t, fakeListSpecsHandler{})

	old := listJSON
	listJSON = true
	t.Cleanup(func() { listJSON = old })

	cmd := newCmdWithCtx()
	err := runList(cmd, nil)
	require.NoError(t, err)
}

func TestRunList_EmptyResults(t *testing.T) {
	startFakeSpecServer(t, fakeListSpecsEmptyHandler{})

	old := listJSON
	listJSON = false
	t.Cleanup(func() { listJSON = old })

	err := runList(newCmdWithCtx(), nil)
	require.NoError(t, err)
}

func TestRunShow_HappyPath(t *testing.T) {
	startFakeSpecServer(t, fakeGetSpecHandler{})

	old := showJSON
	showJSON = false
	t.Cleanup(func() { showJSON = old })

	err := runShow(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunShow_HappyPath_JSON(t *testing.T) {
	startFakeSpecServer(t, fakeGetSpecHandler{})

	old := showJSON
	showJSON = true
	t.Cleanup(func() { showJSON = old })

	cmd := newCmdWithCtx()
	err := runShow(cmd, []string{"my-spec"})
	require.NoError(t, err)
}

// --- RPC error tests ---

func TestRunCreate_RPCError(t *testing.T) {
	startFakeSpecServer(t, fakeCreateSpecErrHandler{})
	err := runCreate(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create spec")
}

func TestRunUpdate_RPCError(t *testing.T) {
	startFakeSpecServer(t, fakeUpdateSpecErrHandler{})

	cmd := newCmdWithCtx()
	cmd.Flags().String("intent", "", "")
	cmd.Flags().String("stage", "", "")
	cmd.Flags().String("priority", "", "")
	cmd.Flags().String("complexity", "", "")
	cmd.Flags().String("notes", "", "")

	err := runUpdate(cmd, []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update spec")
}

func TestRunShow_RPCError(t *testing.T) {
	startFakeSpecServer(t, fakeGetSpecErrHandler{})

	old := showJSON
	showJSON = false
	t.Cleanup(func() { showJSON = old })

	err := runShow(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get spec")
}

func TestRunList_RPCError(t *testing.T) {
	startFakeSpecServer(t, fakeListSpecsErrHandler{})

	old := listJSON
	listJSON = false
	t.Cleanup(func() { listJSON = old })

	err := runList(newCmdWithCtx(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list specs")
}

// --- cobra arg validation tests ---

func TestCreateCmd_RequiresSlug(t *testing.T) {
	err := createCmd.Args(createCmd, []string{})
	require.Error(t, err)
}

func TestUpdateCmd_RequiresSlug(t *testing.T) {
	err := updateCmd.Args(updateCmd, []string{})
	require.Error(t, err)
}

func TestShowCmd_RequiresSlug(t *testing.T) {
	err := showCmd.Args(showCmd, []string{})
	require.Error(t, err)
}

func TestListCmd_AcceptsNoArgs(t *testing.T) {
	// listCmd has no Args validator set (nil), which means cobra allows any args.
	assert.Nil(t, listCmd.Args, "listCmd should not restrict args")
}
