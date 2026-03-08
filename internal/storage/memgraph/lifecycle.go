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
			Stage:   e.Stage,
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

// AmendSpec transitions a done spec back into authoring, appending a history entry
// and setting the stage to "amended". Returns ErrSpecNotDone if the spec is not
// at the "done" stage, and ErrSpecNotFound if the spec does not exist.
func (s *Store) AmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*storage.Spec, error) {
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, err
	}
	if spec.Stage != storage.SpecStageDone {
		return nil, fmt.Errorf("amend spec %q (stage=%s): %w", slug, spec.Stage, storage.ErrSpecNotDone)
	}

	newVersion := spec.Version + 1
	entry := storage.HistoryEntry{
		Version: newVersion,
		Stage:   string(storage.SpecStageAmended),
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

// SupersedeSpec marks the old spec as superseded and links it to the new spec via
// a SUPERSEDES edge. Both specs are returned with updated fields. Returns
// ErrSpecNotFound if the old spec doesn't exist, and ErrNewSpecNotFound if the
// new spec doesn't exist.
func (s *Store) SupersedeSpec(ctx context.Context, oldSlug, newSlug string) (oldSpec, newSpec *storage.Spec, retErr error) {
	if _, err := s.GetSpec(ctx, oldSlug); err != nil {
		return nil, nil, err
	}
	if _, err := s.GetSpec(ctx, newSlug); err != nil {
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

// AbandonSpec transitions a spec to the abandoned terminal state. Returns
// ErrSpecTerminal if the spec is already in a terminal state, and
// ErrSpecNotFound if the spec does not exist.
func (s *Store) AbandonSpec(ctx context.Context, slug, reason string) (*storage.Spec, error) {
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
		Stage:   string(storage.SpecStageAbandoned),
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

// CheckDrift detects dependency drift for a spec by comparing upstream updated_at
// timestamps. If slug is empty and scope is "all", checks all specs (not yet
// implemented — currently requires a slug).
func (s *Store) CheckDrift(ctx context.Context, slug, _ string) ([]storage.DriftReport, error) {
	if slug == "" {
		return nil, nil
	}

	query := `
		MATCH (s:Spec {slug: $slug})-[:DEPENDS_ON]->(upstream:Spec)
		WHERE upstream.updated_at > s.updated_at
		RETURN upstream.slug, upstream.version, s.version
	`
	records, err := s.executeQuery(ctx, query, map[string]any{"slug": slug})
	if err != nil {
		return nil, fmt.Errorf("memgraph: check drift: %w", err)
	}

	if len(records) == 0 {
		return nil, nil
	}

	var items []storage.DriftItem
	for _, rec := range records {
		upstreamSlug, err := recordString(rec, 0, "upstream.slug")
		if err != nil {
			return nil, err
		}
		upstreamVersion, err := recordInt64(rec, 1, "upstream.version")
		if err != nil {
			return nil, err
		}
		specVersion, err := recordInt64(rec, 2, "s.version")
		if err != nil {
			return nil, err
		}
		items = append(items, storage.DriftItem{
			Type:            storage.DriftTypeDependency,
			Severity:        storage.DriftSeverityMedium,
			Description:     fmt.Sprintf("upstream %q updated (v%d) since spec was last updated (v%d)", upstreamSlug, upstreamVersion, specVersion),
			SpecSlug:        slug,
			UpstreamSlug:    upstreamSlug,
			ExpectedVersion: safeInt32(specVersion),
			ActualVersion:   safeInt32(upstreamVersion),
		})
	}

	return []storage.DriftReport{{
		SpecSlug: slug,
		Items:    items,
	}}, nil
}

// AcknowledgeDrift sets drift as acknowledged on the spec node and returns a
// DriftReport reflecting the acknowledgment. Returns ErrSpecNotFound if the
// spec does not exist.
func (s *Store) AcknowledgeDrift(ctx context.Context, slug, note string) (*storage.DriftReport, error) {
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
