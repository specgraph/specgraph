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

// --- flag parsing / validation ---

func TestParseDecisionConfidence(t *testing.T) {
	tests := []struct {
		input   string
		want    specv1.DecisionConfidence
		wantErr bool
	}{
		{"", specv1.DecisionConfidence_DECISION_CONFIDENCE_UNSPECIFIED, false},
		{"high", specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH, false},
		{"medium", specv1.DecisionConfidence_DECISION_CONFIDENCE_MEDIUM, false},
		{"low", specv1.DecisionConfidence_DECISION_CONFIDENCE_LOW, false},
		{"extreme", 0, true},
		{"HIGH", 0, true},
	}
	for _, tt := range tests {
		t.Run("confidence_"+tt.input, func(t *testing.T) {
			got, err := parseDecisionConfidence(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unknown confidence")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseDecisionScope(t *testing.T) {
	tests := []struct {
		input   string
		want    specv1.DecisionScope
		wantErr bool
	}{
		{"", specv1.DecisionScope_DECISION_SCOPE_UNSPECIFIED, false},
		{"project", specv1.DecisionScope_DECISION_SCOPE_PROJECT, false},
		{"team", specv1.DecisionScope_DECISION_SCOPE_TEAM, false},
		{"org", specv1.DecisionScope_DECISION_SCOPE_ORG, false},
		{"global", 0, true},
		{"PROJECT", 0, true},
	}
	for _, tt := range tests {
		t.Run("scope_"+tt.input, func(t *testing.T) {
			got, err := parseDecisionScope(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "unknown scope")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestRunDecisionCreate_RejectedValidation(t *testing.T) {
	startFakeServer[specgraphv1connect.DecisionServiceHandler](t, fakeDecisionCreateHandler{}, specgraphv1connect.NewDecisionServiceHandler)

	tests := []struct {
		name    string
		value   string
		wantErr string
	}{
		{"missing colon", "NoColonHere", `invalid --rejected value "NoColonHere"`},
		{"empty option", ":some reason", `invalid --rejected value ":some reason"`},
		{"empty reason", "option:", `invalid --rejected value "option:"`},
		{"whitespace option", "  :reason", `invalid --rejected value "  :reason"`},
		{"whitespace reason", "option:  ", `invalid --rejected value "option:  "`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldTitle := decisionTitle
			oldRejected := decisionRejected
			oldConfidence := decisionConfidence
			oldScope := decisionScope
			decisionTitle = "Test"
			decisionRejected = []string{tt.value}
			decisionConfidence = ""
			decisionScope = ""
			t.Cleanup(func() {
				decisionTitle = oldTitle
				decisionRejected = oldRejected
				decisionConfidence = oldConfidence
				decisionScope = oldScope
			})

			err := runDecisionCreate(newCmdWithCtx(), []string{"test-slug"})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestRunDecisionCreate_RejectedHappyPath(t *testing.T) {
	h := &capturingDecisionCreateHandler{}
	startFakeServer[specgraphv1connect.DecisionServiceHandler](t, h, specgraphv1connect.NewDecisionServiceHandler)

	oldTitle := decisionTitle
	oldRejected := decisionRejected
	oldConfidence := decisionConfidence
	oldScope := decisionScope
	decisionTitle = "Test"
	decisionRejected = []string{"PostgreSQL:Not graph-native", " Neo4j : Too expensive "}
	decisionConfidence = ""
	decisionScope = ""
	t.Cleanup(func() {
		decisionTitle = oldTitle
		decisionRejected = oldRejected
		decisionConfidence = oldConfidence
		decisionScope = oldScope
	})

	err := runDecisionCreate(newCmdWithCtx(), []string{"test-slug"})
	require.NoError(t, err)
	require.Len(t, h.lastReq.RejectedAlternatives, 2)
	assert.Equal(t, "PostgreSQL", h.lastReq.RejectedAlternatives[0].Option)
	assert.Equal(t, "Not graph-native", h.lastReq.RejectedAlternatives[0].Reason)
	assert.Equal(t, "Neo4j", h.lastReq.RejectedAlternatives[1].Option)
	assert.Equal(t, "Too expensive", h.lastReq.RejectedAlternatives[1].Reason)
}

func TestRunDecisionCreate_TagsParsing(t *testing.T) {
	h := &capturingDecisionCreateHandler{}
	startFakeServer[specgraphv1connect.DecisionServiceHandler](t, h, specgraphv1connect.NewDecisionServiceHandler)

	tests := []struct {
		name     string
		tags     string
		wantTags []string
	}{
		{"simple", "db,infra", []string{"db", "infra"}},
		{"with spaces", " db , infra , ", []string{"db", "infra"}},
		{"empty segments filtered", "db,,infra", []string{"db", "infra"}},
		{"empty string", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldTitle := decisionTitle
			oldTags := decisionTags
			oldConfidence := decisionConfidence
			oldScope := decisionScope
			oldRejected := decisionRejected
			decisionTitle = "Test"
			decisionTags = tt.tags
			decisionConfidence = ""
			decisionScope = ""
			decisionRejected = nil
			t.Cleanup(func() {
				decisionTitle = oldTitle
				decisionTags = oldTags
				decisionConfidence = oldConfidence
				decisionScope = oldScope
				decisionRejected = oldRejected
			})

			err := runDecisionCreate(newCmdWithCtx(), []string{"test-slug"})
			require.NoError(t, err)
			assert.Equal(t, tt.wantTags, h.lastReq.Tags)
		})
	}
}

func TestRunDecisionCreate_AllNewFields(t *testing.T) {
	h := &capturingDecisionCreateHandler{}
	startFakeServer[specgraphv1connect.DecisionServiceHandler](t, h, specgraphv1connect.NewDecisionServiceHandler)

	oldTitle := decisionTitle
	oldQuestion := decisionQuestion
	oldConfidence := decisionConfidence
	oldScope := decisionScope
	oldOriginSpec := decisionOriginSpec
	oldOriginStage := decisionOriginStage
	oldTags := decisionTags
	oldRejected := decisionRejected
	decisionTitle = "Full test"
	decisionQuestion = "Which DB?"
	decisionConfidence = "high"
	decisionScope = "team"
	decisionOriginSpec = "storage-design"
	decisionOriginStage = "specify"
	decisionTags = "db,infra"
	decisionRejected = []string{"Postgres:Not graph"}
	t.Cleanup(func() {
		decisionTitle = oldTitle
		decisionQuestion = oldQuestion
		decisionConfidence = oldConfidence
		decisionScope = oldScope
		decisionOriginSpec = oldOriginSpec
		decisionOriginStage = oldOriginStage
		decisionTags = oldTags
		decisionRejected = oldRejected
	})

	err := runDecisionCreate(newCmdWithCtx(), []string{"full-test"})
	require.NoError(t, err)
	assert.Equal(t, "Which DB?", h.lastReq.Question)
	assert.Equal(t, specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH, h.lastReq.Confidence)
	assert.Equal(t, specv1.DecisionScope_DECISION_SCOPE_TEAM, h.lastReq.Scope)
	assert.Equal(t, "storage-design", h.lastReq.OriginSpec)
	assert.Equal(t, "specify", h.lastReq.OriginStage)
	assert.Equal(t, []string{"db", "infra"}, h.lastReq.Tags)
	require.Len(t, h.lastReq.RejectedAlternatives, 1)
}

func TestRunDecisionCreate_InvalidConfidence(t *testing.T) {
	startFakeServer[specgraphv1connect.DecisionServiceHandler](t, fakeDecisionCreateHandler{}, specgraphv1connect.NewDecisionServiceHandler)

	oldTitle := decisionTitle
	oldConfidence := decisionConfidence
	oldScope := decisionScope
	oldRejected := decisionRejected
	decisionTitle = "Test"
	decisionConfidence = "extreme"
	decisionScope = ""
	decisionRejected = nil
	t.Cleanup(func() {
		decisionTitle = oldTitle
		decisionConfidence = oldConfidence
		decisionScope = oldScope
		decisionRejected = oldRejected
	})

	err := runDecisionCreate(newCmdWithCtx(), []string{"test-slug"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown confidence")
}

func TestRunDecisionCreate_InvalidScope(t *testing.T) {
	startFakeServer[specgraphv1connect.DecisionServiceHandler](t, fakeDecisionCreateHandler{}, specgraphv1connect.NewDecisionServiceHandler)

	oldTitle := decisionTitle
	oldConfidence := decisionConfidence
	oldScope := decisionScope
	oldRejected := decisionRejected
	decisionTitle = "Test"
	decisionConfidence = ""
	decisionScope = "global"
	decisionRejected = nil
	t.Cleanup(func() {
		decisionTitle = oldTitle
		decisionConfidence = oldConfidence
		decisionScope = oldScope
		decisionRejected = oldRejected
	})

	err := runDecisionCreate(newCmdWithCtx(), []string{"test-slug"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown scope")
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

// --- capturing fake handler ---

type capturingDecisionCreateHandler struct {
	specgraphv1connect.UnimplementedDecisionServiceHandler
	lastReq *specv1.CreateDecisionRequest
}

func (h *capturingDecisionCreateHandler) CreateDecision(_ context.Context, req *connect.Request[specv1.CreateDecisionRequest]) (*connect.Response[specv1.CreateDecisionResponse], error) {
	h.lastReq = req.Msg
	return connect.NewResponse(&specv1.CreateDecisionResponse{
		Decision: &specv1.Decision{
			Id:   "dec-captured",
			Slug: req.Msg.GetSlug(),
		},
	}), nil
}
