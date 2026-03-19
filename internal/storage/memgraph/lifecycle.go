// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"errors"
	"fmt"

	"github.com/seanb4t/specgraph/internal/storage"
)

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

// preconditionError re-reads the spec after an atomic WHERE guard failed
// and returns the appropriate sentinel error. The op parameter names the
// operation (e.g. "amend spec", "supersede spec") for error messages.
// extraChecks, if non-nil, runs after the terminal-stage check passes,
// allowing callers to add operation-specific precondition validation.
func (s *Store) preconditionError(ctx context.Context, slug, op string, extraChecks func(*storage.Spec) error) error {
	current, err := s.GetSpec(ctx, slug)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return fmt.Errorf("%s %q: %w", op, slug, storage.ErrSpecNotFound)
		}
		// Guard failure indicates concurrent modification. If re-read also
		// fails, wrap ErrConcurrentModification so the handler maps to
		// CodeAborted (retryable) rather than CodeInternal.
		return fmt.Errorf("%s %q: %w (re-read failed: %w)", op, slug, storage.ErrConcurrentModification, err)
	}
	if terminalStages[current.Stage] {
		return fmt.Errorf("%s %q (stage=%s): %w", op, slug, current.Stage, storage.ErrSpecTerminal)
	}
	if extraChecks != nil {
		if err := extraChecks(current); err != nil {
			return err
		}
	}
	// Catch-all: the atomic WHERE guard returned 0 rows but the spec exists,
	// is not terminal, and no extra-check explains the failure. This branch
	// is only reachable through Cypher bugs or unexpected Memgraph behavior
	// (version guard should always catch concurrent modifications). Kept as
	// defense-in-depth; exercised only at the integration level.
	return fmt.Errorf("%s %q (stage=%s, version=%d): unexplained guard failure: %w",
		op, slug, current.Stage, current.Version, storage.ErrInternalGuardFailure)
}

// LifecycleAmendSpec transitions a done spec back into an earlier authoring stage.
// If reEntryStage is empty, the spec is set to "amended".
// Returns ErrSpecNotDone if the spec is not at the "done" stage, and ErrSpecNotFound
// if the spec does not exist.
//
// The transition is atomic: a single Cypher query gates on the expected stage
// and version so concurrent requests cannot overwrite each other.
func (s *Store) LifecycleAmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*storage.Spec, error) {
	// Read current state to compute new version.
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("memgraph: amend spec: pre-read %q: %w", slug, err)
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

	nowStr := s.now()
	// Atomic: WHERE guards ensure we only transition if the spec is still at
	// the expected stage and version, preventing TOCTOU races.
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		WHERE s.stage = $expected_stage AND s.version = $expected_version
		SET s.stage = $stage,
		    s.version = $version,
		    s.updated_at = $updated_at
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes,
		       s.drift_acknowledged, s.drift_acknowledge_note, s.notes,
		       s.content_hash
	`
	records, err := s.executeQuery(ctx, query, mergeParams(s.projectParam(), map[string]any{
		"slug":             slug,
		"expected_stage":   string(storage.SpecStageDone),
		"expected_version": spec.Version,
		"stage":            string(targetStage),
		"version":          int64(newVersion),
		"updated_at":       nowStr,
	}))
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
	if hashErr := s.recomputeContentHash(ctx, slug); hashErr != nil {
		return nil, hashErr
	}
	updatedSpec, err := recordToSpec(records[0])
	if err != nil {
		return nil, err
	}
	// Re-read content hash after recomputation for the ChangeLog entry.
	freshSpec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, err
	}
	summary := "Amended from done"
	if targetStage != storage.SpecStageAmended {
		summary = fmt.Sprintf("Amended from done, re-entering at: %s", targetStage)
	}
	deltas := []storage.FieldChange{{Field: "stage", OldValue: string(storage.SpecStageDone), NewValue: string(targetStage)}}
	clEntry := &storage.ChangeLogEntry{
		Version:     freshSpec.Version,
		Stage:       freshSpec.Stage,
		ContentHash: freshSpec.ContentHash,
		Checkpoint:  true,
		Summary:     summary,
		Reason:      reason,
		Date:        freshSpec.UpdatedAt,
	}
	if clErr := s.createChangeLog(ctx, slug, clEntry, deltas); clErr != nil {
		return nil, clErr
	}
	return updatedSpec, nil
}

// LifecycleSupersedeSpec marks the old spec as superseded and links it to the new spec via
// a SUPERSEDES edge. Both specs are returned with updated fields. Returns
// ErrSpecNotFound if the old spec doesn't exist, and ErrNewSpecNotFound if the
// new spec doesn't exist.
//
// Unlike AmendSpec, supersession is allowed from any non-terminal stage (not
// just "done"). A spec may be superseded at any point in the authoring funnel
// when requirements change enough to warrant a new spec.
//
// The transition is atomic: a WHERE clause gates on the old spec not being
// in a terminal stage, preventing TOCTOU races.
func (s *Store) LifecycleSupersedeSpec(ctx context.Context, oldSlug, newSlug string) (oldSpec, newSpec *storage.Spec, err error) {
	if oldSlug == newSlug {
		return nil, nil, fmt.Errorf("supersede spec (%q): %w", oldSlug, storage.ErrSameSlugs)
	}
	// Pre-validate: check old spec exists and new spec exists.
	oldCheck, err := s.GetSpec(ctx, oldSlug)
	if err != nil {
		return nil, nil, fmt.Errorf("memgraph: supersede spec: pre-read %q: %w", oldSlug, err)
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
	if terminalStages[newCheck.Stage] {
		return nil, nil, fmt.Errorf("supersede spec: new spec %q (stage=%s): %w", newSlug, newCheck.Stage, storage.ErrNewSpecTerminal)
	}

	oldVersion := oldCheck.Version + 1
	newVersion := newCheck.Version + 1

	nowStr := s.now()
	// Atomic: WHERE guards ensure neither spec has entered a terminal state
	// since our pre-validation read.
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(old:Spec {slug: $old_slug}),
		      (p)<-[:BELONGS_TO]-(new:Spec {slug: $new_slug})
		WHERE NOT old.stage IN $terminal_stages
		      AND NOT new.stage IN $terminal_stages
		      AND old.version = $expected_version
		      AND new.version = $expected_new_version
		SET old.stage = $stage,
		    old.superseded_by = $new_slug,
		    old.version = $version,
		    old.updated_at = $updated_at,
		    new.supersedes = $old_slug,
		    new.version = $new_version,
		    new.updated_at = $updated_at
		MERGE (new)-[:SUPERSEDES]->(old)
		RETURN old.id, old.slug, old.intent, old.stage, old.priority, old.complexity,
		       old.version, old.created_at, old.updated_at,
		       old.lifecycle, old.superseded_by, old.supersedes,
		       old.drift_acknowledged, old.drift_acknowledge_note, old.notes,
		       old.content_hash,
		       new.id, new.slug, new.intent, new.stage, new.priority, new.complexity,
		       new.version, new.created_at, new.updated_at,
		       new.lifecycle, new.superseded_by, new.supersedes,
		       new.drift_acknowledged, new.drift_acknowledge_note, new.notes,
		       new.content_hash
	`
	records, err := s.executeQuery(ctx, query, mergeParams(s.projectParam(), map[string]any{
		"old_slug":             oldSlug,
		"new_slug":             newSlug,
		"stage":                string(storage.SpecStageSuperseded),
		"terminal_stages":      terminalStageStrings,
		"expected_version":     oldCheck.Version,
		"expected_new_version": newCheck.Version,
		"version":              int64(oldVersion),
		"updated_at":           nowStr,
		"new_version":          int64(newVersion),
	}))
	if err != nil {
		return nil, nil, fmt.Errorf("memgraph: supersede spec: %w", err)
	}
	if len(records) == 0 {
		// Check old spec first for precondition errors.
		oldErr := s.preconditionError(ctx, oldSlug, "supersede spec (old)", func(current *storage.Spec) error {
			if current.Version != oldCheck.Version {
				return fmt.Errorf("supersede spec (old) %q: %w", oldSlug, storage.ErrConcurrentModification)
			}
			return nil
		})
		// Only check the new spec when the old spec doesn't provide a
		// definitive answer (NotFound/Terminal explain the guard failure on
		// their own). ErrConcurrentModification is ambiguous — both specs
		// may have raced — so we still check the new spec in that case.
		// preconditionError always returns non-nil, so we fall through
		// directly to returning oldErr here.
		if errors.Is(oldErr, storage.ErrConcurrentModification) {
			newErr := s.preconditionError(ctx, newSlug, "supersede spec (new)", func(current *storage.Spec) error {
				if current.Version != newCheck.Version {
					return fmt.Errorf("supersede spec (new) %q: %w", newSlug, storage.ErrConcurrentModification)
				}
				return nil
			})
			if errors.Is(newErr, storage.ErrSpecNotFound) {
				return nil, nil, fmt.Errorf("supersede spec %q: %w", newSlug, storage.ErrNewSpecNotFound)
			}
			if errors.Is(newErr, storage.ErrSpecTerminal) {
				return nil, nil, fmt.Errorf("supersede spec: new spec %q is in a terminal state: %w", newSlug, storage.ErrNewSpecTerminal)
			}
			// Both specs had concurrent modifications — surface both errors.
			// Go 1.20+ fmt.Errorf with two %w creates a multi-error (errors.Join
			// semantics). errors.Is checks match either wrapped error.
			return nil, nil, fmt.Errorf("supersede spec: new %q: %w (old %q also had precondition error: %w)", newSlug, newErr, oldSlug, oldErr)
		}
		return nil, nil, oldErr
	}

	rec := records[0]
	// Parse old spec from positions 0-15 (16 fields).
	oldSpec, err = recordToSpec(rec)
	if err != nil {
		return nil, nil, fmt.Errorf("memgraph: supersede: parse old spec: %w", err)
	}

	// Parse new spec from positions 16-31 (16 fields) using a shifted record adapter.
	newSpec, err = recordToSpecOffset(rec, 16)
	if err != nil {
		return nil, nil, fmt.Errorf("memgraph: supersede: parse new spec: %w", err)
	}

	// Create checkpoint ChangeLog for the old spec (→ superseded).
	oldDeltas := []storage.FieldChange{
		{Field: "stage", OldValue: string(oldCheck.Stage), NewValue: string(storage.SpecStageSuperseded)},
		{Field: "superseded_by", OldValue: "", NewValue: newSlug},
	}
	oldCLEntry := &storage.ChangeLogEntry{
		Version:     oldSpec.Version,
		Stage:       storage.SpecStageSuperseded,
		ContentHash: oldSpec.ContentHash,
		Checkpoint:  true,
		Summary:     "Spec superseded",
		Reason:      fmt.Sprintf("Superseded by %s", newSlug),
		Date:        oldSpec.UpdatedAt,
	}
	if clErr := s.createChangeLog(ctx, oldSlug, oldCLEntry, oldDeltas); clErr != nil {
		return nil, nil, clErr
	}

	// Create checkpoint ChangeLog for the new spec (supersedes predecessor).
	newDeltas := []storage.FieldChange{
		{Field: "supersedes", OldValue: "", NewValue: oldSlug},
	}
	newCLEntry := &storage.ChangeLogEntry{
		Version:     newSpec.Version,
		Stage:       newSpec.Stage,
		ContentHash: newSpec.ContentHash,
		Checkpoint:  true,
		Summary:     "Supersedes predecessor",
		Reason:      fmt.Sprintf("Supersedes %s", oldSlug),
		Date:        newSpec.UpdatedAt,
	}
	if clErr := s.createChangeLog(ctx, newSlug, newCLEntry, newDeltas); clErr != nil {
		return nil, nil, clErr
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
		return nil, fmt.Errorf("memgraph: abandon spec: pre-read %q: %w", slug, err)
	}
	if terminalStages[spec.Stage] {
		return nil, fmt.Errorf("abandon spec %q (stage=%s): %w", slug, spec.Stage, storage.ErrSpecTerminal)
	}

	newVersion := spec.Version + 1

	nowStr := s.now()
	// Atomic: WHERE guards on stage and version prevent TOCTOU races.
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		WHERE NOT s.stage IN $terminal_stages AND s.version = $expected_version
		SET s.stage = $stage,
		    s.version = $version,
		    s.updated_at = $updated_at
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes,
		       s.drift_acknowledged, s.drift_acknowledge_note, s.notes,
		       s.content_hash
	`
	records, err := s.executeQuery(ctx, query, mergeParams(s.projectParam(), map[string]any{
		"slug":             slug,
		"expected_version": spec.Version,
		"terminal_stages":  terminalStageStrings,
		"stage":            string(storage.SpecStageAbandoned),
		"version":          int64(newVersion),
		"updated_at":       nowStr,
	}))
	if err != nil {
		return nil, fmt.Errorf("memgraph: abandon spec: %w", err)
	}
	if len(records) == 0 {
		return nil, s.preconditionError(ctx, slug, "abandon spec", func(current *storage.Spec) error {
			if current.Version != spec.Version {
				return fmt.Errorf("abandon spec %q: %w", slug, storage.ErrConcurrentModification)
			}
			return nil
		})
	}
	abandonedSpec, err := recordToSpec(records[0])
	if err != nil {
		return nil, err
	}
	deltas := []storage.FieldChange{
		{Field: "stage", OldValue: string(spec.Stage), NewValue: string(storage.SpecStageAbandoned)},
	}
	clEntry := &storage.ChangeLogEntry{
		Version:     abandonedSpec.Version,
		Stage:       storage.SpecStageAbandoned,
		ContentHash: abandonedSpec.ContentHash,
		Checkpoint:  true,
		Summary:     "Spec abandoned",
		Reason:      reason,
		Date:        abandonedSpec.UpdatedAt,
	}
	if clErr := s.createChangeLog(ctx, slug, clEntry, deltas); clErr != nil {
		return nil, clErr
	}
	return abandonedSpec, nil
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
// spec does not exist, or ErrSpecIneligibleStage if the spec is not in an eligible
// stage (done or amended).
//
// The WHERE guard is atomic: drift can only be acknowledged on specs in the
// done or amended stages, preventing TOCTOU races between the handler check
// and the storage write.
func (s *Store) LifecycleAcknowledgeDrift(ctx context.Context, slug, note string) (*storage.DriftReport, error) {
	eligibleStages := []string{string(storage.SpecStageDone), string(storage.SpecStageAmended)}
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		WHERE s.stage IN $eligible_stages
		SET s.drift_acknowledged = true, s.drift_acknowledge_note = $note
		RETURN s.slug, s.drift_acknowledged, s.drift_acknowledge_note
	`
	records, err := s.executeQuery(ctx, query, mergeParams(s.projectParam(), map[string]any{
		"slug":            slug,
		"note":            note,
		"eligible_stages": eligibleStages,
	}))
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
	rec := records[0]
	ack, _ := rec.Get("s.drift_acknowledged")
	ackNote, _ := rec.Get("s.drift_acknowledge_note")
	acknowledged, ok := ack.(bool)
	if !ok {
		return nil, fmt.Errorf("memgraph: acknowledge drift %q: unexpected type for drift_acknowledged: %T", slug, ack)
	}
	acknowledgeNote, ok := ackNote.(string)
	if !ok {
		return nil, fmt.Errorf("memgraph: acknowledge drift %q: unexpected type for drift_acknowledge_note: %T", slug, ackNote)
	}
	return &storage.DriftReport{
		SpecSlug:        slug,
		Acknowledged:    acknowledged,
		AcknowledgeNote: acknowledgeNote,
		Items:           []storage.DriftItem{},
	}, nil
}
