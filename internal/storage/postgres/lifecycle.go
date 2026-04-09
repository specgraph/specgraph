// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/contenthash"
)

// Compile-time interface assertion.
var _ storage.LifecycleBackend = (*Store)(nil)

// terminalStages maps stages from which no further lifecycle transitions
// are allowed. Derived from storage.FullyTerminalStages() to maintain a
// single source of truth in spec_domain.go.
var terminalStages = func() map[storage.SpecStage]bool {
	m := make(map[storage.SpecStage]bool)
	for _, s := range storage.FullyTerminalStages() {
		m[s] = true
	}
	return m
}()

// LifecycleAmendSpec transitions an in-flight spec back into an earlier authoring stage.
// The spec must be in an amend-eligible stage (approved, in_progress, review).
// reEntryStage is required — one of: spark, shape, specify, decompose.
// Returns ErrReEntryStageRequired if reEntryStage is empty,
// ErrSpecNotAmendable if the spec is not in an eligible stage, and
// ErrSpecNotFound if the spec does not exist.
func (s *Store) LifecycleAmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*storage.Spec, error) {
	if reEntryStage == "" {
		return nil, fmt.Errorf("amend spec %q: %w", slug, storage.ErrReEntryStageRequired)
	}
	targetStage := storage.SpecStage(reEntryStage)
	if targetStage.ExcludesReEntry() {
		return nil, fmt.Errorf("amend spec %q: re_entry_stage %q: %w", slug, reEntryStage, storage.ErrInvalidReEntryStage)
	}

	// landingStage is the stage the spec is set to in storage. It is one step
	// before targetStage so that the authoring command for targetStage (which
	// transitions landingStage → targetStage) succeeds after the amend.
	// For example: re-entry "shape" → landing "spark" → user runs `shape` (spark→shape).
	// For "spark" (first stage) there is no preceding stage, so spark stays at spark.
	landingStage := targetStage.PrecedingAuthStage()

	var result *storage.Spec
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		spec, getErr := s.GetSpec(txCtx, slug)
		if getErr != nil {
			return fmt.Errorf("postgres: amend spec: pre-read %q: %w", slug, getErr)
		}

		// Version guard: only proceed if spec is in an amend-eligible stage.
		tag, execErr := s.exec(txCtx,
			`UPDATE specs SET stage = $1, version = version + 1, updated_at = $2
			 WHERE slug = $3 AND project_slug = $4 AND version = $5
			   AND stage IN ('approved', 'in_progress', 'review')`,
			string(landingStage), s.now(), slug, s.project, spec.Version,
		)
		if execErr != nil {
			return fmt.Errorf("postgres: amend spec: %w", execErr)
		}
		if tag.RowsAffected() == 0 {
			return s.preconditionError(txCtx, slug, "amend spec", func(current *storage.Spec) error {
				if current.Version != spec.Version {
					return fmt.Errorf("amend spec %q: %w", slug, storage.ErrConcurrentModification)
				}
				if !current.Stage.IsAmendEligible() {
					return fmt.Errorf("amend spec %q (stage=%s): %w", slug, current.Stage, storage.ErrSpecNotAmendable)
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

		summary := fmt.Sprintf("Amended from %s, re-entering at: %s", spec.Stage, targetStage)
		deltas := []storage.FieldChange{{Field: "stage", OldValue: string(spec.Stage), NewValue: string(landingStage)}}
		clEntry := &storage.ChangeLogEntry{
			Version:     freshSpec.Version,
			Stage:       string(freshSpec.Stage),
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

// LifecycleSupersedeSpec marks the old spec as superseded and links it to the new spec
// via a SUPERSEDES edge. Both specs are returned with updated fields.
func (s *Store) LifecycleSupersedeSpec(ctx context.Context, oldSlug, newSlug string) (oldSpec, newSpec *storage.Spec, err error) {
	if oldSlug == newSlug {
		return nil, nil, fmt.Errorf("supersede spec (%q): %w", oldSlug, storage.ErrSameSlugs)
	}

	txErr := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		oldCheck, preErr := s.GetSpec(txCtx, oldSlug)
		if preErr != nil {
			return fmt.Errorf("postgres: supersede spec: pre-read %q: %w", oldSlug, preErr)
		}
		if oldCheck.Stage != storage.SpecStageDone {
			return fmt.Errorf("supersede spec %q (stage=%s): %w", oldSlug, oldCheck.Stage, storage.ErrSpecNotDone)
		}
		newCheck, newErr := s.GetSpec(txCtx, newSlug)
		if newErr != nil {
			if errors.Is(newErr, storage.ErrSpecNotFound) {
				return fmt.Errorf("supersede spec: new spec %q: %w", newSlug, storage.ErrNewSpecNotFound)
			}
			return fmt.Errorf("supersede spec: new spec %q: %w", newSlug, newErr)
		}
		if terminalStages[newCheck.Stage] {
			return fmt.Errorf("supersede spec: new spec %q (stage=%s): %w", newSlug, newCheck.Stage, storage.ErrNewSpecTerminal)
		}

		now := s.now()

		// Version guard on old spec: set to superseded.
		oldTag, oldErr := s.exec(txCtx,
			`UPDATE specs SET stage = $1, superseded_by = $2, version = version + 1, updated_at = $3
			 WHERE slug = $4 AND project_slug = $5 AND version = $6 AND stage = 'done'`,
			string(storage.SpecStageSuperseded), newSlug, now,
			oldSlug, s.project, oldCheck.Version,
		)
		if oldErr != nil {
			return fmt.Errorf("postgres: supersede spec (old): %w", oldErr)
		}
		if oldTag.RowsAffected() == 0 {
			return s.preconditionError(txCtx, oldSlug, "supersede spec (old)", func(current *storage.Spec) error {
				if current.Version != oldCheck.Version {
					return fmt.Errorf("supersede spec (old) %q: %w", oldSlug, storage.ErrConcurrentModification)
				}
				return nil
			})
		}

		// Version guard on new spec: set supersedes field.
		newTag, newExecErr := s.exec(txCtx,
			`UPDATE specs SET supersedes = $1, version = version + 1, updated_at = $2
			 WHERE slug = $3 AND project_slug = $4 AND version = $5
			   AND stage NOT IN (SELECT unnest($6::text[]))`,
			oldSlug, now,
			newSlug, s.project, newCheck.Version,
			terminalStageStrings(),
		)
		if newExecErr != nil {
			return fmt.Errorf("postgres: supersede spec (new): %w", newExecErr)
		}
		if newTag.RowsAffected() == 0 {
			return s.preconditionError(txCtx, newSlug, "supersede spec (new)", func(current *storage.Spec) error {
				if current.Version != newCheck.Version {
					return fmt.Errorf("supersede spec (new) %q: %w", newSlug, storage.ErrConcurrentModification)
				}
				return nil
			})
		}

		// Recompute content hash for old spec (stage changed to superseded).
		if hashErr := s.recomputeContentHash(txCtx, oldSlug); hashErr != nil {
			return hashErr
		}

		// Create SUPERSEDES edge: new -> old.
		_, edgeErr := s.exec(txCtx,
			`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug)
			 VALUES ($1, $2, 'SUPERSEDES', $3)
			 ON CONFLICT (project_slug, from_slug, to_slug, edge_type) DO NOTHING`,
			newSlug, oldSlug, s.project,
		)
		if edgeErr != nil {
			return fmt.Errorf("postgres: supersede spec: create edge: %w", edgeErr)
		}

		// Re-read both specs.
		var getErr error
		oldSpec, getErr = s.GetSpec(txCtx, oldSlug)
		if getErr != nil {
			return fmt.Errorf("postgres: supersede: re-read old spec: %w", getErr)
		}
		newSpec, getErr = s.GetSpec(txCtx, newSlug)
		if getErr != nil {
			return fmt.Errorf("postgres: supersede: re-read new spec: %w", getErr)
		}

		// Changelog for old spec.
		oldDeltas := []storage.FieldChange{
			{Field: "stage", OldValue: string(oldCheck.Stage), NewValue: string(storage.SpecStageSuperseded)},
			{Field: "superseded_by", OldValue: "", NewValue: newSlug},
		}
		oldCLEntry := &storage.ChangeLogEntry{
			Version:     oldSpec.Version,
			Stage:       string(storage.SpecStageSuperseded),
			ContentHash: oldSpec.ContentHash,
			Checkpoint:  true,
			Summary:     "Spec superseded",
			Reason:      fmt.Sprintf("Superseded by %s", newSlug),
			Date:        oldSpec.UpdatedAt,
		}
		if clErr := s.createChangeLog(txCtx, oldSlug, oldCLEntry, oldDeltas); clErr != nil {
			return clErr
		}

		// Changelog for new spec.
		newDeltas := []storage.FieldChange{
			{Field: "supersedes", OldValue: "", NewValue: oldSlug},
		}
		newCLEntry := &storage.ChangeLogEntry{
			Version:     newSpec.Version,
			Stage:       string(newSpec.Stage),
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

// LifecycleAbandonSpec transitions a spec to the abandoned terminal state.
func (s *Store) LifecycleAbandonSpec(ctx context.Context, slug, reason string) (*storage.Spec, error) {
	var result *storage.Spec
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		spec, getErr := s.GetSpec(txCtx, slug)
		if getErr != nil {
			return fmt.Errorf("postgres: abandon spec: pre-read %q: %w", slug, getErr)
		}
		if terminalStages[spec.Stage] {
			return fmt.Errorf("abandon spec %q (stage=%s): %w", slug, spec.Stage, storage.ErrSpecTerminal)
		}

		tag, execErr := s.exec(txCtx,
			`UPDATE specs SET stage = $1, version = version + 1, updated_at = $2
			 WHERE slug = $3 AND project_slug = $4 AND version = $5
			   AND stage NOT IN (SELECT unnest($6::text[]))`,
			string(storage.SpecStageAbandoned), s.now(),
			slug, s.project, spec.Version,
			terminalStageStrings(),
		)
		if execErr != nil {
			return fmt.Errorf("postgres: abandon spec: %w", execErr)
		}
		if tag.RowsAffected() == 0 {
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
			Stage:       string(storage.SpecStageAbandoned),
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

// LifecycleAcknowledgeDrift refreshes content_hash_at_link on DEPENDS_ON edges,
// acknowledging drift from an upstream spec (or all upstreams if upstreamSlug
// is empty).
func (s *Store) LifecycleAcknowledgeDrift(ctx context.Context, slug, upstreamSlug, note string) error {
	eligibleStages := []string{string(storage.SpecStageDone)}

	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Verify spec exists and is in eligible stage.
		var specStage string
		err := s.queryRow(txCtx,
			`SELECT stage FROM specs WHERE slug = $1 AND project_slug = $2`,
			slug, s.project,
		).Scan(&specStage)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("acknowledge drift %q: %w", slug, storage.ErrSpecNotFound)
			}
			return fmt.Errorf("postgres: acknowledge drift: check spec: %w", err)
		}
		stageOK := false
		for _, es := range eligibleStages {
			if specStage == es {
				stageOK = true
				break
			}
		}
		if !stageOK {
			return fmt.Errorf("acknowledge drift %q (stage=%s): %w", slug, specStage, storage.ErrSpecIneligibleStage)
		}

		// Refresh edge hashes.
		if upstreamSlug != "" {
			// Per-upstream: update specific edge.
			upTag, upErr := s.exec(txCtx,
				`UPDATE edges e
				 SET content_hash_at_link = COALESCE(upstream.content_hash, '')
				 FROM (
				     SELECT slug, content_hash FROM specs WHERE project_slug = $3
				     UNION ALL
				     SELECT slug, content_hash FROM decisions WHERE project_slug = $3
				 ) upstream
				 WHERE e.from_slug = $1 AND e.to_slug = $2 AND e.edge_type = 'DEPENDS_ON'
				   AND e.project_slug = $3 AND upstream.slug = e.to_slug`,
				slug, upstreamSlug, s.project,
			)
			if upErr != nil {
				return fmt.Errorf("postgres: acknowledge drift update edge: %w", upErr)
			}
			if upTag.RowsAffected() == 0 {
				return fmt.Errorf("postgres: no DEPENDS_ON edge from %q to %q: %w", slug, upstreamSlug, storage.ErrEdgeNotFound)
			}
		} else {
			// Blanket: refresh all outgoing DEPENDS_ON edges.
			if refreshErr := s.RefreshDependencyHashes(txCtx, slug); refreshErr != nil {
				return refreshErr
			}
		}

		// Create changelog entry.
		target := upstreamSlug
		if target == "" {
			target = "all upstreams"
		}
		spec, specErr := s.GetSpec(txCtx, slug)
		if specErr != nil {
			return fmt.Errorf("postgres: acknowledge drift changelog: %w", specErr)
		}
		clEntry := &storage.ChangeLogEntry{
			Version:     spec.Version,
			Stage:       string(spec.Stage),
			ContentHash: spec.ContentHash,
			Summary:     fmt.Sprintf("Acknowledged drift from %s", target),
			Reason:      note,
			Checkpoint:  false,
			Date:        s.now(),
		}
		if clErr := s.createChangeLog(txCtx, slug, clEntry, nil); clErr != nil {
			return fmt.Errorf("postgres: acknowledge drift changelog: %w", clErr)
		}

		return nil
	})
}

// preconditionError re-reads the spec after an atomic WHERE guard failed
// and returns the appropriate sentinel error.
func (s *Store) preconditionError(ctx context.Context, slug, op string, extraChecks func(*storage.Spec) error) error {
	current, err := s.GetSpec(ctx, slug)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return fmt.Errorf("%s %q: %w", op, slug, storage.ErrSpecNotFound)
		}
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
	return fmt.Errorf("%s %q (stage=%s, version=%d): unexplained guard failure: %w",
		op, slug, current.Stage, current.Version, storage.ErrInternalGuardFailure)
}

// terminalStageStrings returns terminal stages as a string slice for use
// in SQL IN/ANY clauses.
func terminalStageStrings() []string {
	stages := make([]string, 0, len(terminalStages))
	for stage := range terminalStages {
		stages = append(stages, string(stage))
	}
	return stages
}

// recomputeContentHash reads all hash-input fields from the spec,
// computes a new Murmur3-128 content hash, and persists it.
func (s *Store) recomputeContentHash(ctx context.Context, slug string) error {
	var (
		intent          string
		stage           string
		priority        string
		complexity      string
		sparkOutput     *storage.SparkOutput
		shapeOutput     *storage.ShapeOutput
		specifyOutput   *storage.SpecifyOutput
		decomposeOutput *storage.DecomposeOutput
	)

	err := s.queryRow(ctx,
		`SELECT intent, stage, priority, complexity,
		        spark_output, shape_output, specify_output, decompose_output
		 FROM specs WHERE slug = $1 AND project_slug = $2`,
		slug, s.project,
	).Scan(&intent, &stage, &priority, &complexity,
		&sparkOutput, &shapeOutput, &specifyOutput, &decomposeOutput)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("postgres: recompute content_hash %q: %w", slug, storage.ErrSpecNotFound)
		}
		return fmt.Errorf("postgres: recompute content_hash: read fields: %w", err)
	}

	// Build authoring outputs map from non-nil fields.
	outputs := make(map[string]string)
	if sparkOutput != nil {
		b, mErr := json.Marshal(sparkOutput)
		if mErr != nil {
			return fmt.Errorf("postgres: recompute content_hash: marshal spark_output: %w", mErr)
		}
		outputs["spark_output"] = string(b)
	}
	if shapeOutput != nil {
		b, mErr := json.Marshal(shapeOutput)
		if mErr != nil {
			return fmt.Errorf("postgres: recompute content_hash: marshal shape_output: %w", mErr)
		}
		outputs["shape_output"] = string(b)
	}
	if specifyOutput != nil {
		b, mErr := json.Marshal(specifyOutput)
		if mErr != nil {
			return fmt.Errorf("postgres: recompute content_hash: marshal specify_output: %w", mErr)
		}
		outputs["specify_output"] = string(b)
	}
	if decomposeOutput != nil {
		b, mErr := json.Marshal(decomposeOutput)
		if mErr != nil {
			return fmt.Errorf("postgres: recompute content_hash: marshal decompose_output: %w", mErr)
		}
		outputs["decompose_output"] = string(b)
	}

	ch := contenthash.Spec(intent, stage, priority, complexity, outputs)

	_, err = s.exec(ctx,
		`UPDATE specs SET content_hash = $1 WHERE slug = $2 AND project_slug = $3`,
		ch, slug, s.project,
	)
	if err != nil {
		return fmt.Errorf("postgres: recompute content_hash: update: %w", err)
	}
	return nil
}
