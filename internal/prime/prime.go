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
	// Blockers contains only execution events with
	// Type=storage.ExecutionEventTypeBlocker for this spec.
	Blockers []*storage.ExecutionEvent
}

// NOTE on Claims: the proto SpecView includes a claims repeated field
// (forward compat), but the storage layer currently exposes no public
// read path for the active claim — only the private fetchActiveClaim
// in internal/storage/postgres. ClaimBackend's public surface is
// ClaimSpec/UnclaimSpec/Heartbeat only. Adding GetActiveClaim is a
// schema/storage change, out of scope for this bead per its acceptance
// criteria. The proto field stays empty; renderers skip the section.
// A follow-up bead can expose the active claim and populate this view.
