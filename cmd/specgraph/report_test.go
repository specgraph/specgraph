// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fake handlers ---

type fakeReportProgressHandler struct {
	specgraphv1connect.UnimplementedExecutionServiceHandler
}

func (fakeReportProgressHandler) ReportProgress(_ context.Context, _ *connect.Request[specv1.ReportProgressRequest]) (*connect.Response[specv1.ReportProgressResponse], error) {
	return connect.NewResponse(&specv1.ReportProgressResponse{}), nil
}

type fakeReportBlockerHandler struct {
	specgraphv1connect.UnimplementedExecutionServiceHandler
}

func (fakeReportBlockerHandler) ReportBlocker(_ context.Context, _ *connect.Request[specv1.ReportBlockerRequest]) (*connect.Response[specv1.ReportBlockerResponse], error) {
	return connect.NewResponse(&specv1.ReportBlockerResponse{}), nil
}

type fakeReportCompletionHandler struct {
	specgraphv1connect.UnimplementedExecutionServiceHandler
}

func (fakeReportCompletionHandler) ReportCompletion(_ context.Context, _ *connect.Request[specv1.ReportCompletionRequest]) (*connect.Response[specv1.ReportCompletionResponse], error) {
	return connect.NewResponse(&specv1.ReportCompletionResponse{NewStage: "done"}), nil
}

type fakeReportProgressErrorHandler struct {
	specgraphv1connect.UnimplementedExecutionServiceHandler
}

func (fakeReportProgressErrorHandler) ReportProgress(_ context.Context, _ *connect.Request[specv1.ReportProgressRequest]) (*connect.Response[specv1.ReportProgressResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, nil)
}

type fakeReportBlockerErrorHandler struct {
	specgraphv1connect.UnimplementedExecutionServiceHandler
}

func (fakeReportBlockerErrorHandler) ReportBlocker(_ context.Context, _ *connect.Request[specv1.ReportBlockerRequest]) (*connect.Response[specv1.ReportBlockerResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, nil)
}

type fakeReportCompletionErrorHandler struct {
	specgraphv1connect.UnimplementedExecutionServiceHandler
}

func (fakeReportCompletionErrorHandler) ReportCompletion(_ context.Context, _ *connect.Request[specv1.ReportCompletionRequest]) (*connect.Response[specv1.ReportCompletionResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, nil)
}

// --- happy-path tests ---

func TestRunReportProgress_HappyPath(t *testing.T) {
	startFakeExecutionServer(t, fakeReportProgressHandler{})

	oldAgent := reportProgressAgent
	oldMessage := reportProgressMessage
	reportProgressAgent = "agent-1"
	reportProgressMessage = "making progress"
	t.Cleanup(func() {
		reportProgressAgent = oldAgent
		reportProgressMessage = oldMessage
	})

	err := runReportProgress(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunReportBlocker_HappyPath(t *testing.T) {
	startFakeExecutionServer(t, fakeReportBlockerHandler{})

	oldAgent := reportBlockerAgent
	oldDesc := reportBlockerDescription
	reportBlockerAgent = "agent-1"
	reportBlockerDescription = "blocked on dependency"
	t.Cleanup(func() {
		reportBlockerAgent = oldAgent
		reportBlockerDescription = oldDesc
	})

	err := runReportBlocker(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunReportCompletion_HappyPath(t *testing.T) {
	startFakeExecutionServer(t, fakeReportCompletionHandler{})

	oldAgent := reportCompletionAgent
	reportCompletionAgent = "agent-1"
	t.Cleanup(func() { reportCompletionAgent = oldAgent })

	err := runReportCompletion(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

// --- RPC error tests ---

func TestRunReportProgress_RPCError(t *testing.T) {
	startFakeExecutionServer(t, fakeReportProgressErrorHandler{})

	oldAgent := reportProgressAgent
	oldMessage := reportProgressMessage
	reportProgressAgent = "agent-1"
	reportProgressMessage = "making progress"
	t.Cleanup(func() {
		reportProgressAgent = oldAgent
		reportProgressMessage = oldMessage
	})

	err := runReportProgress(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "report progress")
}

func TestRunReportBlocker_RPCError(t *testing.T) {
	startFakeExecutionServer(t, fakeReportBlockerErrorHandler{})

	oldAgent := reportBlockerAgent
	oldDesc := reportBlockerDescription
	reportBlockerAgent = "agent-1"
	reportBlockerDescription = "blocked on dependency"
	t.Cleanup(func() {
		reportBlockerAgent = oldAgent
		reportBlockerDescription = oldDesc
	})

	err := runReportBlocker(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "report blocker")
}

func TestRunReportCompletion_RPCError(t *testing.T) {
	startFakeExecutionServer(t, fakeReportCompletionErrorHandler{})

	oldAgent := reportCompletionAgent
	reportCompletionAgent = "agent-1"
	t.Cleanup(func() { reportCompletionAgent = oldAgent })

	err := runReportCompletion(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "report completion")
}

// --- cobra args tests ---

func TestReportProgressCmd_RequiresSlug(t *testing.T) {
	err := reportProgressCmd.Args(reportProgressCmd, []string{})
	require.Error(t, err)
}

func TestReportBlockerCmd_RequiresSlug(t *testing.T) {
	err := reportBlockerCmd.Args(reportBlockerCmd, []string{})
	require.Error(t, err)
}

func TestReportCompletionCmd_RequiresSlug(t *testing.T) {
	err := reportCompletionCmd.Args(reportCompletionCmd, []string{})
	require.Error(t, err)
}
