// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

type mockDecisionBackend struct {
	stubBackend
	mu        sync.Mutex
	decisions map[string]*storage.Decision
	seq       int
}

func newMockDecisionBackend() *mockDecisionBackend {
	return &mockDecisionBackend{decisions: make(map[string]*storage.Decision)}
}

func (m *mockDecisionBackend) CreateDecision(_ context.Context, slug, title, decision, rationale, question string,
	rejectedAlts []storage.RejectedAlternative, confidence storage.DecisionConfidence,
	tags []string, scope storage.DecisionScope, originSpec, originStage string) (*storage.Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	now := time.Now().UTC()
	d := &storage.Decision{
		ID:                   fmt.Sprintf("dec-%05d", m.seq),
		Slug:                 slug,
		Title:                title,
		Status:               storage.DecisionStatusProposed,
		Body:                 decision,
		Rationale:            rationale,
		Question:             question,
		RejectedAlternatives: rejectedAlts,
		Confidence:           confidence,
		Tags:                 tags,
		Scope:                scope,
		OriginSpec:           originSpec,
		OriginStage:          originStage,
		ContentHash:          strings.Repeat("a", 32),
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	m.decisions[slug] = d
	return d, nil
}

func (m *mockDecisionBackend) GetDecision(_ context.Context, slug string) (*storage.Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.decisions[slug]
	if !ok {
		return nil, storage.ErrDecisionNotFound
	}
	return d, nil
}

func (m *mockDecisionBackend) ListDecisions(_ context.Context, status storage.DecisionStatus, limit int) ([]*storage.Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*storage.Decision
	for _, d := range m.decisions {
		if status != "" && d.Status != status {
			continue
		}
		result = append(result, d)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (m *mockDecisionBackend) UpdateDecision(_ context.Context, slug string, title *string, status *storage.DecisionStatus,
	decision, rationale, supersededBy, question *string,
	rejectedAlts *[]storage.RejectedAlternative, confidence *storage.DecisionConfidence,
	tags *[]string, scope *storage.DecisionScope, originSpec, originStage *string) (*storage.Decision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.decisions[slug]
	if !ok {
		return nil, storage.ErrDecisionNotFound
	}
	if title != nil {
		d.Title = *title
	}
	if status != nil {
		d.Status = *status
	}
	if decision != nil {
		d.Body = *decision
	}
	if rationale != nil {
		d.Rationale = *rationale
	}
	if supersededBy != nil {
		d.SupersededBy = *supersededBy
	}
	if question != nil {
		d.Question = *question
	}
	if rejectedAlts != nil {
		d.RejectedAlternatives = *rejectedAlts
	}
	if confidence != nil {
		d.Confidence = *confidence
	}
	if tags != nil {
		d.Tags = *tags
	}
	if scope != nil {
		d.Scope = *scope
	}
	if originSpec != nil {
		d.OriginSpec = *originSpec
	}
	if originStage != nil {
		d.OriginStage = *originStage
	}
	d.UpdatedAt = time.Now().UTC()
	return d, nil
}

func setupDecisionServer(t *testing.T) specgraphv1connect.DecisionServiceClient {
	t.Helper()
	mb := newMockDecisionBackend()
	scoper := &testScoper{backend: mb}
	mux := http.NewServeMux()
	server.RegisterDecisionService(mux, scoper)
	srv := httptest.NewServer(wrapTestProject(mux))
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
	require.Equal(t, "use-memgraph", createResp.Msg.GetDecision().GetSlug())
	require.Equal(t, specv1.DecisionStatus_DECISION_STATUS_PROPOSED, createResp.Msg.GetDecision().GetStatus())
	require.NotEmpty(t, createResp.Msg.GetDecision().GetId())

	getResp, err := client.GetDecision(ctx, connect.NewRequest(&specv1.GetDecisionRequest{
		Slug: "use-memgraph",
	}))
	require.NoError(t, err)
	require.Equal(t, createResp.Msg.GetDecision().GetId(), getResp.Msg.GetDecision().GetId())

	_, err = client.GetDecision(ctx, connect.NewRequest(&specv1.GetDecisionRequest{
		Slug: "nonexistent",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestDecisionHandler_SlugValidation(t *testing.T) {
	client := setupDecisionServer(t)
	ctx := context.Background()

	tests := []struct {
		name string
		slug string
	}{
		{"empty slug", ""},
		{"path traversal", "../admin"},
		{"uppercase", "Use-PostgreSQL"},
		{"too long", string(make([]byte, 257))},
	}
	for _, tt := range tests {
		t.Run("create_"+tt.name, func(t *testing.T) {
			_, err := client.CreateDecision(ctx, connect.NewRequest(&specv1.CreateDecisionRequest{
				Slug:  tt.slug,
				Title: "test",
			}))
			require.Error(t, err)
			require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
		})
		t.Run("update_"+tt.name, func(t *testing.T) {
			_, err := client.UpdateDecision(ctx, connect.NewRequest(&specv1.UpdateDecisionRequest{
				Slug: tt.slug,
			}))
			require.Error(t, err)
			require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
		})
	}
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
	require.Equal(t, specv1.DecisionStatus_DECISION_STATUS_ACCEPTED, updateResp.Msg.GetDecision().GetStatus())
}

func TestDecisionHandler_CreateWithAllNewFields(t *testing.T) {
	client := setupDecisionServer(t)
	ctx := context.Background()

	resp, err := client.CreateDecision(ctx, connect.NewRequest(&specv1.CreateDecisionRequest{
		Slug:       "full-decision",
		Title:      "Use Memgraph",
		Decision:   "We will use Memgraph.",
		Rationale:  "Graph-native.",
		Question:   "Which database?",
		Confidence: specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH,
		Tags:       []string{"database", "infrastructure"},
		Scope:      specv1.DecisionScope_DECISION_SCOPE_TEAM,
		OriginSpec: "storage-design",
		OriginStage: "specify",
		RejectedAlternatives: []*specv1.RejectedAlternative{
			{Option: "PostgreSQL", Reason: "Not graph-native"},
			{Option: "Neo4j", Reason: "Too expensive"},
		},
	}))
	require.NoError(t, err)
	d := resp.Msg.GetDecision()
	require.Equal(t, "full-decision", d.GetSlug())
	require.Equal(t, "Which database?", d.GetQuestion())
	require.Equal(t, specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH, d.GetConfidence())
	require.Equal(t, []string{"database", "infrastructure"}, d.GetTags())
	require.Equal(t, specv1.DecisionScope_DECISION_SCOPE_TEAM, d.GetScope())
	require.Equal(t, "storage-design", d.GetOriginSpec())
	require.Equal(t, "specify", d.GetOriginStage())
	require.Len(t, d.GetRejectedAlternatives(), 2)
	require.Equal(t, "PostgreSQL", d.GetRejectedAlternatives()[0].GetOption())
	require.Equal(t, "Neo4j", d.GetRejectedAlternatives()[1].GetOption())
}

func TestDecisionHandler_UpdateWithNewFields(t *testing.T) {
	client := setupDecisionServer(t)
	ctx := context.Background()

	// Create a minimal decision first.
	_, err := client.CreateDecision(ctx, connect.NewRequest(&specv1.CreateDecisionRequest{
		Slug:  "update-target",
		Title: "Minimal",
	}))
	require.NoError(t, err)

	// Update with all new fields.
	q := "Updated question?"
	conf := specv1.DecisionConfidence_DECISION_CONFIDENCE_LOW
	sc := specv1.DecisionScope_DECISION_SCOPE_ORG
	originSpec := "origin-spec-slug"
	originStage := "shape"
	resp, err := client.UpdateDecision(ctx, connect.NewRequest(&specv1.UpdateDecisionRequest{
		Slug:        "update-target",
		Question:    &q,
		Confidence:  &conf,
		Scope:       &sc,
		Tags:        []string{"new-tag"},
		OriginSpec:  &originSpec,
		OriginStage: &originStage,
		RejectedAlternatives: []*specv1.RejectedAlternative{
			{Option: "Option A", Reason: "Too slow"},
		},
	}))
	require.NoError(t, err)
	d := resp.Msg.GetDecision()
	require.Equal(t, "Updated question?", d.GetQuestion())
	require.Equal(t, specv1.DecisionConfidence_DECISION_CONFIDENCE_LOW, d.GetConfidence())
	require.Equal(t, specv1.DecisionScope_DECISION_SCOPE_ORG, d.GetScope())
	require.Equal(t, []string{"new-tag"}, d.GetTags())
	require.Equal(t, "origin-spec-slug", d.GetOriginSpec())
	require.Equal(t, "shape", d.GetOriginStage())
	require.Len(t, d.GetRejectedAlternatives(), 1)
}

func TestDecisionHandler_UpdateWithNoNewFields(t *testing.T) {
	client := setupDecisionServer(t)
	ctx := context.Background()

	// Create with some fields.
	_, err := client.CreateDecision(ctx, connect.NewRequest(&specv1.CreateDecisionRequest{
		Slug:       "no-change-target",
		Title:      "Existing",
		Question:   "Original Q?",
		Confidence: specv1.DecisionConfidence_DECISION_CONFIDENCE_MEDIUM,
		Scope:      specv1.DecisionScope_DECISION_SCOPE_PROJECT,
		Tags:       []string{"keep"},
	}))
	require.NoError(t, err)

	// Update with none of the new fields set — they should be unchanged.
	newTitle := "Updated title"
	resp, err := client.UpdateDecision(ctx, connect.NewRequest(&specv1.UpdateDecisionRequest{
		Slug:  "no-change-target",
		Title: &newTitle,
	}))
	require.NoError(t, err)
	d := resp.Msg.GetDecision()
	require.Equal(t, "Updated title", d.GetTitle())
	// New fields should be preserved from create.
	require.Equal(t, "Original Q?", d.GetQuestion())
	require.Equal(t, specv1.DecisionConfidence_DECISION_CONFIDENCE_MEDIUM, d.GetConfidence())
	require.Equal(t, specv1.DecisionScope_DECISION_SCOPE_PROJECT, d.GetScope())
	require.Equal(t, []string{"keep"}, d.GetTags())
}
