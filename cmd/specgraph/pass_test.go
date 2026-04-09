// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"io"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePassRunHandler struct {
	specgraphv1connect.UnimplementedAnalyticalPassServiceHandler
}

func (fakePassRunHandler) RunAnalyticalPass(_ context.Context, req *connect.Request[specv1.RunAnalyticalPassRequest]) (*connect.Response[specv1.RunAnalyticalPassResponse], error) {
	return connect.NewResponse(&specv1.RunAnalyticalPassResponse{
		PassType:       req.Msg.PassType,
		PromptTemplate: "You are a test analyst.",
		Tools: []*specv1.ToolReference{
			{Name: "show_spec", Command: "specgraph show \"test-slug\"", Description: "Read the spec"},
		},
		InitialMessage: "Run the pass.",
		Stage:          "shape",
		OfferedPasses:  []string{"peripheral_vision"},
	}), nil
}

type fakePassRunErrorHandler struct {
	specgraphv1connect.UnimplementedAnalyticalPassServiceHandler
}

func (fakePassRunErrorHandler) RunAnalyticalPass(_ context.Context, _ *connect.Request[specv1.RunAnalyticalPassRequest]) (*connect.Response[specv1.RunAnalyticalPassResponse], error) {
	return nil, connect.NewError(connect.CodeNotFound, nil)
}

func TestRunPassRun_Markdown(t *testing.T) {
	startFakeServer[specgraphv1connect.AnalyticalPassServiceHandler](t, fakePassRunHandler{}, specgraphv1connect.NewAnalyticalPassServiceHandler)

	oldPT := passRunPassType
	oldJSON := passRunJSON
	passRunPassType = "constitution-check"
	passRunJSON = false
	t.Cleanup(func() {
		passRunPassType = oldPT
		passRunJSON = oldJSON
	})

	cmd := newCmdWithCtx()
	cmd.SetOut(io.Discard)
	err := runPassRun(cmd, []string{"test-slug"})
	require.NoError(t, err)
}

func TestRunPassRun_JSON(t *testing.T) {
	startFakeServer[specgraphv1connect.AnalyticalPassServiceHandler](t, fakePassRunHandler{}, specgraphv1connect.NewAnalyticalPassServiceHandler)

	oldPT := passRunPassType
	oldJSON := passRunJSON
	passRunPassType = "constitution-check"
	passRunJSON = true
	t.Cleanup(func() {
		passRunPassType = oldPT
		passRunJSON = oldJSON
	})

	cmd := newCmdWithCtx()
	cmd.SetOut(io.Discard)
	err := runPassRun(cmd, []string{"test-slug"})
	require.NoError(t, err)
}

func TestRunPassRun_MissingPassType(t *testing.T) {
	startFakeServer[specgraphv1connect.AnalyticalPassServiceHandler](t, fakePassRunHandler{}, specgraphv1connect.NewAnalyticalPassServiceHandler)

	oldPT := passRunPassType
	passRunPassType = ""
	t.Cleanup(func() { passRunPassType = oldPT })

	err := runPassRun(newCmdWithCtx(), []string{"test-slug"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pass-type is required")
}

func TestRunPassRun_InvalidPassType(t *testing.T) {
	startFakeServer[specgraphv1connect.AnalyticalPassServiceHandler](t, fakePassRunHandler{}, specgraphv1connect.NewAnalyticalPassServiceHandler)

	oldPT := passRunPassType
	passRunPassType = "bogus"
	t.Cleanup(func() { passRunPassType = oldPT })

	err := runPassRun(newCmdWithCtx(), []string{"test-slug"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown pass type")
}

func TestRunPassRun_RPCError(t *testing.T) {
	startFakeServer[specgraphv1connect.AnalyticalPassServiceHandler](t, fakePassRunErrorHandler{}, specgraphv1connect.NewAnalyticalPassServiceHandler)

	oldPT := passRunPassType
	oldJSON := passRunJSON
	passRunPassType = "constitution-check"
	passRunJSON = false
	t.Cleanup(func() {
		passRunPassType = oldPT
		passRunJSON = oldJSON
	})

	err := runPassRun(newCmdWithCtx(), []string{"test-slug"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "run analytical pass")
}

func TestRunPassRun_ClientError(t *testing.T) {
	setMissingConfig(t)

	oldPT := passRunPassType
	passRunPassType = "constitution-check"
	t.Cleanup(func() { passRunPassType = oldPT })

	err := runPassRun(newCmdWithCtx(), []string{"test-slug"})
	require.Error(t, err)
}

func TestPassRunCmd_RequiresSlug(t *testing.T) {
	err := passRunCmd.Args(passRunCmd, []string{})
	require.Error(t, err)
}
