// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/seanb4t/specgraph/internal/storage"
)

// TransitionStage advances or validates a spec's stage transition.
func (s *Store) TransitionStage(ctx context.Context, slug, from, to string) error {
	if err := authoring.ValidateTransition(from, to); err != nil {
		return fmt.Errorf("memgraph: %w: %w", storage.ErrInvalidStageTransition, err)
	}
	nowStr := time.Now().UTC().Format(time.RFC3339)
	query := `
		MATCH (s:Spec {slug: $slug})
		WHERE s.stage = $from OR ($from = "" AND (s.stage IS NULL OR s.stage = ""))
		SET s.stage = $to, s.updated_at = $updated_at
		RETURN s.slug
	`
	result, err := neo4j.ExecuteQuery(ctx, s.driver, query,
		map[string]any{"slug": slug, "from": from, "to": to, "updated_at": nowStr},
		neo4j.EagerResultTransformer)
	if err != nil {
		return fmt.Errorf("memgraph: transition stage: %w", err)
	}
	if len(result.Records) == 0 {
		return fmt.Errorf("memgraph: transition stage %q: %w", slug, storage.ErrSpecNotFound)
	}
	return nil
}

// StoreSparkOutput persists the spark stage output as JSON on the spec node.
func (s *Store) StoreSparkOutput(ctx context.Context, slug string, output *specv1.SparkOutput) error {
	return s.storeJSONProperty(ctx, slug, "spark_output", output)
}

// StoreShapeOutput persists the shape stage output as JSON on the spec node.
func (s *Store) StoreShapeOutput(ctx context.Context, slug string, output *specv1.ShapeOutput) error {
	return s.storeJSONProperty(ctx, slug, "shape_output", output)
}

// StoreSpecifyOutput persists the specify stage output as JSON on the spec node.
func (s *Store) StoreSpecifyOutput(ctx context.Context, slug string, output *specv1.SpecifyOutput) error {
	return s.storeJSONProperty(ctx, slug, "specify_output", output)
}

// StoreDecomposeOutput persists the decompose output and creates child spec nodes with edges.
func (s *Store) StoreDecomposeOutput(ctx context.Context, slug string, output *specv1.DecomposeOutput) ([]*specv1.Spec, error) {
	if err := s.storeJSONProperty(ctx, slug, "decompose_output", output); err != nil {
		return nil, err
	}
	var children []*specv1.Spec
	for _, sl := range output.Slices {
		child, err := s.CreateSpec(ctx, sl.Id, sl.Intent, "p2", "medium")
		if err != nil {
			return nil, fmt.Errorf("memgraph: create child spec %q: %w", sl.Id, err)
		}
		query := `
			MATCH (child:Spec {slug: $child_slug}), (parent:Spec {slug: $parent_slug})
			CREATE (child)-[:COMPOSES]->(parent)
		`
		_, err = neo4j.ExecuteQuery(ctx, s.driver, query,
			map[string]any{"child_slug": sl.Id, "parent_slug": slug},
			neo4j.EagerResultTransformer)
		if err != nil {
			return nil, fmt.Errorf("memgraph: create COMPOSES edge: %w", err)
		}
		for _, dep := range sl.DependsOn {
			depQuery := `
				MATCH (from:Spec {slug: $from_slug}), (to:Spec {slug: $to_slug})
				CREATE (from)-[:DEPENDS_ON]->(to)
			`
			_, err = neo4j.ExecuteQuery(ctx, s.driver, depQuery,
				map[string]any{"from_slug": sl.Id, "to_slug": dep},
				neo4j.EagerResultTransformer)
			if err != nil {
				return nil, fmt.Errorf("memgraph: create DEPENDS_ON edge: %w", err)
			}
		}
		children = append(children, child)
	}
	return children, nil
}

// StoreRedTeamFindings persists red team findings as JSON on the spec node.
func (s *Store) StoreRedTeamFindings(ctx context.Context, slug string, findings []*specv1.RedTeamFinding) error {
	return s.storeJSONProperty(ctx, slug, "red_team_findings", findings)
}

// StorePeripheralVision persists peripheral vision items as JSON on the spec node.
func (s *Store) StorePeripheralVision(ctx context.Context, slug string, items []*specv1.PeripheralVisionItem) error {
	return s.storeJSONProperty(ctx, slug, "peripheral_vision", items)
}

// StoreConsistencyIssues persists consistency issues as JSON on the spec node.
func (s *Store) StoreConsistencyIssues(ctx context.Context, slug string, issues []*specv1.ConsistencyIssue) error {
	return s.storeJSONProperty(ctx, slug, "consistency_issues", issues)
}

// StoreSimplicityFindings persists simplicity findings as JSON on the spec node.
func (s *Store) StoreSimplicityFindings(ctx context.Context, slug string, findings []*specv1.SimplicityFinding) error {
	return s.storeJSONProperty(ctx, slug, "simplicity_findings", findings)
}

// StoreSafetyFlags persists safety flags as JSON on the spec node.
func (s *Store) StoreSafetyFlags(ctx context.Context, slug string, flags []*specv1.SafetyFlag) error {
	return s.storeJSONProperty(ctx, slug, "safety_flags", flags)
}

// StoreConstitutionViolations persists constitution violations as JSON on the spec node.
func (s *Store) StoreConstitutionViolations(ctx context.Context, slug string, violations []*specv1.ConstitutionViolation) error {
	return s.storeJSONProperty(ctx, slug, "constitution_violations", violations)
}

// SupersedeSpec marks a spec as superseded and creates a SUPERSEDES edge to the replacement.
func (s *Store) SupersedeSpec(ctx context.Context, slug, supersededBy, reason string) error {
	nowStr := time.Now().UTC().Format(time.RFC3339)
	query := `
		MATCH (old:Spec {slug: $old_slug}), (new:Spec {slug: $new_slug})
		SET old.stage = "superseded", old.updated_at = $updated_at
		CREATE (new)-[:SUPERSEDES {reason: $reason}]->(old)
		RETURN old.slug
	`
	result, err := neo4j.ExecuteQuery(ctx, s.driver, query,
		map[string]any{"old_slug": slug, "new_slug": supersededBy, "reason": reason, "updated_at": nowStr},
		neo4j.EagerResultTransformer)
	if err != nil {
		return fmt.Errorf("memgraph: supersede spec: %w", err)
	}
	if len(result.Records) == 0 {
		return fmt.Errorf("memgraph: supersede spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	return nil
}

// AmendSpec moves a spec backward to an earlier stage, bumping its version.
func (s *Store) AmendSpec(ctx context.Context, slug, reason, targetStage string) (*specv1.Spec, error) {
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, err
	}
	if vErr := authoring.ValidateTransition(spec.Stage, targetStage); vErr != nil {
		return nil, fmt.Errorf("memgraph: amend: %w: %w", storage.ErrInvalidStageTransition, vErr)
	}
	nowStr := time.Now().UTC().Format(time.RFC3339)
	query := `
		MATCH (s:Spec {slug: $slug})
		SET s.stage = $stage, s.amend_reason = $reason,
		    s.version = s.version + 1, s.updated_at = $updated_at
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity, s.version,
		       s.created_at, s.updated_at
	`
	result, err := neo4j.ExecuteQuery(ctx, s.driver, query,
		map[string]any{"slug": slug, "stage": targetStage, "reason": reason, "updated_at": nowStr},
		neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: amend spec: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: amend spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	return recordToSpec(result.Records[0])
}

// storeJSONProperty marshals data to JSON and stores it as a string property on the spec node.
func (s *Store) storeJSONProperty(ctx context.Context, slug, property string, data any) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("memgraph: marshal %s: %w", property, err)
	}
	nowStr := time.Now().UTC().Format(time.RFC3339)
	query := fmt.Sprintf(`
		MATCH (s:Spec {slug: $slug})
		SET s.%s = $data, s.updated_at = $updated_at
		RETURN s.slug
	`, property)
	result, err := neo4j.ExecuteQuery(ctx, s.driver, query,
		map[string]any{"slug": slug, "data": string(jsonBytes), "updated_at": nowStr},
		neo4j.EagerResultTransformer)
	if err != nil {
		return fmt.Errorf("memgraph: store %s: %w", property, err)
	}
	if len(result.Records) == 0 {
		return fmt.Errorf("memgraph: store %s for %q: %w", property, slug, storage.ErrSpecNotFound)
	}
	return nil
}
