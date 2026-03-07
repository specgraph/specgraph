// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"testing"
	"time"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpecToProto(t *testing.T) {
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	spec := &storage.Spec{
		ID: "spec-abc", Slug: "login", Intent: "Login API",
		Stage: "spark", Priority: "p1", Complexity: "medium",
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	pb := specToProto(spec)
	assert.Equal(t, "spec-abc", pb.Id)
	assert.Equal(t, "login", pb.Slug)
	assert.Equal(t, "Login API", pb.Intent)
	assert.Equal(t, "spark", pb.Stage)
	assert.Equal(t, "p1", pb.Priority)
	assert.Equal(t, "medium", pb.Complexity)
	assert.Equal(t, int32(1), pb.Version)
	require.NotNil(t, pb.CreatedAt)
	assert.Equal(t, now.Unix(), pb.CreatedAt.AsTime().Unix())
}

func TestSpecsToProto(t *testing.T) {
	specs := []*storage.Spec{
		{ID: "a", Slug: "a"},
		{ID: "b", Slug: "b"},
	}
	pbs := specsToProto(specs)
	assert.Len(t, pbs, 2)
	assert.Equal(t, "a", pbs[0].Id)
}

func TestDecisionToProto(t *testing.T) {
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	d := &storage.Decision{
		ID: "dec-abc", Slug: "use-memgraph", Title: "Use Memgraph",
		Status: storage.DecisionStatusAccepted, Body: "We chose Memgraph",
		Rationale: "Graph-native", CreatedAt: now, UpdatedAt: now,
	}
	pb, err := decisionToProto(d)
	require.NoError(t, err)
	assert.Equal(t, "dec-abc", pb.Id)
	assert.Equal(t, "use-memgraph", pb.Slug)
	assert.Equal(t, specv1.DecisionStatus_DECISION_STATUS_ACCEPTED, pb.Status)
}

func TestDecisionStatusToProto(t *testing.T) {
	v, err := decisionStatusToProto(storage.DecisionStatusProposed)
	require.NoError(t, err)
	assert.Equal(t, specv1.DecisionStatus_DECISION_STATUS_PROPOSED, v)

	v, err = decisionStatusToProto(storage.DecisionStatusAccepted)
	require.NoError(t, err)
	assert.Equal(t, specv1.DecisionStatus_DECISION_STATUS_ACCEPTED, v)

	v, err = decisionStatusToProto(storage.DecisionStatusSuperseded)
	require.NoError(t, err)
	assert.Equal(t, specv1.DecisionStatus_DECISION_STATUS_SUPERSEDED, v)

	v, err = decisionStatusToProto(storage.DecisionStatusDeprecated)
	require.NoError(t, err)
	assert.Equal(t, specv1.DecisionStatus_DECISION_STATUS_DEPRECATED, v)

	_, err = decisionStatusToProto("unknown")
	assert.Error(t, err)
}

func TestDecisionStatusFromProto(t *testing.T) {
	got, err := decisionStatusFromProto(specv1.DecisionStatus_DECISION_STATUS_PROPOSED)
	require.NoError(t, err)
	assert.Equal(t, storage.DecisionStatusProposed, got)

	got, err = decisionStatusFromProto(specv1.DecisionStatus_DECISION_STATUS_ACCEPTED)
	require.NoError(t, err)
	assert.Equal(t, storage.DecisionStatusAccepted, got)

	got, err = decisionStatusFromProto(specv1.DecisionStatus_DECISION_STATUS_SUPERSEDED)
	require.NoError(t, err)
	assert.Equal(t, storage.DecisionStatusSuperseded, got)

	got, err = decisionStatusFromProto(specv1.DecisionStatus_DECISION_STATUS_DEPRECATED)
	require.NoError(t, err)
	assert.Equal(t, storage.DecisionStatusDeprecated, got)

	_, err = decisionStatusFromProto(specv1.DecisionStatus_DECISION_STATUS_UNSPECIFIED)
	assert.Error(t, err)
}

func TestEdgeToProto(t *testing.T) {
	e := &storage.Edge{FromID: "a", ToID: "b", EdgeType: storage.EdgeTypeDependsOn}
	pb, err := edgeToProto(e)
	require.NoError(t, err)
	assert.Equal(t, "a", pb.FromId)
	assert.Equal(t, "b", pb.ToId)
	assert.Equal(t, specv1.EdgeType_EDGE_TYPE_DEPENDS_ON, pb.EdgeType)
}

func TestEdgeTypeFromProto(t *testing.T) {
	tests := []struct {
		proto  specv1.EdgeType
		domain storage.EdgeType
	}{
		{specv1.EdgeType_EDGE_TYPE_DEPENDS_ON, storage.EdgeTypeDependsOn},
		{specv1.EdgeType_EDGE_TYPE_BLOCKS, storage.EdgeTypeBlocks},
		{specv1.EdgeType_EDGE_TYPE_COMPOSES, storage.EdgeTypeComposes},
		{specv1.EdgeType_EDGE_TYPE_RELATES_TO, storage.EdgeTypeRelatesTo},
		{specv1.EdgeType_EDGE_TYPE_INFORMS, storage.EdgeTypeInforms},
		{specv1.EdgeType_EDGE_TYPE_DECIDED_IN, storage.EdgeTypeDecidedIn},
	}
	for _, tt := range tests {
		got, err := edgeTypeFromProto(tt.proto)
		require.NoError(t, err)
		assert.Equal(t, tt.domain, got)
	}

	_, err := edgeTypeFromProto(specv1.EdgeType_EDGE_TYPE_UNSPECIFIED)
	assert.Error(t, err)
}

func TestEdgeTypeToProto(t *testing.T) {
	tests := []struct {
		domain storage.EdgeType
		proto  specv1.EdgeType
	}{
		{storage.EdgeTypeDependsOn, specv1.EdgeType_EDGE_TYPE_DEPENDS_ON},
		{storage.EdgeTypeBlocks, specv1.EdgeType_EDGE_TYPE_BLOCKS},
		{storage.EdgeTypeComposes, specv1.EdgeType_EDGE_TYPE_COMPOSES},
		{storage.EdgeTypeRelatesTo, specv1.EdgeType_EDGE_TYPE_RELATES_TO},
		{storage.EdgeTypeInforms, specv1.EdgeType_EDGE_TYPE_INFORMS},
		{storage.EdgeTypeDecidedIn, specv1.EdgeType_EDGE_TYPE_DECIDED_IN},
	}
	for _, tt := range tests {
		got, err := edgeTypeToProto(tt.domain)
		require.NoError(t, err)
		assert.Equal(t, tt.proto, got)
	}

	_, err := edgeTypeToProto("unknown")
	assert.Error(t, err)
}
