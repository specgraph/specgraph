// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"errors"
	"fmt"

	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/seanb4t/specgraph/internal/storage"
)

const (
	defaultChildPriority   = "p2"
	defaultChildComplexity = "medium"
)

// allowedJSONProperties lists the spec node properties that storeJSONProperty may write.
var allowedJSONProperties = map[string]bool{
	"spark_output":            true,
	"shape_output":            true,
	"specify_output":          true,
	"decompose_output":        true,
	"red_team_findings":       true,
	"peripheral_vision":       true,
	"consistency_issues":      true,
	"simplicity_findings":     true,
	"safety_flags":            true,
	"constitution_violations": true,
}

// TransitionStage validates and applies a spec's stage transition.
// It first checks the transition is valid via authoring.ValidateTransition,
// then updates the spec's stage in the database. Returns ErrSpecNotFound if
// the spec doesn't exist, or ErrInvalidStageTransition if the spec is at
// a different stage than expected. Returns ErrSpecAlreadyApproved if from
// is the approved stage.
func (s *Store) TransitionStage(ctx context.Context, slug string, from, to storage.AuthoringStage) error {
	if from == authoring.StageApproved {
		return storage.ErrSpecAlreadyApproved
	}
	if err := authoring.ValidateTransition(string(from), string(to)); err != nil {
		return fmt.Errorf("memgraph: %w: %w", storage.ErrInvalidStageTransition, err)
	}
	nowStr := nowRFC3339()
	fromStr := string(from)
	toStr := string(to)
	query := `
		MATCH (s:Spec {slug: $slug})
		WHERE s.stage = $from OR ($from = "" AND (s.stage IS NULL OR s.stage = ""))
		SET s.stage = $to, s.updated_at = $updated_at
		RETURN s.slug
	`
	records, err := s.executeQuery(ctx, query,
		map[string]any{"slug": slug, "from": fromStr, "to": toStr, "updated_at": nowStr})
	if err != nil {
		return fmt.Errorf("memgraph: transition stage: %w", err)
	}
	if len(records) == 0 {
		// Distinguish between "spec not found" and "spec at wrong stage".
		checkQuery := `MATCH (s:Spec {slug: $slug}) RETURN s.stage AS stage`
		checkRecords, checkErr := s.executeQuery(ctx, checkQuery,
			map[string]any{"slug": slug})
		if checkErr != nil {
			return fmt.Errorf("memgraph: check spec stage: %w", checkErr)
		}
		if len(checkRecords) == 0 {
			return fmt.Errorf("memgraph: transition stage %q: %w", slug, storage.ErrSpecNotFound)
		}
		actualStage, _ := checkRecords[0].Get("stage")
		return fmt.Errorf("memgraph: spec %q at stage %v, expected %q: %w", slug, actualStage, from, storage.ErrInvalidStageTransition)
	}
	return nil
}

// StoreSparkOutput persists the spark stage output as JSON on the spec node.
func (s *Store) StoreSparkOutput(ctx context.Context, slug string, output *storage.SparkOutput) error {
	return s.storeJSONProperty(ctx, slug, "spark_output", output)
}

// StoreShapeOutput persists the shape stage output as JSON on the spec node.
func (s *Store) StoreShapeOutput(ctx context.Context, slug string, output *storage.ShapeOutput) error {
	return s.storeJSONProperty(ctx, slug, "shape_output", output)
}

// StoreSpecifyOutput persists the specify stage output as JSON on the spec node.
func (s *Store) StoreSpecifyOutput(ctx context.Context, slug string, output *storage.SpecifyOutput) error {
	return s.storeJSONProperty(ctx, slug, "specify_output", output)
}

// StoreDecomposeOutput persists the decompose output and creates child spec nodes with edges.
// When called within a transaction (via RunInTransaction), partial failures roll back automatically.
// Without a transaction, child spec creation uses MERGE for idempotency on retries.
// It returns the slugs of the created (or already-existing) child specs.
func (s *Store) StoreDecomposeOutput(ctx context.Context, slug string, output *storage.DecomposeOutput) ([]string, error) {
	if err := s.storeJSONProperty(ctx, slug, "decompose_output", output); err != nil {
		return nil, err
	}
	var childSlugs []string
	for _, sl := range output.Slices {
		childSlug := fmt.Sprintf("%s/%s", slug, sl.ID)
		// Check if child spec already exists (idempotency for retries).
		_, getErr := s.GetSpec(ctx, childSlug)
		if getErr != nil {
			if !errors.Is(getErr, storage.ErrSpecNotFound) {
				return nil, fmt.Errorf("memgraph: check child spec %q: %w", childSlug, getErr)
			}
			// Not found — proceed to create.
			if _, err := s.CreateSpec(ctx, childSlug, sl.Intent, defaultChildPriority, defaultChildComplexity); err != nil {
				return nil, fmt.Errorf("memgraph: create child spec %q: %w", childSlug, err)
			}
		}
		// If getErr == nil, child spec already exists (idempotent retry).
		query := `
			MATCH (child:Spec {slug: $child_slug}), (parent:Spec {slug: $parent_slug})
			MERGE (child)-[:COMPOSES]->(parent)
		`
		_, err := s.executeQuery(ctx, query,
			map[string]any{"child_slug": childSlug, "parent_slug": slug})
		if err != nil {
			return nil, fmt.Errorf("memgraph: merge COMPOSES edge: %w", err)
		}
		for _, dep := range sl.DependsOn {
			depSlug := fmt.Sprintf("%s/%s", slug, dep)
			depQuery := `
				MATCH (from:Spec {slug: $from_slug}), (to:Spec {slug: $to_slug})
				MERGE (from)-[:DEPENDS_ON]->(to)
			`
			_, err = s.executeQuery(ctx, depQuery,
				map[string]any{"from_slug": childSlug, "to_slug": depSlug})
			if err != nil {
				return nil, fmt.Errorf("memgraph: merge DEPENDS_ON edge: %w", err)
			}
		}
		childSlugs = append(childSlugs, childSlug)
	}
	return childSlugs, nil
}

// --- Analytical pass storage (thin wrappers over storeJSONProperty) ---

// StoreRedTeamFindings persists red team findings as JSON on the spec node.
func (s *Store) StoreRedTeamFindings(ctx context.Context, slug string, findings []storage.RedTeamFinding) error {
	return s.storeJSONProperty(ctx, slug, "red_team_findings", findings)
}

// StorePeripheralVision persists peripheral vision items as JSON on the spec node.
func (s *Store) StorePeripheralVision(ctx context.Context, slug string, items []storage.PeripheralVisionItem) error {
	return s.storeJSONProperty(ctx, slug, "peripheral_vision", items)
}

// StoreConsistencyIssues persists consistency issues as JSON on the spec node.
func (s *Store) StoreConsistencyIssues(ctx context.Context, slug string, issues []storage.ConsistencyIssue) error {
	return s.storeJSONProperty(ctx, slug, "consistency_issues", issues)
}

// StoreSimplicityFindings persists simplicity findings as JSON on the spec node.
func (s *Store) StoreSimplicityFindings(ctx context.Context, slug string, findings []storage.SimplicityFinding) error {
	return s.storeJSONProperty(ctx, slug, "simplicity_findings", findings)
}

// StoreSafetyFlags persists safety flags as JSON on the spec node.
func (s *Store) StoreSafetyFlags(ctx context.Context, slug string, flags []storage.SafetyFlag) error {
	return s.storeJSONProperty(ctx, slug, "safety_flags", flags)
}

// StoreConstitutionViolations persists constitution violations as JSON on the spec node.
func (s *Store) StoreConstitutionViolations(ctx context.Context, slug string, violations []storage.ConstitutionViolation) error {
	return s.storeJSONProperty(ctx, slug, "constitution_violations", violations)
}

// SupersedeSpec marks a spec as superseded and creates a SUPERSEDES edge to the replacement.
func (s *Store) SupersedeSpec(ctx context.Context, slug, supersededBy, reason string) error {
	// Validate both specs exist before the combined operation so callers get
	// a precise error identifying which slug was missing.
	if _, err := s.GetSpec(ctx, slug); err != nil {
		return fmt.Errorf("memgraph: supersede spec: old spec %q: %w", slug, err)
	}
	if _, err := s.GetSpec(ctx, supersededBy); err != nil {
		return fmt.Errorf("memgraph: supersede spec: new spec %q: %w", supersededBy, err)
	}
	nowStr := nowRFC3339()
	query := `
		MATCH (old:Spec {slug: $old_slug}), (new:Spec {slug: $new_slug})
		SET old.stage = "superseded", old.updated_at = $updated_at
		CREATE (new)-[:SUPERSEDES {reason: $reason}]->(old)
		RETURN old.slug
	`
	records, err := s.executeQuery(ctx, query,
		map[string]any{"old_slug": slug, "new_slug": supersededBy, "reason": reason, "updated_at": nowStr})
	if err != nil {
		return fmt.Errorf("memgraph: supersede spec: %w", err)
	}
	if len(records) == 0 {
		return fmt.Errorf("memgraph: supersede spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	return nil
}

// AmendSpec moves a spec backward to an earlier stage, bumping its version.
func (s *Store) AmendSpec(ctx context.Context, slug, reason string, targetStage storage.AuthoringStage) (*storage.AmendResult, error) {
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, err
	}
	if spec.Stage == authoring.StageApproved {
		return nil, storage.ErrSpecAlreadyApproved
	}
	if vErr := authoring.ValidateAmendTransition(spec.Stage, string(targetStage)); vErr != nil {
		return nil, fmt.Errorf("memgraph: amend: %w: %w", storage.ErrInvalidStageTransition, vErr)
	}
	nowStr := nowRFC3339()
	query := `
		MATCH (s:Spec {slug: $slug})
		SET s.stage = $stage, s.amend_reason = $reason,
		    s.version = s.version + 1, s.updated_at = $updated_at
		RETURN s.slug, s.stage, s.version
	`
	records, err := s.executeQuery(ctx, query,
		map[string]any{"slug": slug, "stage": string(targetStage), "reason": reason, "updated_at": nowStr})
	if err != nil {
		return nil, fmt.Errorf("memgraph: amend spec: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: amend spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	rec := records[0]
	retSlug, _ := rec.Get("s.slug")
	retStage, _ := rec.Get("s.stage")
	retVersion, _ := rec.Get("s.version")
	result := &storage.AmendResult{
		Slug:  fmt.Sprintf("%v", retSlug),
		Stage: fmt.Sprintf("%v", retStage),
	}
	if v, ok := retVersion.(int64); ok {
		result.Version = safeInt32(v)
	}
	return result, nil
}

// storeJSONProperty marshals data to JSON and stores it as a string property on the spec node.
// Property names must be interpolated into the Cypher query (parameterized property names are
// not supported by Cypher). The allowlist is the primary defense against Cypher injection;
// character validation provides a secondary check.
func (s *Store) storeJSONProperty(ctx context.Context, slug, property string, data any) error {
	if data == nil {
		return fmt.Errorf("memgraph: %s data must not be nil", property)
	}
	if !allowedJSONProperties[property] {
		return fmt.Errorf("memgraph: disallowed property name %q", property)
	}
	// Defense-in-depth: validate property name contains only safe characters.
	for _, r := range property {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return fmt.Errorf("memgraph: unsafe property name character in %q", property)
		}
	}
	jsonStr, err := marshalJSON(data)
	if err != nil {
		return fmt.Errorf("memgraph: marshal %s: %w", property, err)
	}
	nowStr := nowRFC3339()
	query := fmt.Sprintf(`
		MATCH (s:Spec {slug: $slug})
		SET s.%s = $data, s.updated_at = $updated_at
		RETURN s.slug
	`, property)
	records, err := s.executeQuery(ctx, query,
		map[string]any{"slug": slug, "data": jsonStr, "updated_at": nowStr})
	if err != nil {
		return fmt.Errorf("memgraph: store %s: %w", property, err)
	}
	if len(records) == 0 {
		return fmt.Errorf("memgraph: store %s for %q: %w", property, slug, storage.ErrSpecNotFound)
	}
	return nil
}
