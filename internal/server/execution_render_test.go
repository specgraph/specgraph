// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"strings"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestRenderBundleMarkdown_FullBundle(t *testing.T) {
	b := &storage.Bundle{
		Version: 2,
		Spec: &storage.Spec{
			Slug:        "my-feature",
			Intent:      "Add drift detection",
			Stage:       storage.SpecStageApproved,
			Priority:    storage.SpecPriorityP1,
			ContentHash: "abcd1234",
			ShapeOutput: &storage.ShapeOutput{
				ScopeIn:        []string{"detection loop", "CLI reporting"},
				ScopeOut:       []string{"UI dashboard"},
				ChosenApproach: "Event-driven with hash comparison",
				Risks:          []string{"Hash collisions"},
			},
			SpecifyOutput: &storage.SpecifyOutput{
				VerifyCriteria: []storage.VerifyCriterion{
					{Category: "functional", Description: "Drift detected within 5s"},
				},
				Invariants: []string{"Content hash is never empty"},
				Interfaces: []storage.InterfaceSection{
					{Name: "DriftDetector", Body: "func Detect(ctx, slug) ([]Change, error)"},
				},
				Touches: []storage.FileTouch{
					{Path: "internal/drift/detector.go", Purpose: "Core engine", ChangeType: "create"},
				},
			},
			DecomposeOutput: &storage.DecomposeOutput{
				Strategy: storage.StrategyVerticalSlice,
				Slices: []storage.DecomposeSlice{
					{ID: "slice-1", Intent: "Core detection loop", Verify: []string{"detector returns changes"}, Touches: []string{"internal/drift/detector.go"}},
				},
			},
		},
		Decisions: []*storage.Decision{
			{Slug: "adr-007", Title: "Use content hashing", Status: storage.DecisionStatusAccepted, Body: "Compare Murmur3-128 hashes", Rationale: "Timestamp comparison fails"},
		},
		Claim: &storage.Claim{
			Agent:        "agent-1",
			LeaseExpires: time.Date(2026, 3, 26, 15, 0, 0, 0, time.UTC),
		},
		Dependencies: []storage.DependencyInfo{
			{Slug: "auth-middleware", Stage: storage.SpecStageDone, Drifted: false},
			{Slug: "db-schema", Stage: storage.SpecStageInProgress, Drifted: true, Note: "content changed since baseline"},
		},
	}

	result := renderBundleMarkdown(b)

	// Frontmatter
	assert.True(t, strings.HasPrefix(result, "---\n"), "should start with frontmatter")
	assert.Contains(t, result, "version: 2")
	assert.Contains(t, result, "slug: my-feature")
	assert.Contains(t, result, "stage: approved")
	assert.Contains(t, result, "priority: p1")
	assert.Contains(t, result, "content_hash: abcd1234")

	// What to Build
	assert.Contains(t, result, "## What to Build")
	assert.Contains(t, result, "**Intent:** Add drift detection")
	assert.Contains(t, result, "- **In:** detection loop")
	assert.Contains(t, result, "- **Out:** UI dashboard")
	assert.Contains(t, result, "- [ ] functional: Drift detected within 5s")
	assert.Contains(t, result, "- Content hash is never empty")
	assert.Contains(t, result, "**DriftDetector**")
	assert.Contains(t, result, "| `internal/drift/detector.go` | Core engine | create |")

	// Work Slices
	assert.Contains(t, result, "## Work Slices")
	assert.Contains(t, result, "Strategy: `vertical_slice`")
	assert.Contains(t, result, "### Slice 1: Core detection loop")

	// How to Work
	assert.Contains(t, result, "## How to Work")
	assert.Contains(t, result, "specgraph claim my-feature --agent <your-id>")
	assert.Contains(t, result, "specgraph report-progress my-feature")
	assert.Contains(t, result, "**Current claim:** agent-1")

	// Dependencies (with Note column)
	assert.Contains(t, result, "| auth-middleware | done | no |  |")
	assert.Contains(t, result, "| db-schema | in_progress | **yes** | content changed since baseline |")

	// Decisions
	assert.Contains(t, result, "## Decisions")
	assert.Contains(t, result, "### adr-007: Use content hashing")
	assert.Contains(t, result, "**Status:** accepted")
	assert.Contains(t, result, "**Decision:** Compare Murmur3-128 hashes")
	assert.Contains(t, result, "**Rationale:** Timestamp comparison fails")

	// Design Context
	assert.Contains(t, result, "## Design Context")
	assert.Contains(t, result, "**Chosen approach:** Event-driven with hash comparison")
	assert.Contains(t, result, "- Hash collisions")

	// Constitution pointer
	assert.Contains(t, result, "specgraph prime my-feature")
}

func TestRenderBundleMarkdown_Minimal(t *testing.T) {
	b := &storage.Bundle{
		Version: 2,
		Spec: &storage.Spec{
			Slug:        "minimal",
			Intent:      "Do a thing",
			Stage:       storage.SpecStageApproved,
			Priority:    storage.SpecPriorityP2,
			ContentHash: "aaaa",
		},
	}

	result := renderBundleMarkdown(b)

	assert.Contains(t, result, "## What to Build")
	assert.Contains(t, result, "**Intent:** Do a thing")
	assert.NotContains(t, result, "## Work Slices")
	assert.NotContains(t, result, "## Decisions")
	assert.NotContains(t, result, "## Design Context")
	assert.Contains(t, result, "## How to Work")
	assert.Contains(t, result, "Current claim:** unclaimed")
	assert.NotContains(t, result, "### Dependencies")
}

func TestRenderBundleMarkdown_SingleUnitWithSlice(t *testing.T) {
	b := &storage.Bundle{
		Version: 2,
		Spec: &storage.Spec{
			Slug: "single", Intent: "One thing", Stage: storage.SpecStageApproved,
			Priority: storage.SpecPriorityP2, ContentHash: "bbbb",
			DecomposeOutput: &storage.DecomposeOutput{
				Strategy: storage.StrategySingleUnit,
				Slices:   []storage.DecomposeSlice{{ID: "s1", Intent: "The whole thing"}},
			},
		},
	}

	result := renderBundleMarkdown(b)
	assert.Contains(t, result, "## Work Slices")
	assert.Contains(t, result, "Strategy: `single_unit`")
}

func TestRenderBundleMarkdown_DecisionStatusDisplay(t *testing.T) {
	b := &storage.Bundle{
		Version: 2,
		Spec: &storage.Spec{
			Slug: "dec-test", Intent: "test", Stage: storage.SpecStageApproved,
			Priority: storage.SpecPriorityP2, ContentHash: "cccc",
		},
		Decisions: []*storage.Decision{
			{Slug: "d1", Title: "Proposed", Status: storage.DecisionStatusProposed, Body: "body", Rationale: "why"},
			{Slug: "d2", Title: "Superseded", Status: storage.DecisionStatusSuperseded, Body: "body2", Rationale: "why2"},
		},
	}

	result := renderBundleMarkdown(b)
	assert.Contains(t, result, "**Status:** proposed")
	assert.Contains(t, result, "**Status:** superseded")
}

func TestDecisionDisplayStatus_NoPrefixFallback(t *testing.T) {
	result := decisionDisplayStatus("accepted")
	assert.Equal(t, "accepted", result)
}

func TestRenderBundleMarkdown_ErrorFallback(t *testing.T) {
	b := &storage.Bundle{Version: 2, Spec: nil}
	result := renderBundleMarkdown(b)
	assert.Contains(t, result, "Error rendering bundle")
}

func TestRenderBundleMarkdown_ScopeOutOnly(t *testing.T) {
	b := &storage.Bundle{
		Version: 2,
		Spec: &storage.Spec{
			Slug: "scope-test", Intent: "test", Stage: storage.SpecStageApproved,
			Priority: storage.SpecPriorityP2, ContentHash: "dddd",
			ShapeOutput: &storage.ShapeOutput{
				ScopeOut: []string{"not this"},
			},
		},
	}
	result := renderBundleMarkdown(b)
	assert.Contains(t, result, "### Scope")
	assert.Contains(t, result, "- **Out:** not this")
	assert.NotContains(t, result, "**In scope:**")
}

func TestRenderBundleMarkdown_SliceWithDependsOn(t *testing.T) {
	b := &storage.Bundle{
		Version: 2,
		Spec: &storage.Spec{
			Slug: "dep-slice", Intent: "test", Stage: storage.SpecStageApproved,
			Priority: storage.SpecPriorityP2, ContentHash: "eeee",
			DecomposeOutput: &storage.DecomposeOutput{
				Strategy: storage.StrategyVerticalSlice,
				Slices: []storage.DecomposeSlice{
					{ID: "s1", Intent: "First"},
					{ID: "s2", Intent: "Second", DependsOn: []string{"s1"}},
				},
			},
		},
	}
	result := renderBundleMarkdown(b)
	assert.Contains(t, result, "### Slice 1: First")
	assert.Contains(t, result, "### Slice 2: Second")
	assert.Contains(t, result, "**Depends on:** s1")
}
