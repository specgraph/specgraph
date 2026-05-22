// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package prime

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/specgraph/specgraph/internal/mcp/skills"
	"github.com/specgraph/specgraph/internal/storage"
)

// readyCap is the maximum number of "ready to work" specs surfaced in a
// ProjectView. Matches the cap used by the existing prime resource
// handler in internal/mcp/resources.go.
const readyCap = 10

// Composer assembles ProjectView and SpecView values by reading through a
// Backend. It does not render output; rendering is performed separately by
// callers (e.g. internal/render).
//
// A Composer is safe for concurrent use as long as the underlying Backend
// and skills Source are.
type Composer struct {
	backend Backend
	skills  skills.Source
}

// New returns a Composer that reads through backend and skillsSrc. Both
// arguments must be non-nil; passing nil will panic on first use.
func New(backend Backend, skillsSrc skills.Source) *Composer {
	return &Composer{backend: backend, skills: skillsSrc}
}

// Project composes a ProjectView for the current project.
//
// An absent constitution (storage.ErrConstitutionNotFound) is treated as
// an empty state: Constitution and ConstitutionProvenance are nil and no
// error is returned. Any other backend error is wrapped and returned.
func (c *Composer) Project(ctx context.Context) (*ProjectView, error) {
	view := &ProjectView{
		GraphOverview:      GraphOverview{CountsByStage: map[string]int{}},
		FindingsBySeverity: map[storage.FindingSeverity]int{},
	}

	// Constitution + provenance.
	merged, err := c.backend.GetMergedConstitution(ctx)
	switch {
	case errors.Is(err, storage.ErrConstitutionNotFound):
		// Soft empty state — leave Constitution/Provenance nil.
	case err != nil:
		return nil, fmt.Errorf("prime: get merged constitution: %w", err)
	case merged != nil:
		view.Constitution = merged.Constitution
		view.ConstitutionProvenance = provenanceFromMap(merged.Provenance)
	}

	// Graph overview — bucket by stage.
	specs, err := c.backend.ListSpecs(ctx, "", "", 0)
	if err != nil {
		return nil, fmt.Errorf("prime: list specs: %w", err)
	}
	for _, s := range specs {
		view.GraphOverview.CountsByStage[string(s.Stage)]++
	}

	// Ready specs — capped.
	ready, err := c.backend.GetReady(ctx)
	if err != nil {
		return nil, fmt.Errorf("prime: get ready: %w", err)
	}
	if len(ready) > readyCap {
		ready = ready[:readyCap]
	}
	readySpecs := make([]*storage.Spec, 0, len(ready))
	for _, ref := range ready {
		spec, gerr := c.backend.GetSpec(ctx, ref.Slug)
		if gerr != nil {
			// Skip specs that disappear between GetReady and GetSpec —
			// this is benign in the presence of concurrent writers.
			if errors.Is(gerr, storage.ErrSpecNotFound) {
				continue
			}
			return nil, fmt.Errorf("prime: get ready spec %q: %w", ref.Slug, gerr)
		}
		readySpecs = append(readySpecs, spec)
	}
	view.Ready = readySpecs

	// Findings — bucket by severity. Uses ListAllFindings (project-wide
	// list) — the closest storage method to the RPC ListProjectFindings.
	findings, err := c.backend.ListAllFindings(ctx)
	if err != nil {
		return nil, fmt.Errorf("prime: list all findings: %w", err)
	}
	for _, f := range findings {
		view.FindingsBySeverity[f.Severity]++
	}

	// Skills count.
	metas, err := c.skills.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("prime: list skills: %w", err)
	}
	view.SkillsCount = len(metas)

	return view, nil
}

// Spec composes a SpecView for the named spec.
//
// Returns storage.ErrSpecNotFound (verbatim, via errors.Is-friendly
// wrapping) when the spec does not exist; the handler layer translates
// this to connect.CodeNotFound. As in Project, an absent constitution is
// a soft empty state.
func (c *Composer) Spec(ctx context.Context, slug string) (*SpecView, error) {
	spec, err := c.backend.GetSpec(ctx, slug)
	if err != nil {
		// Wrap with %w so callers can still errors.Is(...,
		// storage.ErrSpecNotFound) while satisfying wrapcheck.
		return nil, fmt.Errorf("prime: get spec %q: %w", slug, err)
	}

	view := &SpecView{Spec: spec}

	merged, err := c.backend.GetMergedConstitution(ctx)
	switch {
	case errors.Is(err, storage.ErrConstitutionNotFound):
		// Soft empty state — leave nil.
	case err != nil:
		return nil, fmt.Errorf("prime: get merged constitution: %w", err)
	case merged != nil:
		view.Constitution = merged.Constitution
		view.ConstitutionProvenance = provenanceFromMap(merged.Provenance)
	}

	// Decisions linked via DECIDED_IN edges (spec -> decision per ADR-003).
	edges, err := c.backend.ListEdges(ctx, slug, storage.EdgeTypeDecidedIn)
	if err != nil {
		return nil, fmt.Errorf("prime: list DECIDED_IN edges for %q: %w", slug, err)
	}
	decisions := make([]*storage.Decision, 0, len(edges))
	for _, e := range edges {
		// ADR-003: DECIDED_IN is spec -> decision; ToID is the decision slug.
		dec, derr := c.backend.GetDecision(ctx, e.ToID)
		if derr != nil {
			if errors.Is(derr, storage.ErrDecisionNotFound) {
				continue
			}
			return nil, fmt.Errorf("prime: get decision %q: %w", e.ToID, derr)
		}
		decisions = append(decisions, dec)
	}
	view.Decisions = decisions

	// Slices.
	slices, err := c.backend.ListSlices(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("prime: list slices for %q: %w", slug, err)
	}
	view.Slices = slices

	// Blocker events. GetExecutionEvents returns all event types; we
	// filter to blockers only here so callers do not have to.
	events, err := c.backend.GetExecutionEvents(ctx, slug, 0)
	if err != nil {
		return nil, fmt.Errorf("prime: get execution events for %q: %w", slug, err)
	}
	blockers := make([]*storage.ExecutionEvent, 0)
	for _, ev := range events {
		if ev.Type == storage.ExecutionEventTypeBlocker {
			blockers = append(blockers, ev)
		}
	}
	view.Blockers = blockers

	return view, nil
}

// provenanceFromMap turns the MergedResult.Provenance map into a
// deterministic slice of ProvenanceEntry. We sort by Path so the order is
// stable across calls — important for rendering and for test assertions.
func provenanceFromMap(m map[string]storage.ConstitutionLayer) []storage.ProvenanceEntry {
	if len(m) == 0 {
		return nil
	}
	entries := make([]storage.ProvenanceEntry, 0, len(m))
	for path, layer := range m {
		entries = append(entries, storage.ProvenanceEntry{Path: path, Layer: layer})
	}
	// Sort lexicographically by Path for determinism.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries
}
