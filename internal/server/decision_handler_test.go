// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockDecisionBackend struct {
	mu        sync.Mutex
	decisions map[string]*specv1.Decision
	seq       int
}

func newMockDecisionBackend() *mockDecisionBackend {
	return &mockDecisionBackend{decisions: make(map[string]*specv1.Decision)}
}

func (m *mockDecisionBackend) CreateDecision(_ context.Context, slug, title, decision, rationale string) (*specv1.Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	now := timestamppb.Now()
	d := &specv1.Decision{
		Id:        fmt.Sprintf("dec-%05d", m.seq),
		Slug:      slug,
		Title:     title,
		Status:    specv1.DecisionStatus_DECISION_STATUS_PROPOSED,
		Decision:  decision,
		Rationale: rationale,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.decisions[slug] = d
	return d, nil
}

func (m *mockDecisionBackend) GetDecision(_ context.Context, slug string) (*specv1.Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.decisions[slug]
	if !ok {
		return nil, fmt.Errorf("decision %q not found", slug)
	}
	return d, nil
}

func (m *mockDecisionBackend) ListDecisions(_ context.Context, status specv1.DecisionStatus, limit int) ([]*specv1.Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*specv1.Decision
	for _, d := range m.decisions {
		if status != specv1.DecisionStatus_DECISION_STATUS_UNSPECIFIED && d.Status != status {
			continue
		}
		result = append(result, d)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (m *mockDecisionBackend) UpdateDecision(_ context.Context, slug string, title *string, status *specv1.DecisionStatus, decision, rationale, supersededBy *string) (*specv1.Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.decisions[slug]
	if !ok {
		return nil, fmt.Errorf("decision %q not found", slug)
	}
	if title != nil {
		d.Title = *title
	}
	if status != nil {
		d.Status = *status
	}
	if decision != nil {
		d.Decision = *decision
	}
	if rationale != nil {
		d.Rationale = *rationale
	}
	if supersededBy != nil {
		d.SupersededBy = *supersededBy
	}
	d.UpdatedAt = timestamppb.Now()
	return d, nil
}

func setupDecisionServer(t *testing.T) specgraphv1connect.DecisionServiceClient {
	t.Helper()
	mb := newMockDecisionBackend()
	mux := http.NewServeMux()
	server.RegisterDecisionService(mux, mb)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewDecisionServiceClient(http.DefaultClient, srv.URL)
}

func TestDecisionHandler_CreateAndGet(t *testing.T) {
	client := setupDecisionServer(t)
	ctx := context.Background()

	createResp, err := client.CreateDecision(ctx, connect.NewRequest(&specv1.CreateDecisionRequest{
		Slug:      "use-memgraph",
		Title:     "Use Memgraph for storage",
		Decision:  "We will use Memgraph as the primary graph database.",
		Rationale: "Native Cypher support, good performance for our scale.",
	}))
	require.NoError(t, err)
	require.Equal(t, "use-memgraph", createResp.Msg.Slug)
	require.Equal(t, specv1.DecisionStatus_DECISION_STATUS_PROPOSED, createResp.Msg.Status)
	require.NotEmpty(t, createResp.Msg.Id)

	getResp, err := client.GetDecision(ctx, connect.NewRequest(&specv1.GetDecisionRequest{
		Slug: "use-memgraph",
	}))
	require.NoError(t, err)
	require.Equal(t, createResp.Msg.Id, getResp.Msg.Id)

	_, err = client.GetDecision(ctx, connect.NewRequest(&specv1.GetDecisionRequest{
		Slug: "nonexistent",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestDecisionHandler_ListAndUpdate(t *testing.T) {
	client := setupDecisionServer(t)
	ctx := context.Background()

	_, err := client.CreateDecision(ctx, connect.NewRequest(&specv1.CreateDecisionRequest{
		Slug:  "dec-alpha",
		Title: "Alpha decision",
	}))
	require.NoError(t, err)

	_, err = client.CreateDecision(ctx, connect.NewRequest(&specv1.CreateDecisionRequest{
		Slug:  "dec-beta",
		Title: "Beta decision",
	}))
	require.NoError(t, err)

	listResp, err := client.ListDecisions(ctx, connect.NewRequest(&specv1.ListDecisionsRequest{}))
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Decisions, 2)

	newStatus := specv1.DecisionStatus_DECISION_STATUS_ACCEPTED
	updateResp, err := client.UpdateDecision(ctx, connect.NewRequest(&specv1.UpdateDecisionRequest{
		Slug:   "dec-alpha",
		Status: &newStatus,
	}))
	require.NoError(t, err)
	require.Equal(t, specv1.DecisionStatus_DECISION_STATUS_ACCEPTED, updateResp.Msg.Status)
}
