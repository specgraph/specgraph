// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/seanb4t/specgraph/internal/storage"
)

// maxHistoryEntries caps the number of history entries stored per spec.
// When exceeded, the oldest entries are trimmed to prevent unbounded growth.
const maxHistoryEntries = 100

// terminalStages are stages from which no further lifecycle transitions are allowed.
var terminalStages = map[storage.SpecStage]bool{
	storage.SpecStageSuperseded: true,
	storage.SpecStageAbandoned:  true,
}

// marshalHistory serializes a slice of HistoryEntry to a JSON string for storage.
// If len(entries) exceeds maxHistoryEntries, the oldest entries are trimmed.
func marshalHistory(entries []storage.HistoryEntry) (string, error) {
	if len(entries) > maxHistoryEntries {
		entries = entries[len(entries)-maxHistoryEntries:]
	}
	jsonEntries := make([]historyEntryJSON, len(entries))
	for i, e := range entries {
		jsonEntries[i] = historyEntryJSON{
			Version: e.Version,
			Stage:   string(e.Stage),
			Summary: e.Summary,
			Reason:  e.Reason,
			Date:    e.Date.UTC().Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	data, err := json.Marshal(jsonEntries)
	if err != nil {
		return "", fmt.Errorf("memgraph: marshal history: %w", err)
	}
	return string(data), nil
}

// LifecycleAmendSpec transitions a done spec back into an earlier authoring stage,
// appending a history entry. If reEntryStage is empty, the spec is set to "amended".
// Returns ErrSpecNotDone if the spec is not at the "done" stage, and ErrSpecNotFound
// if the spec does not exist.
//
// The transition is atomic: a single Cypher query gates on the expected stage
// and version so concurrent requests cannot overwrite each other.
func (s *Store) LifecycleAmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*storage.Spec, error) {
	// Read current state to build history and compute new version.
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, err
	}
	if terminalStages[spec.Stage] {
		return nil, fmt.Errorf("amend spec %q (stage=%s): %w", slug, spec.Stage, storage.ErrSpecTerminal)
	}
	if spec.Stage != storage.SpecStageDone {
		return nil, fmt.Errorf("amend spec %q (stage=%s): %w", slug, spec.Stage, storage.ErrSpecNotDone)
	}

	// Determine the target stage: use reEntryStage if provided, default to "amended".
	targetStage := storage.SpecStageAmended
	if reEntryStage != "" {
		targetStage = storage.SpecStage(reEntryStage)
	}

	newVersion := spec.Version + 1
	entry := storage.HistoryEntry{
		Version: newVersion,
		Stage:   targetStage,
		Summary: fmt.Sprintf("Amended from done, re-entry stage: %s", targetStage),
		Reason:  reason,
		Date:    time.Now().UTC(),
	}
	history := make([]storage.HistoryEntry, len(spec.History)+1)
	copy(history, spec.History)
	history[len(spec.History)] = entry
	historyJSON, err := marshalHistory(history)
	if err != nil {
		return nil, err
	}

	nowStr := nowRFC3339()
	// Atomic: WHERE guards ensure we only transition if the spec is still at
	// the expected stage and version, preventing TOCTOU races.
	query := `
		MATCH (s:Spec {slug: $slug})
		WHERE s.stage = $expected_stage AND s.version = $expected_version
		SET s.stage = $stage,
		    s.version = $version,
		    s.updated_at = $updated_at,
		    s.history_json = $history_json
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes, s.history_json
	`
	records, err := s.executeQuery(ctx, query, map[string]any{
		"slug":             slug,
		"expected_stage":   string(storage.SpecStageDone),
		"expected_version": spec.Version,
		"stage":            string(targetStage),
		"version":          int64(newVersion),
		"updated_at":       nowStr,
		"history_json":     historyJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("memgraph: amend spec: %w", err)
	}
	if len(records) == 0 {
		// Re-read to determine the specific error.
		return nil, s.amendPreconditionError(ctx, slug)
	}
	return recordToSpec(records[0])
}

// amendPreconditionError re-reads the spec to produce the correct error after
// an atomic amend failed its WHERE guard.
func (s *Store) amendPreconditionError(ctx context.Context, slug string) error {
	current, err := s.GetSpec(ctx, slug)
	if err != nil {
		return err
	}
	if terminalStages[current.Stage] {
		return fmt.Errorf("amend spec %q (stage=%s): %w", slug, current.Stage, storage.ErrSpecTerminal)
	}
	if current.Stage != storage.SpecStageDone {
		return fmt.Errorf("amend spec %q (stage=%s): %w", slug, current.Stage, storage.ErrSpecNotDone)
	}
	return fmt.Errorf("amend spec %q: %w", slug, storage.ErrConcurrentModification)
}

// LifecycleSupersedeSpec marks the old spec as superseded and links it to the new spec via
// a SUPERSEDES edge. Both specs are returned with updated fields. Returns
// ErrSpecNotFound if the old spec doesn't exist, and ErrNewSpecNotFound if the
// new spec doesn't exist.
//
// The transition is atomic: a WHERE clause gates on the old spec not being
// in a terminal stage, preventing TOCTOU races.
func (s *Store) LifecycleSupersedeSpec(ctx context.Context, oldSlug, newSlug string) (oldSpec, newSpec *storage.Spec, retErr error) {
	// Pre-validate: check old spec exists and new spec exists.
	oldCheck, err := s.GetSpec(ctx, oldSlug)
	if err != nil {
		return nil, nil, err
	}
	if terminalStages[oldCheck.Stage] {
		return nil, nil, fmt.Errorf("supersede spec %q (stage=%s): %w", oldSlug, oldCheck.Stage, storage.ErrSpecTerminal)
	}
	if _, newErr := s.GetSpec(ctx, newSlug); newErr != nil {
		if errors.Is(newErr, storage.ErrSpecNotFound) {
			return nil, nil, fmt.Errorf("supersede spec: new spec %q: %w", newSlug, storage.ErrNewSpecNotFound)
		}
		return nil, nil, fmt.Errorf("supersede spec: new spec %q: %w", newSlug, newErr)
	}

	nowStr := nowRFC3339()
	// Atomic: WHERE guards ensure old spec hasn't entered a terminal state
	// since our pre-validation read.
	query := `
		MATCH (old:Spec {slug: $old_slug}), (new:Spec {slug: $new_slug})
		WHERE NOT old.stage IN $terminal_stages
		SET old.stage = $stage,
		    old.superseded_by = $new_slug,
		    old.version = old.version + 1,
		    old.updated_at = $updated_at,
		    new.supersedes = $old_slug,
		    new.updated_at = $updated_at
		MERGE (new)-[:SUPERSEDES]->(old)
		RETURN old.id, old.slug, old.intent, old.stage, old.priority, old.complexity,
		       old.version, old.created_at, old.updated_at,
		       old.lifecycle, old.superseded_by, old.supersedes, old.history_json,
		       new.id, new.slug, new.intent, new.stage, new.priority, new.complexity,
		       new.version, new.created_at, new.updated_at,
		       new.lifecycle, new.superseded_by, new.supersedes, new.history_json
	`
	records, err := s.executeQuery(ctx, query, map[string]any{
		"old_slug":        oldSlug,
		"new_slug":        newSlug,
		"stage":           string(storage.SpecStageSuperseded),
		"terminal_stages": terminalStagesList(),
		"updated_at":      nowStr,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("memgraph: supersede spec: %w", err)
	}
	if len(records) == 0 {
		// Re-read to determine the specific error.
		current, rerr := s.GetSpec(ctx, oldSlug)
		if rerr != nil {
			return nil, nil, rerr
		}
		if terminalStages[current.Stage] {
			return nil, nil, fmt.Errorf("supersede spec %q (stage=%s): %w", oldSlug, current.Stage, storage.ErrSpecTerminal)
		}
		return nil, nil, fmt.Errorf("supersede spec %q: %w", oldSlug, storage.ErrConcurrentModification)
	}

	rec := records[0]
	// Parse old spec from positions 0-12.
	oldSpec, err = recordToSpec(rec)
	if err != nil {
		return nil, nil, fmt.Errorf("memgraph: supersede: parse old spec: %w", err)
	}

	// Parse new spec from positions 13-25 using a shifted record adapter.
	newSpec, err = recordToSpecOffset(rec, 13)
	if err != nil {
		return nil, nil, fmt.Errorf("memgraph: supersede: parse new spec: %w", err)
	}

	return oldSpec, newSpec, nil
}

// LifecycleAbandonSpec transitions a spec to the abandoned terminal state. Returns
// ErrSpecTerminal if the spec is already in a terminal state, and
// ErrSpecNotFound if the spec does not exist.
//
// The transition is atomic: WHERE guards prevent concurrent transitions from
// racing past the terminal-state check.
func (s *Store) LifecycleAbandonSpec(ctx context.Context, slug, reason string) (*storage.Spec, error) {
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, err
	}
	if terminalStages[spec.Stage] {
		return nil, fmt.Errorf("abandon spec %q (stage=%s): %w", slug, spec.Stage, storage.ErrSpecTerminal)
	}

	newVersion := spec.Version + 1
	entry := storage.HistoryEntry{
		Version: newVersion,
		Stage:   storage.SpecStageAbandoned,
		Summary: "Spec abandoned",
		Reason:  reason,
		Date:    time.Now().UTC(),
	}
	history := make([]storage.HistoryEntry, len(spec.History)+1)
	copy(history, spec.History)
	history[len(spec.History)] = entry
	historyJSON, err := marshalHistory(history)
	if err != nil {
		return nil, err
	}

	nowStr := nowRFC3339()
	// Atomic: WHERE guards on stage and version prevent TOCTOU races.
	query := `
		MATCH (s:Spec {slug: $slug})
		WHERE NOT s.stage IN $terminal_stages AND s.version = $expected_version
		SET s.stage = $stage,
		    s.version = $version,
		    s.updated_at = $updated_at,
		    s.history_json = $history_json
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes, s.history_json
	`
	records, err := s.executeQuery(ctx, query, map[string]any{
		"slug":             slug,
		"expected_version": spec.Version,
		"terminal_stages":  terminalStagesList(),
		"stage":            string(storage.SpecStageAbandoned),
		"version":          int64(newVersion),
		"updated_at":       nowStr,
		"history_json":     historyJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("memgraph: abandon spec: %w", err)
	}
	if len(records) == 0 {
		// Re-read to determine the specific error.
		current, rerr := s.GetSpec(ctx, slug)
		if rerr != nil {
			return nil, rerr
		}
		if terminalStages[current.Stage] {
			return nil, fmt.Errorf("abandon spec %q (stage=%s): %w", slug, current.Stage, storage.ErrSpecTerminal)
		}
		return nil, fmt.Errorf("abandon spec %q: %w", slug, storage.ErrConcurrentModification)
	}
	return recordToSpec(records[0])
}

// terminalStagesList returns the terminal stages as a string slice for use in
// Cypher IN clauses.
func terminalStagesList() []string {
	stages := make([]string, 0, len(terminalStages))
	for stage := range terminalStages {
		stages = append(stages, string(stage))
	}
	return stages
}

// LifecycleAcknowledgeDrift sets drift as acknowledged on the spec node and returns a
// DriftReport reflecting the acknowledgment. Returns ErrSpecNotFound if the
// spec does not exist.
func (s *Store) LifecycleAcknowledgeDrift(ctx context.Context, slug, note string) (*storage.DriftReport, error) {
	query := `
		MATCH (s:Spec {slug: $slug})
		SET s.drift_acknowledged = true, s.drift_acknowledge_note = $note
		RETURN s.slug
	`
	records, err := s.executeQuery(ctx, query, map[string]any{"slug": slug, "note": note})
	if err != nil {
		return nil, fmt.Errorf("memgraph: acknowledge drift: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: acknowledge drift %q: %w", slug, storage.ErrSpecNotFound)
	}
	return &storage.DriftReport{
		SpecSlug:        slug,
		Acknowledged:    true,
		AcknowledgeNote: note,
	}, nil
}
