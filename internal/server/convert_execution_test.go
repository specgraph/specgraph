// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"testing"
	"time"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/prime"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrimeProjectViewToProto_RoundTripsProvenanceOrder(t *testing.T) {
	v := &prime.ProjectView{
		Constitution: &storage.Constitution{
			ID:    "c-1",
			Layer: storage.ConstitutionLayerProject,
			Name:  "test",
		},
		ConstitutionProvenance: []storage.ProvenanceEntry{
			{Path: "tech.languages.primary", Layer: storage.ConstitutionLayerProject},
			{Path: "principles[0]", Layer: storage.ConstitutionLayerOrg},
		},
	}
	pb, err := primeProjectViewToProto(v)
	require.NoError(t, err)
	require.NotNil(t, pb)
	require.Len(t, pb.ConstitutionProvenance, 2)
	assert.Equal(t, "tech.languages.primary", pb.ConstitutionProvenance[0].Path)
	assert.Equal(t, specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT, pb.ConstitutionProvenance[0].Layer)
	assert.Equal(t, "principles[0]", pb.ConstitutionProvenance[1].Path)
	assert.Equal(t, specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG, pb.ConstitutionProvenance[1].Layer)
}

func TestPrimeProjectViewToProto_NilConstitutionPassesThrough(t *testing.T) {
	v := &prime.ProjectView{
		Constitution:           nil,
		ConstitutionProvenance: nil,
	}
	pb, err := primeProjectViewToProto(v)
	require.NoError(t, err)
	require.NotNil(t, pb)
	assert.Nil(t, pb.Constitution)
	assert.Empty(t, pb.ConstitutionProvenance)
}

func TestPrimeProjectViewToProto_GraphOverviewCountsCast(t *testing.T) {
	v := &prime.ProjectView{
		GraphOverview: prime.GraphOverview{
			CountsByStage: map[string]int{
				"spark":    2,
				"approved": 5,
			},
		},
	}
	pb, err := primeProjectViewToProto(v)
	require.NoError(t, err)
	require.NotNil(t, pb.GraphOverview)
	assert.Equal(t, int32(2), pb.GraphOverview.CountsByStage["spark"])
	assert.Equal(t, int32(5), pb.GraphOverview.CountsByStage["approved"])
}

func TestPrimeProjectViewToProto_FindingsBySeverityKeyCast(t *testing.T) {
	v := &prime.ProjectView{
		FindingsBySeverity: map[storage.FindingSeverity]int{
			storage.SeverityCritical: 3,
			storage.SeverityWarning:  1,
		},
	}
	pb, err := primeProjectViewToProto(v)
	require.NoError(t, err)
	assert.Equal(t,
		int32(3),
		pb.FindingsBySeverity[int32(specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL)],
	)
	assert.Equal(t,
		int32(1),
		pb.FindingsBySeverity[int32(specv1.FindingSeverity_FINDING_SEVERITY_WARNING)],
	)
}

func TestPrimeProjectViewToProto_SkillsCount(t *testing.T) {
	v := &prime.ProjectView{SkillsCount: 7}
	pb, err := primeProjectViewToProto(v)
	require.NoError(t, err)
	assert.Equal(t, int32(7), pb.SkillsCount)
}

func TestPrimeProjectViewToProto_NilInputReturnsNil(t *testing.T) {
	pb, err := primeProjectViewToProto(nil)
	require.NoError(t, err)
	assert.Nil(t, pb)
}

func TestPrimeSpecViewToProto_Basic(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	v := &prime.SpecView{
		Spec: &storage.Spec{
			ID:        "spec-1",
			Slug:      "build-thing",
			Intent:    "build a thing",
			Stage:      storage.SpecStageApproved,
			Provenance: storage.SpecProvenanceAuthored,
			Version:    1,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		Constitution: &storage.Constitution{
			ID:    "c-1",
			Layer: storage.ConstitutionLayerProject,
			Name:  "proj",
		},
		ConstitutionProvenance: []storage.ProvenanceEntry{
			{Path: "principles[0]", Layer: storage.ConstitutionLayerProject},
		},
		Decisions: []*storage.Decision{
			{
				ID:     "dec-1",
				Slug:   "use-x",
				Title:  "Use X",
				Status: storage.DecisionStatusAccepted,
			},
		},
		Slices: []*storage.Slice{
			{
				ID:         "sl-1",
				Slug:       "build-thing/slice-1",
				ParentSlug: "build-thing",
				SliceID:    "slice-1",
				Intent:     "first piece",
				Status:     storage.SliceStatusOpen,
				CreatedAt:  now,
				UpdatedAt:  now,
			},
		},
		Blockers: []*storage.ExecutionEvent{
			{
				ID:        "evt-1",
				SpecSlug:  "build-thing",
				Agent:     "agent-a",
				Type:      storage.ExecutionEventTypeBlocker,
				Message:   "needs review",
				CreatedAt: now,
			},
		},
	}
	pb, err := primeSpecViewToProto(v)
	require.NoError(t, err)
	require.NotNil(t, pb)
	require.NotNil(t, pb.Spec)
	assert.Equal(t, "build-thing", pb.Spec.Slug)
	require.NotNil(t, pb.Constitution)
	assert.Equal(t, "c-1", pb.Constitution.Id)
	require.Len(t, pb.ConstitutionProvenance, 1)
	assert.Equal(t, "principles[0]", pb.ConstitutionProvenance[0].Path)
	require.Len(t, pb.Decisions, 1)
	assert.Equal(t, "use-x", pb.Decisions[0].Slug)
	require.Len(t, pb.Slices, 1)
	assert.Equal(t, "build-thing/slice-1", pb.Slices[0].Slug)
	// Claims intentionally not populated by the converter.
	assert.Empty(t, pb.Claims)
	require.Len(t, pb.Blockers, 1)
	assert.Equal(t, "evt-1", pb.Blockers[0].Id)
	assert.Equal(t, specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_BLOCKER, pb.Blockers[0].Type)
}

func TestPrimeSpecViewToProto_BlockersOnly(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	v := &prime.SpecView{
		Spec: &storage.Spec{
			ID:        "spec-2",
			Slug:      "lonely-spec",
			Intent:    "alone",
			Stage:      storage.SpecStageSpark,
			Provenance: storage.SpecProvenanceAuthored,
			Version:    1,
			CreatedAt:  now,
			UpdatedAt:  now,
		},
		Blockers: []*storage.ExecutionEvent{
			{
				ID:        "evt-2",
				SpecSlug:  "lonely-spec",
				Agent:     "agent-b",
				Type:      storage.ExecutionEventTypeBlocker,
				Message:   "stuck",
				CreatedAt: now,
			},
		},
	}
	pb, err := primeSpecViewToProto(v)
	require.NoError(t, err)
	require.NotNil(t, pb)
	assert.Nil(t, pb.Constitution)
	assert.Empty(t, pb.ConstitutionProvenance)
	assert.Empty(t, pb.Decisions)
	assert.Empty(t, pb.Slices)
	assert.Empty(t, pb.Claims)
	require.Len(t, pb.Blockers, 1)
	assert.Equal(t, "evt-2", pb.Blockers[0].Id)
}

func TestPrimeSpecViewToProto_NilInputReturnsNil(t *testing.T) {
	pb, err := primeSpecViewToProto(nil)
	require.NoError(t, err)
	assert.Nil(t, pb)
}
