// SPDX-License-Identifier: MIT
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

// --- fake handlers ---

type fakeDecisionCreateHandler struct {
	specgraphv1connect.UnimplementedDecisionServiceHandler
}

func (fakeDecisionCreateHandler) CreateDecision(_ context.Context, req *connect.Request[specv1.CreateDecisionRequest]) (*connect.Response[specv1.CreateDecisionResponse], error) {
	return connect.NewResponse(&specv1.CreateDecisionResponse{
		Decision: &specv1.Decision{
			Id:   "dec-01ABC",
			Slug: req.Msg.GetSlug(),
		},
	}), nil
}

type fakeDecisionCreateErrorHandler struct {
	specgraphv1connect.UnimplementedDecisionServiceHandler
}

func (fakeDecisionCreateErrorHandler) CreateDecision(_ context.Context, _ *connect.Request[specv1.CreateDecisionRequest]) (*connect.Response[specv1.CreateDecisionResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, nil)
}

type fakeDecisionListHandler struct {
	specgraphv1connect.UnimplementedDecisionServiceHandler
	decisions []*specv1.Decision
}

func (h fakeDecisionListHandler) ListDecisions(_ context.Context, _ *connect.Request[specv1.ListDecisionsRequest]) (*connect.Response[specv1.ListDecisionsResponse], error) {
	return connect.NewResponse(&specv1.ListDecisionsResponse{
		Decisions: h.decisions,
	}), nil
}

type fakeDecisionListErrorHandler struct {
	specgraphv1connect.UnimplementedDecisionServiceHandler
}

func (fakeDecisionListErrorHandler) ListDecisions(_ context.Context, _ *connect.Request[specv1.ListDecisionsRequest]) (*connect.Response[specv1.ListDecisionsResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, nil)
}

type fakeDecisionShowHandler struct {
	specgraphv1connect.UnimplementedDecisionServiceHandler
}

func (fakeDecisionShowHandler) GetDecision(_ context.Context, req *connect.Request[specv1.GetDecisionRequest]) (*connect.Response[specv1.GetDecisionResponse], error) {
	return connect.NewResponse(&specv1.GetDecisionResponse{
		Decision: &specv1.Decision{
			Id:    "dec-01ABC",
			Slug:  req.Msg.GetSlug(),
			Title: "Use Memgraph",
		},
	}), nil
}

type fakeDecisionShowErrorHandler struct {
	specgraphv1connect.UnimplementedDecisionServiceHandler
}

func (fakeDecisionShowErrorHandler) GetDecision(_ context.Context, _ *connect.Request[specv1.GetDecisionRequest]) (*connect.Response[specv1.GetDecisionResponse], error) {
	return nil, connect.NewError(connect.CodeNotFound, nil)
}

// --- create tests ---

func TestRunDecisionCreate_HappyPath(t *testing.T) {
	startFakeServer[specgraphv1connect.DecisionServiceHandler](t, fakeDecisionCreateHandler{}, specgraphv1connect.NewDecisionServiceHandler)

	oldTitle := decisionTitle
	oldText := decisionText
	oldRationale := decisionRationale
	decisionTitle = "Use Memgraph"
	decisionText = "We will use Memgraph as the graph database."
	decisionRationale = "Best performance for our use case."
	t.Cleanup(func() {
		decisionTitle = oldTitle
		decisionText = oldText
		decisionRationale = oldRationale
	})

	err := runDecisionCreate(newCmdWithCtx(), []string{"use-memgraph"})
	require.NoError(t, err)
}

func TestRunDecisionCreate_RPCError(t *testing.T) {
	startFakeServer[specgraphv1connect.DecisionServiceHandler](t, fakeDecisionCreateErrorHandler{}, specgraphv1connect.NewDecisionServiceHandler)

	oldTitle := decisionTitle
	decisionTitle = "Fail"
	t.Cleanup(func() { decisionTitle = oldTitle })

	err := runDecisionCreate(newCmdWithCtx(), []string{"fail-slug"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create decision")
}

// --- list tests ---

func TestRunDecisionList_HappyPath(t *testing.T) {
	h := fakeDecisionListHandler{
		decisions: []*specv1.Decision{
			{Id: "dec-01", Slug: "use-memgraph", Title: "Use Memgraph"},
			{Id: "dec-02", Slug: "adopt-grpc", Title: "Adopt gRPC"},
		},
	}
	startFakeServer[specgraphv1connect.DecisionServiceHandler](t, h, specgraphv1connect.NewDecisionServiceHandler)

	oldStatus := decisionListStatus
	oldJSON := decisionListJSON
	decisionListStatus = ""
	decisionListJSON = false
	t.Cleanup(func() {
		decisionListStatus = oldStatus
		decisionListJSON = oldJSON
	})

	err := runDecisionList(newCmdWithCtx(), nil)
	require.NoError(t, err)
}

func TestRunDecisionList_HappyPath_JSON(t *testing.T) {
	h := fakeDecisionListHandler{
		decisions: []*specv1.Decision{
			{Id: "dec-01", Slug: "use-memgraph", Title: "Use Memgraph"},
		},
	}
	startFakeServer[specgraphv1connect.DecisionServiceHandler](t, h, specgraphv1connect.NewDecisionServiceHandler)

	oldStatus := decisionListStatus
	oldJSON := decisionListJSON
	decisionListStatus = ""
	decisionListJSON = true
	t.Cleanup(func() {
		decisionListStatus = oldStatus
		decisionListJSON = oldJSON
	})

	cmd := newCmdWithCtx()
	cmd.SetOut(io.Discard)
	err := runDecisionList(cmd, nil)
	require.NoError(t, err)
}

func TestRunDecisionList_EmptyResults(t *testing.T) {
	h := fakeDecisionListHandler{decisions: nil}
	startFakeServer[specgraphv1connect.DecisionServiceHandler](t, h, specgraphv1connect.NewDecisionServiceHandler)

	oldStatus := decisionListStatus
	oldJSON := decisionListJSON
	decisionListStatus = ""
	decisionListJSON = false
	t.Cleanup(func() {
		decisionListStatus = oldStatus
		decisionListJSON = oldJSON
	})

	err := runDecisionList(newCmdWithCtx(), nil)
	require.NoError(t, err)
}

func TestRunDecisionList_RPCError(t *testing.T) {
	startFakeServer[specgraphv1connect.DecisionServiceHandler](t, fakeDecisionListErrorHandler{}, specgraphv1connect.NewDecisionServiceHandler)

	oldStatus := decisionListStatus
	decisionListStatus = ""
	t.Cleanup(func() { decisionListStatus = oldStatus })

	err := runDecisionList(newCmdWithCtx(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list decisions")
}

func TestRunDecisionList_InvalidStatus_WithServer(t *testing.T) {
	// Start a real server so the client is created successfully — the error
	// should come from the status validation, not from client creation.
	h := fakeDecisionListHandler{decisions: nil}
	startFakeServer[specgraphv1connect.DecisionServiceHandler](t, h, specgraphv1connect.NewDecisionServiceHandler)

	oldStatus := decisionListStatus
	decisionListStatus = "bogus"
	t.Cleanup(func() { decisionListStatus = oldStatus })

	err := runDecisionList(newCmdWithCtx(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown status")
}

// --- show tests ---

func TestRunDecisionShow_HappyPath(t *testing.T) {
	startFakeServer[specgraphv1connect.DecisionServiceHandler](t, fakeDecisionShowHandler{}, specgraphv1connect.NewDecisionServiceHandler)

	oldJSON := decisionShowJSON
	decisionShowJSON = false
	t.Cleanup(func() { decisionShowJSON = oldJSON })

	err := runDecisionShow(newCmdWithCtx(), []string{"use-memgraph"})
	require.NoError(t, err)
}

func TestRunDecisionShow_HappyPath_JSON(t *testing.T) {
	startFakeServer[specgraphv1connect.DecisionServiceHandler](t, fakeDecisionShowHandler{}, specgraphv1connect.NewDecisionServiceHandler)

	oldJSON := decisionShowJSON
	decisionShowJSON = true
	t.Cleanup(func() { decisionShowJSON = oldJSON })

	cmd := newCmdWithCtx()
	cmd.SetOut(io.Discard)
	err := runDecisionShow(cmd, []string{"use-memgraph"})
	require.NoError(t, err)
}

func TestRunDecisionShow_RPCError(t *testing.T) {
	startFakeServer[specgraphv1connect.DecisionServiceHandler](t, fakeDecisionShowErrorHandler{}, specgraphv1connect.NewDecisionServiceHandler)

	oldJSON := decisionShowJSON
	decisionShowJSON = false
	t.Cleanup(func() { decisionShowJSON = oldJSON })

	err := runDecisionShow(newCmdWithCtx(), []string{"nonexistent"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get decision")
}

// --- cobra arg validation ---

func TestDecisionCreateCmd_RequiresSlug(t *testing.T) {
	err := decisionCreateCmd.Args(decisionCreateCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestDecisionShowCmd_RequiresSlug(t *testing.T) {
	err := decisionShowCmd.Args(decisionShowCmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}
