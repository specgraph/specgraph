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

// terminalStages maps stages from which no further lifecycle transitions
// are allowed. Derived from storage.FullyTerminalStages() to maintain a
// single source of truth in spec_domain.go. Computed once at init time.
var terminalStages = func() map[storage.SpecStage]bool {
	m := make(map[storage.SpecStage]bool)
	for _, s := range storage.FullyTerminalStages() {
		m[s] = true
	}
	return m
}()

// appendHistory appends entry to existing history and marshals the result to JSON.
// The combined slice is passed directly to marshalHistory which handles trimming.
func appendHistory(existing []storage.HistoryEntry, entry *storage.HistoryEntry) (string, error) {
	combined := make([]storage.HistoryEntry, len(existing)+1)
	copy(combined, existing)
	combined[len(existing)] = *entry
	return marshalHistory(combined)
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
			Date:    e.Date.UTC().Format(sortableRFC3339Nano),
		}
	}
	data, err := json.Marshal(jsonEntries)
	if err != nil {
		return "", fmt.Errorf("memgraph: marshal history: %w", err)
	}
	return string(data), nil
}

// preconditionError re-reads the spec after an atomic WHERE guard failed
// and returns the appropriate sentinel error. The op parameter names the
// operation (e.g. "amend spec", "supersede spec") for error messages.
// extraChecks, if non-nil, runs after the terminal-stage check passes,
// allowing callers to add operation-specific precondition validation.
func (s *Store) preconditionError(ctx context.Context, slug, op string, extraChecks func(*storage.Spec) error) error {
	current, err := s.GetSpec(ctx, slug)
	if err != nil {
		return fmt.Errorf("%s %q: atomic guard failed and precondition re-read also failed: %w", op, slug, err)
	}
	if terminalStages[current.Stage] {
		return fmt.Errorf("%s %q (stage=%s): %w", op, slug, current.Stage, storage.ErrSpecTerminal)
	}
	if extraChecks != nil {
		if err := extraChecks(current); err != nil {
			return err
		}
	}
	return fmt.Errorf("%s %q: %w", op, slug, storage.ErrConcurrentModification)
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
		if targetStage.ExcludesReEntry() {
			return nil, fmt.Errorf("amend spec %q: re_entry_stage %q: %w", slug, reEntryStage, storage.ErrInvalidReEntryStage)
		}
	}

	newVersion := spec.Version + 1
	entry := storage.HistoryEntry{
		Version: newVersion,
		Stage:   targetStage,
		Summary: fmt.Sprintf("Amended from done, re-entry stage: %s", targetStage),
		Reason:  reason,
		Date:    time.Now().UTC(),
	}
	historyJSON, err := appendHistory(spec.History, &entry)
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
		       s.lifecycle, s.superseded_by, s.supersedes, s.history_json,
		       s.drift_acknowledged, s.drift_acknowledge_note
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
		return nil, s.preconditionError(ctx, slug, "amend spec", func(current *storage.Spec) error {
			// A version mismatch means another operation modified the spec
			// between our pre-read and the atomic query — report concurrent
			// modification rather than the misleading ErrSpecNotDone.
			if current.Version != spec.Version {
				return fmt.Errorf("amend spec %q: %w", slug, storage.ErrConcurrentModification)
			}
			if current.Stage != storage.SpecStageDone {
				return fmt.Errorf("amend spec %q (stage=%s): %w", slug, current.Stage, storage.ErrSpecNotDone)
			}
			return nil
		})
	}
	return recordToSpec(records[0])
}

// LifecycleSupersedeSpec marks the old spec as superseded and links it to the new spec via
// a SUPERSEDES edge. Both specs are returned with updated fields. Returns
// ErrSpecNotFound if the old spec doesn't exist, and ErrNewSpecNotFound if the
// new spec doesn't exist.
//
// The transition is atomic: a WHERE clause gates on the old spec not being
// in a terminal stage, preventing TOCTOU races.
func (s *Store) LifecycleSupersedeSpec(ctx context.Context, oldSlug, newSlug string) (oldSpec, newSpec *storage.Spec, err error) {
	if oldSlug == newSlug {
		return nil, nil, fmt.Errorf("supersede spec: old and new slugs must differ (%q)", oldSlug)
	}
	// Pre-validate: check old spec exists and new spec exists.
	oldCheck, err := s.GetSpec(ctx, oldSlug)
	if err != nil {
		return nil, nil, err
	}
	if terminalStages[oldCheck.Stage] {
		return nil, nil, fmt.Errorf("supersede spec %q (stage=%s): %w", oldSlug, oldCheck.Stage, storage.ErrSpecTerminal)
	}
	newCheck, newErr := s.GetSpec(ctx, newSlug)
	if newErr != nil {
		if errors.Is(newErr, storage.ErrSpecNotFound) {
			return nil, nil, fmt.Errorf("supersede spec: new spec %q: %w", newSlug, storage.ErrNewSpecNotFound)
		}
		return nil, nil, fmt.Errorf("supersede spec: new spec %q: %w", newSlug, newErr)
	}

	now := time.Now().UTC()
	oldVersion := oldCheck.Version + 1
	oldEntry := storage.HistoryEntry{
		Version: oldVersion,
		Stage:   storage.SpecStageSuperseded,
		Summary: "Spec superseded",
		Reason:  fmt.Sprintf("Superseded by %s", newSlug),
		Date:    now,
	}
	historyJSON, err := appendHistory(oldCheck.History, &oldEntry)
	if err != nil {
		return nil, nil, err
	}

	newVersion := newCheck.Version + 1
	newEntry := storage.HistoryEntry{
		Version: newVersion,
		Stage:   newCheck.Stage,
		Summary: "Supersedes predecessor",
		Reason:  fmt.Sprintf("Supersedes %s", oldSlug),
		Date:    now,
	}
	newHistoryJSON, err := appendHistory(newCheck.History, &newEntry)
	if err != nil {
		return nil, nil, err
	}

	nowStr := nowRFC3339()
	// Atomic: WHERE guards ensure old spec hasn't entered a terminal state
	// since our pre-validation read.
	query := `
		MATCH (old:Spec {slug: $old_slug}), (new:Spec {slug: $new_slug})
		WHERE NOT old.stage IN $terminal_stages
		      AND old.version = $expected_version
		      AND new.version = $expected_new_version
		SET old.stage = $stage,
		    old.superseded_by = $new_slug,
		    old.version = $version,
		    old.updated_at = $updated_at,
		    old.history_json = $history_json,
		    new.supersedes = $old_slug,
		    new.version = $new_version,
		    new.updated_at = $updated_at,
		    new.history_json = $new_history_json
		MERGE (new)-[:SUPERSEDES]->(old)
		RETURN old.id, old.slug, old.intent, old.stage, old.priority, old.complexity,
		       old.version, old.created_at, old.updated_at,
		       old.lifecycle, old.superseded_by, old.supersedes, old.history_json,
		       old.drift_acknowledged, old.drift_acknowledge_note,
		       new.id, new.slug, new.intent, new.stage, new.priority, new.complexity,
		       new.version, new.created_at, new.updated_at,
		       new.lifecycle, new.superseded_by, new.supersedes, new.history_json,
		       new.drift_acknowledged, new.drift_acknowledge_note
	`
	records, err := s.executeQuery(ctx, query, map[string]any{
		"old_slug":             oldSlug,
		"new_slug":             newSlug,
		"stage":                string(storage.SpecStageSuperseded),
		"terminal_stages":      terminalStageStrings,
		"expected_version":     oldCheck.Version,
		"expected_new_version": newCheck.Version,
		"version":              int64(oldVersion),
		"new_version":          int64(newVersion),
		"updated_at":           nowStr,
		"history_json":         historyJSON,
		"new_history_json":     newHistoryJSON,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("memgraph: supersede spec: %w", err)
	}
	if len(records) == 0 {
		// Check old spec first for precondition errors.
		oldErr := s.preconditionError(ctx, oldSlug, "supersede spec (old)", nil)
		// Always check the new spec — it may have been deleted between the
		// pre-read and the atomic query, regardless of oldErr's value.
		newErr := s.preconditionError(ctx, newSlug, "supersede spec (new)", nil)
		if newErr != nil {
			if errors.Is(newErr, storage.ErrSpecNotFound) {
				return nil, nil, fmt.Errorf("supersede spec %q: %w", newSlug, storage.ErrNewSpecNotFound)
			}
			if errors.Is(oldErr, storage.ErrConcurrentModification) {
				// Both specs have precondition issues; prefer the new-spec
				// error but include old-spec context for diagnostics.
				return nil, nil, fmt.Errorf("supersede spec: new %q: %w (old %q also concurrently modified)", newSlug, newErr, oldSlug)
			}
		}
		return nil, nil, oldErr
	}

	rec := records[0]
	// Parse old spec from positions 0-14 (15 fields).
	oldSpec, err = recordToSpec(rec)
	if err != nil {
		return nil, nil, fmt.Errorf("memgraph: supersede: parse old spec: %w", err)
	}

	// Parse new spec from positions 15-29 (15 fields) using a shifted record adapter.
	newSpec, err = recordToSpecOffset(rec, 15)
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
	historyJSON, err := appendHistory(spec.History, &entry)
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
		       s.lifecycle, s.superseded_by, s.supersedes, s.history_json,
		       s.drift_acknowledged, s.drift_acknowledge_note
	`
	records, err := s.executeQuery(ctx, query, map[string]any{
		"slug":             slug,
		"expected_version": spec.Version,
		"terminal_stages":  terminalStageStrings,
		"stage":            string(storage.SpecStageAbandoned),
		"version":          int64(newVersion),
		"updated_at":       nowStr,
		"history_json":     historyJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("memgraph: abandon spec: %w", err)
	}
	if len(records) == 0 {
		return nil, s.preconditionError(ctx, slug, "abandon spec", nil)
	}
	return recordToSpec(records[0])
}

// terminalStageStrings contains the terminal stages as a string slice for use
// in Cypher IN clauses. Computed once at init time.
var terminalStageStrings = func() []string {
	stages := make([]string, 0, len(terminalStages))
	for stage := range terminalStages {
		stages = append(stages, string(stage))
	}
	return stages
}()

// LifecycleAcknowledgeDrift sets drift as acknowledged on the spec node and returns a
// DriftReport reflecting the acknowledgment. Returns ErrSpecNotFound if the
// spec does not exist, or ErrSpecNotDone if the spec is not in an eligible
// stage (done or amended).
//
// The WHERE guard is atomic: drift can only be acknowledged on specs in the
// done or amended stages, preventing TOCTOU races between the handler check
// and the storage write.
func (s *Store) LifecycleAcknowledgeDrift(ctx context.Context, slug, note string) (*storage.DriftReport, error) {
	eligibleStages := []string{string(storage.SpecStageDone), string(storage.SpecStageAmended)}
	query := `
		MATCH (s:Spec {slug: $slug})
		WHERE s.stage IN $eligible_stages
		SET s.drift_acknowledged = true, s.drift_acknowledge_note = $note
		RETURN s.slug
	`
	records, err := s.executeQuery(ctx, query, map[string]any{
		"slug":            slug,
		"note":            note,
		"eligible_stages": eligibleStages,
	})
	if err != nil {
		return nil, fmt.Errorf("memgraph: acknowledge drift: %w", err)
	}
	if len(records) == 0 {
		return nil, s.preconditionError(ctx, slug, "acknowledge drift", func(current *storage.Spec) error {
			if current.Stage != storage.SpecStageDone && current.Stage != storage.SpecStageAmended {
				return fmt.Errorf("acknowledge drift %q (stage=%s): %w", slug, current.Stage, storage.ErrSpecIneligibleStage)
			}
			return nil
		})
	}
	return &storage.DriftReport{
		SpecSlug:        slug,
		Acknowledged:    true,
		AcknowledgeNote: note,
		Items:           []storage.DriftItem{},
	}, nil
}
