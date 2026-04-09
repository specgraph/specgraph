// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/specgraph/specgraph/internal/storage"
)

// Compile-time interface assertion.
var _ storage.AuthoringBackend = (*Store)(nil)

// TransitionStage validates and applies a spec's stage transition.
// Returns ErrSpecAlreadyApproved if from is the approved stage,
// ErrInvalidStageTransition if the transition is not allowed or the
// spec is at a different stage than expected, and ErrSpecNotFound if
// the spec doesn't exist.
func (s *Store) TransitionStage(ctx context.Context, slug string, from, to storage.SpecStage) error {
	if from == storage.SpecStageApproved {
		return storage.ErrSpecAlreadyApproved
	}
	if err := storage.ValidateTransition(from, to); err != nil {
		return fmt.Errorf("postgres: %w: %w", storage.ErrInvalidStageTransition, err)
	}

	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		now := s.now()
		fromStr := string(from)
		toStr := string(to)

		setClause := "stage = $1, updated_at = $2, version = version + 1"
		args := make([]any, 0, 5)
		args = append(args, toStr, now)

		// Version guard: from-stage must match.
		whereIdx := len(args) + 1
		args = append(args, slug, s.project, fromStr)

		sql := fmt.Sprintf(
			"UPDATE specs SET %s WHERE slug = $%d AND project_slug = $%d AND stage = $%d",
			setClause, whereIdx, whereIdx+1, whereIdx+2,
		)

		tag, execErr := s.exec(txCtx, sql, args...)
		if execErr != nil {
			return fmt.Errorf("postgres: transition stage: %w", execErr)
		}
		if tag.RowsAffected() == 0 {
			// Distinguish spec-not-found from wrong-stage.
			var actualStage string
			scanErr := s.queryRow(txCtx,
				`SELECT stage FROM specs WHERE slug = $1 AND project_slug = $2`,
				slug, s.project,
			).Scan(&actualStage)
			if scanErr != nil {
				if errors.Is(scanErr, pgx.ErrNoRows) {
					return fmt.Errorf("postgres: transition stage %q: %w", slug, storage.ErrSpecNotFound)
				}
				return fmt.Errorf("postgres: transition stage: check spec: %w", scanErr)
			}
			return fmt.Errorf("postgres: spec %q at stage %q, expected %q: %w",
				slug, actualStage, from, storage.ErrInvalidStageTransition)
		}

		if hashErr := s.recomputeContentHash(txCtx, slug); hashErr != nil {
			return hashErr
		}

		// Create checkpoint changelog for the stage transition.
		updatedSpec, getErr := s.GetSpec(txCtx, slug)
		if getErr != nil {
			return getErr
		}
		deltas := []storage.FieldChange{{Field: "stage", OldValue: fromStr, NewValue: toStr}}
		clEntry := &storage.ChangeLogEntry{
			Version:     updatedSpec.Version,
			Stage:       string(updatedSpec.Stage),
			ContentHash: updatedSpec.ContentHash,
			Checkpoint:  true,
			Summary:     fmt.Sprintf("Stage transition: %s → %s", fromStr, toStr),
			Date:        updatedSpec.UpdatedAt,
		}
		if err := s.createChangeLog(txCtx, slug, clEntry, deltas); err != nil {
			return err
		}

		if to == storage.SpecStageDone {
			if err := s.RefreshDependencyHashes(txCtx, slug); err != nil {
				return fmt.Errorf("refresh dependency hashes after done transition: %w", err)
			}
		}
		return nil
	})
}

// StoreSparkOutput persists the spark stage output as JSONB on the spec row.
func (s *Store) StoreSparkOutput(ctx context.Context, slug string, output *storage.SparkOutput) error {
	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		oldFields, oldHash, err := s.readSpecFields(txCtx, slug)
		if err != nil {
			return err
		}
		if err := s.storeJSONColumn(txCtx, slug, "spark_output", output); err != nil {
			return err
		}
		return s.authoringOutputChangeLog(txCtx, slug, "spark_output", &oldFields, oldHash)
	})
}

// StoreShapeOutput persists the shape stage output as JSONB on the spec row.
// It also promotes structured decisions to first-class Decision graph nodes
// with DECIDED_IN edges (spec->decision per ADR-003).
func (s *Store) StoreShapeOutput(ctx context.Context, slug string, output *storage.ShapeOutput) error {
	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		oldFields, oldHash, err := s.readSpecFields(txCtx, slug)
		if err != nil {
			return err
		}
		if err := s.storeJSONColumn(txCtx, slug, "shape_output", output); err != nil {
			return err
		}
		// Promote decisions to graph nodes with DECIDED_IN edges.
		for i, d := range output.Decisions {
			if d.Slug == "" {
				return fmt.Errorf("decision at index %d: slug is required", i)
			}
			if d.Title == "" {
				return fmt.Errorf("decision %q: title is required", d.Slug)
			}
			// Create decision node only if it does not already exist.
			_, getErr := s.GetDecision(txCtx, d.Slug)
			if errors.Is(getErr, storage.ErrDecisionNotFound) {
				if _, createErr := s.CreateDecision(txCtx, d.Slug, d.Title, d.Body, d.Rationale,
					"", nil, "", nil, "", slug, "shape"); createErr != nil {
					return fmt.Errorf("create decision %q: %w", d.Slug, createErr)
				}
			} else if getErr != nil {
				return fmt.Errorf("check decision %q existence: %w", d.Slug, getErr)
			}
			// Always ensure the DECIDED_IN edge exists. AddEdge uses ON CONFLICT DO NOTHING.
			if _, edgeErr := s.AddEdge(txCtx, slug, d.Slug, storage.EdgeTypeDecidedIn); edgeErr != nil {
				return fmt.Errorf("add DECIDED_IN edge %q->%q: %w", slug, d.Slug, edgeErr)
			}
		}
		return s.authoringOutputChangeLog(txCtx, slug, "shape_output", &oldFields, oldHash)
	})
}

// StoreSpecifyOutput persists the specify stage output as JSONB on the spec row.
func (s *Store) StoreSpecifyOutput(ctx context.Context, slug string, output *storage.SpecifyOutput) error {
	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		oldFields, oldHash, err := s.readSpecFields(txCtx, slug)
		if err != nil {
			return err
		}
		if err := s.storeJSONColumn(txCtx, slug, "specify_output", output); err != nil {
			return err
		}
		return s.authoringOutputChangeLog(txCtx, slug, "specify_output", &oldFields, oldHash)
	})
}

// StoreDecomposeOutput persists the decompose output and creates Slice nodes.
// Returns the slugs of created (or already-existing) slices.
func (s *Store) StoreDecomposeOutput(ctx context.Context, slug string, output *storage.DecomposeOutput) ([]string, error) {
	if !output.Strategy.IsValid() {
		return nil, fmt.Errorf("postgres: invalid decomposition strategy: %q", output.Strategy)
	}

	// Pre-validate slice IDs before writing anything.
	sliceIDs := make(map[string]bool, len(output.Slices))
	for _, sl := range output.Slices {
		if sl.ID == "" {
			return nil, fmt.Errorf("postgres: decompose slice ID must not be empty")
		}
		if sliceIDs[sl.ID] {
			return nil, fmt.Errorf("postgres: duplicate decompose slice ID %q", sl.ID)
		}
		sliceIDs[sl.ID] = true
	}

	var childSlugs []string
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		oldFields, oldHash, rfErr := s.readSpecFields(txCtx, slug)
		if rfErr != nil {
			return rfErr
		}

		// Resolve the slice backend — use self if sliceOps is nil.
		sliceBackend := s.resolveSliceOps()

		// Pass 1: create all Slice nodes (with BELONGS_TO + COMPOSES edges).
		var slugs []string
		for _, sl := range output.Slices {
			childSlug := fmt.Sprintf("%s/%s", slug, sl.ID)
			resolvedDeps := make([]string, len(sl.DependsOn))
			for i, dep := range sl.DependsOn {
				resolvedDeps[i] = fmt.Sprintf("%s/%s", slug, dep)
			}
			// Check if slice already exists (idempotency for retries).
			_, getErr := sliceBackend.GetSlice(txCtx, childSlug)
			if getErr != nil {
				if !errors.Is(getErr, storage.ErrSliceNotFound) {
					return fmt.Errorf("postgres: check slice %q: %w", childSlug, getErr)
				}
				sliceDomain := &storage.Slice{
					Slug:       childSlug,
					ParentSlug: slug,
					SliceID:    sl.ID,
					Intent:     sl.Intent,
					Verify:     sl.Verify,
					Touches:    sl.Touches,
					DependsOn:  resolvedDeps,
				}
				if createErr := sliceBackend.CreateSlice(txCtx, sliceDomain); createErr != nil {
					return fmt.Errorf("postgres: create slice %q: %w", childSlug, createErr)
				}
			}
			slugs = append(slugs, childSlug)
		}

		// Pass 2: create DEPENDS_ON edges between slices.
		for _, sl := range output.Slices {
			childSlug := fmt.Sprintf("%s/%s", slug, sl.ID)
			for _, dep := range sl.DependsOn {
				if !sliceIDs[dep] {
					return fmt.Errorf("postgres: slice %q depends on unknown sibling %q", sl.ID, dep)
				}
				depSlug := fmt.Sprintf("%s/%s", slug, dep)
				_, edgeErr := s.exec(txCtx,
					`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug)
					 VALUES ($1, $2, 'DEPENDS_ON', $3)
					 ON CONFLICT (project_slug, from_slug, to_slug, edge_type) DO NOTHING`,
					childSlug, depSlug, s.project,
				)
				if edgeErr != nil {
					return fmt.Errorf("postgres: create DEPENDS_ON edge %q->%q: %w", childSlug, depSlug, edgeErr)
				}
			}
		}

		// Store slimmed output on the parent spec: strategy + slug references only.
		storedOutput := &storage.DecomposeOutput{
			Strategy:   output.Strategy,
			SliceSlugs: slugs,
		}
		if storeErr := s.storeJSONColumn(txCtx, slug, "decompose_output", storedOutput); storeErr != nil {
			return storeErr
		}

		if clErr := s.authoringOutputChangeLog(txCtx, slug, "decompose_output", &oldFields, oldHash); clErr != nil {
			return clErr
		}

		childSlugs = slugs
		return nil
	})
	return childSlugs, err
}

// StoreSafetyFlags persists safety flags as JSONB on the spec row.
// Safety flags do not affect the content hash and do not create a changelog entry.
func (s *Store) StoreSafetyFlags(ctx context.Context, slug string, flags []storage.SafetyFlag) error {
	if flags == nil {
		return fmt.Errorf("postgres: safety_flags data must not be nil")
	}
	now := s.now()
	tag, err := s.exec(ctx,
		`UPDATE specs SET safety_flags = $1, updated_at = $2
		 WHERE slug = $3 AND project_slug = $4`,
		flags, now, slug, s.project,
	)
	if err != nil {
		return fmt.Errorf("postgres: store safety_flags: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: store safety_flags for %q: %w", slug, storage.ErrSpecNotFound)
	}
	return nil
}

// SupersedeSpec marks a spec as superseded and creates a SUPERSEDES edge to the replacement.
// This is the authoring-level supersession; for lifecycle-level see LifecycleSupersedeSpec.
func (s *Store) SupersedeSpec(ctx context.Context, slug, supersededBy, reason string) error {
	// Validate both specs exist before the combined operation.
	if _, err := s.GetSpec(ctx, slug); err != nil {
		return fmt.Errorf("postgres: supersede spec: old spec %q: %w", slug, err)
	}
	if _, err := s.GetSpec(ctx, supersededBy); err != nil {
		return fmt.Errorf("postgres: supersede spec: new spec %q: %w", supersededBy, err)
	}

	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Read old spec for changelog delta.
		oldSpec, getErr := s.GetSpec(txCtx, slug)
		if getErr != nil {
			return fmt.Errorf("postgres: supersede spec: pre-read %q: %w", slug, getErr)
		}

		now := s.now()
		tag, err := s.exec(txCtx,
			`UPDATE specs SET stage = 'superseded', version = version + 1, updated_at = $1
			 WHERE slug = $2 AND project_slug = $3`,
			now, slug, s.project,
		)
		if err != nil {
			return fmt.Errorf("postgres: supersede spec: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("postgres: supersede spec %q: %w", slug, storage.ErrSpecNotFound)
		}

		// Create SUPERSEDES edge: supersededBy → slug.
		_, err = s.exec(txCtx,
			`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug)
			 VALUES ($1, $2, 'SUPERSEDES', $3)
			 ON CONFLICT (project_slug, from_slug, to_slug, edge_type) DO NOTHING`,
			supersededBy, slug, s.project,
		)
		if err != nil {
			return fmt.Errorf("postgres: supersede spec: create edge: %w", err)
		}

		// Recompute content hash and create checkpoint changelog.
		if hashErr := s.recomputeContentHash(txCtx, slug); hashErr != nil {
			return hashErr
		}
		updatedSpec, getErr := s.GetSpec(txCtx, slug)
		if getErr != nil {
			return getErr
		}
		deltas := []storage.FieldChange{
			{Field: "stage", OldValue: string(oldSpec.Stage), NewValue: string(storage.SpecStageSuperseded)},
		}
		clEntry := &storage.ChangeLogEntry{
			Version:     updatedSpec.Version,
			Stage:       string(storage.SpecStageSuperseded),
			ContentHash: updatedSpec.ContentHash,
			Checkpoint:  true,
			Summary:     "Spec superseded (authoring)",
			Reason:      reason,
			Date:        now,
		}
		return s.createChangeLog(txCtx, slug, clEntry, deltas)
	})
}

// AmendSpec moves a spec backward to an earlier stage, bumping its version.
// This is the authoring-level amendment; for lifecycle-level see LifecycleAmendSpec.
func (s *Store) AmendSpec(ctx context.Context, slug, reason string, targetStage storage.SpecStage) (*storage.AmendResult, error) { //nolint:revive // reason is part of the storage.AuthoringBackend interface
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("amend spec %q: get current: %w", slug, err)
	}
	if spec.Stage == storage.SpecStageApproved {
		return nil, storage.ErrSpecAlreadyApproved
	}
	if spec.Stage == storage.SpecStageSuperseded {
		return nil, fmt.Errorf("amend spec %q: %w", slug, storage.ErrSpecSuperseded)
	}
	if vErr := storage.ValidateAmendTransition(spec.Stage, targetStage); vErr != nil {
		return nil, fmt.Errorf("postgres: amend: %w: %w", storage.ErrInvalidStageTransition, vErr)
	}

	var result *storage.AmendResult
	txErr := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		now := s.now()
		tag, execErr := s.exec(txCtx,
			`UPDATE specs SET stage = $1, version = version + 1, updated_at = $2
			 WHERE slug = $3 AND project_slug = $4 AND version = $5`,
			string(targetStage), now, slug, s.project, spec.Version,
		)
		if execErr != nil {
			return fmt.Errorf("postgres: amend spec: %w", execErr)
		}
		if tag.RowsAffected() == 0 {
			// Version guard failed — either not found or concurrent modification.
			if _, getErr := s.GetSpec(txCtx, slug); getErr != nil {
				return fmt.Errorf("postgres: amend spec %q: %w", slug, storage.ErrSpecNotFound)
			}
			return fmt.Errorf("postgres: amend spec %q: %w", slug, storage.ErrConcurrentModification)
		}

		if hashErr := s.recomputeContentHash(txCtx, slug); hashErr != nil {
			return hashErr
		}

		updatedSpec, getErr := s.GetSpec(txCtx, slug)
		if getErr != nil {
			return getErr
		}

		result = &storage.AmendResult{
			Slug:    updatedSpec.Slug,
			Stage:   updatedSpec.Stage,
			Version: updatedSpec.Version,
		}
		return nil
	})
	return result, txErr
}

// hashInputColumns lists the spec JSONB columns that affect the content hash.
var hashInputColumns = map[string]bool{
	"spark_output":     true,
	"shape_output":     true,
	"specify_output":   true,
	"decompose_output": true,
}

// allowedJSONColumns lists the spec JSONB columns that storeJSONColumn may write.
var allowedJSONColumns = map[string]bool{
	"spark_output":     true,
	"shape_output":     true,
	"specify_output":   true,
	"decompose_output": true,
	"safety_flags":     true,
}

// storeJSONColumn updates a JSONB column on the spec row.
// Column names are validated against an allowlist.
func (s *Store) storeJSONColumn(ctx context.Context, slug, column string, data any) error {
	if data == nil {
		return fmt.Errorf("postgres: %s data must not be nil", column)
	}
	if !allowedJSONColumns[column] {
		return fmt.Errorf("postgres: disallowed column name %q", column)
	}
	// Defense-in-depth: validate characters.
	for _, r := range column {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return fmt.Errorf("postgres: unsafe column name character in %q", column)
		}
	}

	now := s.now()
	sql := fmt.Sprintf(
		"UPDATE specs SET %s = $1, updated_at = $2 WHERE slug = $3 AND project_slug = $4",
		column,
	)
	tag, err := s.exec(ctx, sql, data, now, slug, s.project)
	if err != nil {
		return fmt.Errorf("postgres: store %s: %w", column, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: store %s for %q: %w", column, slug, storage.ErrSpecNotFound)
	}
	if hashInputColumns[column] {
		if err := s.recomputeContentHash(ctx, slug); err != nil {
			return err
		}
	}
	return nil
}

// readSpecFields reads the spec's substantive fields and content hash
// for changelog delta computation. Returns ErrSpecNotFound if the spec doesn't exist.
func (s *Store) readSpecFields(ctx context.Context, slug string) (storage.SpecFields, string, error) {
	var (
		intent          string
		stage           string
		priority        string
		complexity      string
		contentHash     string
		sparkOutput     *storage.SparkOutput
		shapeOutput     *storage.ShapeOutput
		specifyOutput   *storage.SpecifyOutput
		decomposeOutput *storage.DecomposeOutput
	)
	err := s.queryRow(ctx,
		`SELECT intent, stage, priority, complexity, content_hash,
		        spark_output, shape_output, specify_output, decompose_output
		 FROM specs WHERE slug = $1 AND project_slug = $2`,
		slug, s.project,
	).Scan(&intent, &stage, &priority, &complexity, &contentHash,
		&sparkOutput, &shapeOutput, &specifyOutput, &decomposeOutput)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return storage.SpecFields{}, "", fmt.Errorf("postgres: read spec fields %q: %w", slug, storage.ErrSpecNotFound)
		}
		return storage.SpecFields{}, "", fmt.Errorf("postgres: read spec fields: %w", err)
	}

	fields := storage.SpecFields{
		Intent:     intent,
		Stage:      stage,
		Priority:   priority,
		Complexity: complexity,
	}
	if sparkOutput != nil {
		if b, mErr := json.Marshal(sparkOutput); mErr == nil {
			fields.SparkOutput = string(b)
		}
	}
	if shapeOutput != nil {
		if b, mErr := json.Marshal(shapeOutput); mErr == nil {
			fields.ShapeOutput = string(b)
		}
	}
	if specifyOutput != nil {
		if b, mErr := json.Marshal(specifyOutput); mErr == nil {
			fields.SpecifyOutput = string(b)
		}
	}
	if decomposeOutput != nil {
		if b, mErr := json.Marshal(decomposeOutput); mErr == nil {
			fields.DecomposeOutput = string(b)
		}
	}

	return fields, contentHash, nil
}

// authoringOutputChangeLog creates a non-checkpoint changelog entry after a
// Store*Output method succeeds. Only creates an entry if the content hash changed.
func (s *Store) authoringOutputChangeLog(ctx context.Context, slug, field string, oldFields *storage.SpecFields, oldHash string) error {
	newFields, newHash, err := s.readSpecFields(ctx, slug)
	if err != nil {
		return err
	}
	if newHash == oldHash {
		return nil
	}

	deltas := storage.ComputeFieldDeltas(oldFields, &newFields)

	// Read the current spec to get version and timestamp.
	var version int32
	var updatedAt time.Time
	scanErr := s.queryRow(ctx,
		`SELECT version, updated_at FROM specs WHERE slug = $1 AND project_slug = $2`,
		slug, s.project,
	).Scan(&version, &updatedAt)
	if scanErr != nil {
		return fmt.Errorf("postgres: authoring changelog: read version: %w", scanErr)
	}

	clEntry := &storage.ChangeLogEntry{
		Version:     version,
		Stage:       newFields.Stage,
		ContentHash: newHash,
		Checkpoint:  false,
		Summary:     fmt.Sprintf("Updated %s", field),
		Date:        updatedAt,
	}
	return s.createChangeLog(ctx, slug, clEntry, deltas)
}

// resolveSliceOps returns the slice backend, defaulting to self if sliceOps is nil.
func (s *Store) resolveSliceOps() storage.SliceBackend {
	if s.sliceOps != nil {
		return s.sliceOps
	}
	return s
}
