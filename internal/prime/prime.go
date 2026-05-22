// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package prime

import "github.com/specgraph/specgraph/internal/storage"

// GraphOverview summarises the spec graph for the prime response.
type GraphOverview struct {
	// CountsByStage maps SpecStage string values (e.g. "spark", "approved")
	// to the number of specs currently in that stage.
	CountsByStage map[string]int
}

// ProjectView is the composed data for a project-scope prime response.
//
// Constitution is nil when the project has no constitution layers
// configured (Composer treats storage.ErrConstitutionNotFound as a
// soft empty state). ConstitutionProvenance preserves the order
// returned by the backend.
type ProjectView struct {
	Constitution           *storage.Constitution
	ConstitutionProvenance []storage.ProvenanceEntry
	GraphOverview          GraphOverview
	// Ready lists specs whose dependencies are all done, capped at 10.
	Ready []*storage.Spec
	// FindingsBySeverity buckets project findings by their severity value.
	FindingsBySeverity map[storage.FindingSeverity]int
	// SkillsCount is the number of skills exposed by the MCP skills source.
	SkillsCount int
}

// SpecView is the composed data for a single-spec prime response.
//
// Blockers contains only execution events of type
// storage.ExecutionEventTypeBlocker; the composer filters other event
// types out so callers do not have to inspect Type again.
type SpecView struct {
	Spec                   *storage.Spec
	Constitution           *storage.Constitution
	ConstitutionProvenance []storage.ProvenanceEntry
	Decisions              []*storage.Decision
	Slices                 []*storage.Slice
	// Claims carries the currently active lease for this spec.
	// Semantically there is at most one (the lease model is 1:1) and the
	// composer populates either zero or one entry from
	// ClaimBackend.GetActiveClaim. Exposed as a slice to match the proto
	// SpecView.claims repeated field.
	Claims []*storage.Claim
	// Blockers contains only execution events with
	// Type=storage.ExecutionEventTypeBlocker for this spec.
	Blockers []*storage.ExecutionEvent
}
