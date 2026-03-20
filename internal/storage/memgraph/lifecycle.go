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
		       s.notes, s.content_hash
	`
	var result *storage.Spec
	err = s.RunInTransaction(ctx, func(txCtx context.Context) error {
		records, qErr := s.executeQuery(txCtx, query, mergeParams(s.projectParam(), map[string]any{
			"slug":             slug,
			"expected_stage":   string(storage.SpecStageDone),
			"expected_version": spec.Version,
			"stage":            string(targetStage),
			"version":          int64(newVersion),
			"updated_at":       nowStr,
		}))
		if qErr != nil {
			return fmt.Errorf("memgraph: amend spec: %w", qErr)
		}
		if len(records) == 0 {
			return s.preconditionError(txCtx, slug, "amend spec", func(current *storage.Spec) error {
				if current.Version != spec.Version {
					return fmt.Errorf("amend spec %q: %w", slug, storage.ErrConcurrentModification)
				}
				if current.Stage != storage.SpecStageDone {
					return fmt.Errorf("amend spec %q (stage=%s): %w", slug, current.Stage, storage.ErrSpecNotDone)
				}
				return nil
			})
		}
		if hashErr := s.recomputeContentHash(txCtx, slug); hashErr != nil {
			return hashErr
		}
		freshSpec, getErr := s.GetSpec(txCtx, slug)
		if getErr != nil {
			return getErr
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
		if clErr := s.createChangeLog(txCtx, slug, clEntry, deltas); clErr != nil {
			return clErr
		}
		result = freshSpec
		return nil
	})
	return result, err
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
		       old.notes, old.content_hash,
		       new.id, new.slug, new.intent, new.stage, new.priority, new.complexity,
		       new.version, new.created_at, new.updated_at,
		       new.lifecycle, new.superseded_by, new.supersedes,
		       new.notes, new.content_hash
	`
	txErr := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		records, qErr := s.executeQuery(txCtx, query, mergeParams(s.projectParam(), map[string]any{
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
		if qErr != nil {
			return fmt.Errorf("memgraph: supersede spec: %w", qErr)
		}
		if len(records) == 0 {
			// Check old spec first for precondition errors.
			oldErr := s.preconditionError(txCtx, oldSlug, "supersede spec (old)", func(current *storage.Spec) error {
				if current.Version != oldCheck.Version {
					return fmt.Errorf("supersede spec (old) %q: %w", oldSlug, storage.ErrConcurrentModification)
				}
				return nil
			})
			if errors.Is(oldErr, storage.ErrConcurrentModification) {
				newPErr := s.preconditionError(txCtx, newSlug, "supersede spec (new)", func(current *storage.Spec) error {
					if current.Version != newCheck.Version {
						return fmt.Errorf("supersede spec (new) %q: %w", newSlug, storage.ErrConcurrentModification)
					}
					return nil
				})
				if errors.Is(newPErr, storage.ErrSpecNotFound) {
					return fmt.Errorf("supersede spec %q: %w", newSlug, storage.ErrNewSpecNotFound)
				}
				if errors.Is(newPErr, storage.ErrSpecTerminal) {
					return fmt.Errorf("supersede spec: new spec %q is in a terminal state: %w", newSlug, storage.ErrNewSpecTerminal)
				}
				return fmt.Errorf("supersede spec: new %q: %w (old %q also had precondition error: %w)", newSlug, newPErr, oldSlug, oldErr)
			}
			return oldErr
		}

		// Recompute content hashes for both specs after the stage change.
		if hashErr := s.recomputeContentHash(txCtx, oldSlug); hashErr != nil {
			return hashErr
		}
		// Only recompute hash for oldSpec (stage changed to superseded).
		// newSpec's stage is unchanged — supersedes is not a hash-input field.
		// Re-read both specs to get fresh values.
		var getErr error
		oldSpec, getErr = s.GetSpec(txCtx, oldSlug)
		if getErr != nil {
			return fmt.Errorf("memgraph: supersede: re-read old spec: %w", getErr)
		}
		newSpec, getErr = s.GetSpec(txCtx, newSlug)
		if getErr != nil {
			return fmt.Errorf("memgraph: supersede: re-read new spec: %w", getErr)
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
		if clErr := s.createChangeLog(txCtx, oldSlug, oldCLEntry, oldDeltas); clErr != nil {
			return clErr
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
		return s.createChangeLog(txCtx, newSlug, newCLEntry, newDeltas)
	})
	if txErr != nil {
		return nil, nil, txErr
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
		       s.notes, s.content_hash
	`
	var result *storage.Spec
	err = s.RunInTransaction(ctx, func(txCtx context.Context) error {
		records, qErr := s.executeQuery(txCtx, query, mergeParams(s.projectParam(), map[string]any{
			"slug":             slug,
			"expected_version": spec.Version,
			"terminal_stages":  terminalStageStrings,
			"stage":            string(storage.SpecStageAbandoned),
			"version":          int64(newVersion),
			"updated_at":       nowStr,
		}))
		if qErr != nil {
			return fmt.Errorf("memgraph: abandon spec: %w", qErr)
		}
		if len(records) == 0 {
			return s.preconditionError(txCtx, slug, "abandon spec", func(current *storage.Spec) error {
				if current.Version != spec.Version {
					return fmt.Errorf("abandon spec %q: %w", slug, storage.ErrConcurrentModification)
				}
				return nil
			})
		}
		if hashErr := s.recomputeContentHash(txCtx, slug); hashErr != nil {
			return hashErr
		}
		abandonedSpec, getErr := s.GetSpec(txCtx, slug)
		if getErr != nil {
			return getErr
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
		if clErr := s.createChangeLog(txCtx, slug, clEntry, deltas); clErr != nil {
			return clErr
		}
		result = abandonedSpec
		return nil
	})
	return result, err
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

// LifecycleAcknowledgeDrift refreshes content_hash_at_link on DEPENDS_ON edges,
// acknowledging drift from an upstream spec (or all upstreams if upstreamSlug
// is empty). Returns ErrSpecNotFound if the spec does not exist, or
// ErrSpecIneligibleStage if the spec is not in an eligible stage (done or amended).
//
// All queries are wrapped in RunInTransaction per ADR-004.
func (s *Store) LifecycleAcknowledgeDrift(ctx context.Context, slug, upstreamSlug, note string) error {
	eligibleStages := []string{string(storage.SpecStageDone), string(storage.SpecStageAmended)}

	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Verify spec is in eligible stage.
		checkQuery := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
			WHERE s.stage IN $eligible_stages
			RETURN s.slug
		`
		records, err := s.executeQuery(txCtx, checkQuery, mergeParams(s.projectParam(), map[string]any{
			"slug":            slug,
			"eligible_stages": eligibleStages,
		}))
		if err != nil {
			return fmt.Errorf("memgraph: acknowledge drift: %w", err)
		}
		if len(records) == 0 {
			return s.preconditionError(txCtx, slug, "acknowledge drift", func(current *storage.Spec) error {
				if current.Stage != storage.SpecStageDone && current.Stage != storage.SpecStageAmended {
					return fmt.Errorf("acknowledge drift %q (stage=%s): %w", slug, current.Stage, storage.ErrSpecIneligibleStage)
				}
				return nil
			})
		}

		// Update edge hash(es) and return affected count.
		var updateQuery string
		params := mergeParams(s.projectParam(), map[string]any{"slug": slug})

		if upstreamSlug != "" {
			updateQuery = `
				MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a:Spec {slug: $slug})-[dep:DEPENDS_ON]->(upstream {slug: $upstream_slug})
				SET dep.content_hash_at_link = COALESCE(upstream.content_hash, "")
				RETURN count(dep) AS matched
			`
			params["upstream_slug"] = upstreamSlug
		} else {
			updateQuery = `
				MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a:Spec {slug: $slug})-[dep:DEPENDS_ON]->(upstream)
				SET dep.content_hash_at_link = COALESCE(upstream.content_hash, "")
				RETURN count(dep) AS matched
			`
		}

		updateRecords, updateErr := s.executeQuery(txCtx, updateQuery, params)
		if updateErr != nil {
			return fmt.Errorf("memgraph: acknowledge drift update edge: %w", updateErr)
		}
		// If a specific upstream was requested but no edge matched, fail fast.
		if upstreamSlug != "" {
			matched := int64(0)
			if len(updateRecords) > 0 {
				if v, ok := updateRecords[0].Get("matched"); ok && v != nil {
					if n, ok := v.(int64); ok {
						matched = n
					}
				}
			}
			if matched == 0 {
				return fmt.Errorf("memgraph: no DEPENDS_ON edge from %q to %q: %w", slug, upstreamSlug, storage.ErrEdgeNotFound)
			}
		}

		// Create a ChangeLog entry recording the acknowledgment.
		target := upstreamSlug
		if target == "" {
			target = "all upstreams"
		}
		spec, specErr := s.GetSpec(txCtx, slug)
		if specErr != nil {
			return fmt.Errorf("memgraph: acknowledge drift changelog: %w", specErr)
		}
		clEntry := &storage.ChangeLogEntry{
			Version:     spec.Version,
			Stage:       spec.Stage,
			ContentHash: spec.ContentHash,
			Summary:     fmt.Sprintf("Acknowledged drift from %s", target),
			Reason:      note,
			Checkpoint:  false,
			Date:        s.nowTime(),
		}
		if clErr := s.createChangeLog(txCtx, slug, clEntry, nil); clErr != nil {
			return fmt.Errorf("memgraph: acknowledge drift changelog: %w", clErr)
		}

		return nil
	})
}
