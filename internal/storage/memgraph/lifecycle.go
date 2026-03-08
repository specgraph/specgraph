// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/seanb4t/specgraph/internal/storage"
)

// terminalStages are stages from which no further lifecycle transitions are allowed.
var terminalStages = map[storage.SpecStage]bool{
	storage.SpecStageSuperseded: true,
	storage.SpecStageAbandoned:  true,
}

// marshalHistory serializes a slice of HistoryEntry to a JSON string for storage.
func marshalHistory(entries []storage.HistoryEntry) (string, error) {
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

// LifecycleAmendSpec transitions a done spec back into authoring, appending a history entry
// and setting the stage to "amended". Returns ErrSpecNotDone if the spec is not
// at the "done" stage, and ErrSpecNotFound if the spec does not exist.
func (s *Store) LifecycleAmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*storage.Spec, error) {
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

	newVersion := spec.Version + 1
	entry := storage.HistoryEntry{
		Version: newVersion,
		Stage:   storage.SpecStageAmended,
		Summary: fmt.Sprintf("Amended from done, re-entry stage: %s", reEntryStage),
		Reason:  reason,
		Date:    parseNowUTC(),
	}
	history := make([]storage.HistoryEntry, len(spec.History)+1)
	copy(history, spec.History)
	history[len(spec.History)] = entry
	historyJSON, err := marshalHistory(history)
	if err != nil {
		return nil, err
	}

	nowStr := nowRFC3339()
	query := `
		MATCH (s:Spec {slug: $slug})
		SET s.stage = $stage,
		    s.version = $version,
		    s.updated_at = $updated_at,
		    s.history_json = $history_json
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes, s.history_json
	`
	records, err := s.executeQuery(ctx, query, map[string]any{
		"slug":         slug,
		"stage":        string(storage.SpecStageAmended),
		"version":      int64(newVersion),
		"updated_at":   nowStr,
		"history_json": historyJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("memgraph: amend spec: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: amend spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	return recordToSpec(records[0])
}

// LifecycleSupersedeSpec marks the old spec as superseded and links it to the new spec via
// a SUPERSEDES edge. Both specs are returned with updated fields. Returns
// ErrSpecNotFound if the old spec doesn't exist, and ErrNewSpecNotFound if the
// new spec doesn't exist.
func (s *Store) LifecycleSupersedeSpec(ctx context.Context, oldSlug, newSlug string) (oldSpec, newSpec *storage.Spec, retErr error) {
	oldCheck, err := s.GetSpec(ctx, oldSlug)
	if err != nil {
		return nil, nil, err
	}
	if terminalStages[oldCheck.Stage] {
		return nil, nil, fmt.Errorf("supersede spec %q (stage=%s): %w", oldSlug, oldCheck.Stage, storage.ErrSpecTerminal)
	}
	if _, newErr := s.GetSpec(ctx, newSlug); newErr != nil {
		return nil, nil, fmt.Errorf("supersede spec: new spec %q: %w", newSlug, storage.ErrNewSpecNotFound)
	}

	nowStr := nowRFC3339()
	query := `
		MATCH (old:Spec {slug: $old_slug}), (new:Spec {slug: $new_slug})
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
		"old_slug":   oldSlug,
		"new_slug":   newSlug,
		"stage":      string(storage.SpecStageSuperseded),
		"updated_at": nowStr,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("memgraph: supersede spec: %w", err)
	}
	if len(records) == 0 {
		return nil, nil, fmt.Errorf("memgraph: supersede spec %q: %w", oldSlug, storage.ErrSpecNotFound)
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
		Date:    parseNowUTC(),
	}
	history := make([]storage.HistoryEntry, len(spec.History)+1)
	copy(history, spec.History)
	history[len(spec.History)] = entry
	historyJSON, err := marshalHistory(history)
	if err != nil {
		return nil, err
	}

	nowStr := nowRFC3339()
	query := `
		MATCH (s:Spec {slug: $slug})
		SET s.stage = $stage,
		    s.version = $version,
		    s.updated_at = $updated_at,
		    s.history_json = $history_json
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes, s.history_json
	`
	records, err := s.executeQuery(ctx, query, map[string]any{
		"slug":         slug,
		"stage":        string(storage.SpecStageAbandoned),
		"version":      int64(newVersion),
		"updated_at":   nowStr,
		"history_json": historyJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("memgraph: abandon spec: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: abandon spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	return recordToSpec(records[0])
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

// parseNowUTC returns the current UTC time.
func parseNowUTC() time.Time {
	return time.Now().UTC()
}
