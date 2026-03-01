// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/storage"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GetConstitution returns the active constitution node.
func (s *Store) GetConstitution(ctx context.Context) (*specv1.Constitution, error) {
	query := `
		MATCH (c:Constitution)
		RETURN c.id, c.layer, c.name, c.version, c.tech_json,
		       c.principles_json, c.process_json, c.constraints_json,
		       c.antipatterns_json, c.references_json,
		       c.created_at, c.updated_at
	`

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, nil, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get constitution: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: %w", storage.ErrConstitutionNotFound)
	}

	return recordToConstitution(result.Records[0])
}

// UpdateConstitution stores or replaces the constitution, bumping its version.
// Uses MERGE so there is always at most one Constitution node.
func (s *Store) UpdateConstitution(ctx context.Context, constitution *specv1.Constitution) (*specv1.Constitution, error) {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	techJSON, err := marshalJSON(constitution.Tech)
	if err != nil {
		return nil, fmt.Errorf("memgraph: marshal tech: %w", err)
	}
	principlesJSON, err := marshalJSON(constitution.Principles)
	if err != nil {
		return nil, fmt.Errorf("memgraph: marshal principles: %w", err)
	}
	processJSON, err := marshalJSON(constitution.Process)
	if err != nil {
		return nil, fmt.Errorf("memgraph: marshal process: %w", err)
	}
	constraintsJSON, err := marshalJSON(constitution.Constraints)
	if err != nil {
		return nil, fmt.Errorf("memgraph: marshal constraints: %w", err)
	}
	antipatternsJSON, err := marshalJSON(constitution.Antipatterns)
	if err != nil {
		return nil, fmt.Errorf("memgraph: marshal antipatterns: %w", err)
	}
	referencesJSON, err := marshalJSON(constitution.References)
	if err != nil {
		return nil, fmt.Errorf("memgraph: marshal references: %w", err)
	}

	id := constitution.Id
	if id == "" {
		id = generateID("con", constitution.Name, now)
	}

	query := `
		MERGE (c:Constitution)
		ON CREATE SET
			c.id = $id,
			c.version = 1,
			c.created_at = $now
		ON MATCH SET
			c.version = c.version + 1
		SET
			c.layer = $layer,
			c.name = $name,
			c.tech_json = $tech_json,
			c.principles_json = $principles_json,
			c.process_json = $process_json,
			c.constraints_json = $constraints_json,
			c.antipatterns_json = $antipatterns_json,
			c.references_json = $references_json,
			c.updated_at = $now
		RETURN c.id, c.layer, c.name, c.version, c.tech_json,
		       c.principles_json, c.process_json, c.constraints_json,
		       c.antipatterns_json, c.references_json,
		       c.created_at, c.updated_at
	`
	params := map[string]any{
		"id":                id,
		"layer":             constitution.Layer.String(),
		"name":              constitution.Name,
		"tech_json":         techJSON,
		"principles_json":   principlesJSON,
		"process_json":      processJSON,
		"constraints_json":  constraintsJSON,
		"antipatterns_json": antipatternsJSON,
		"references_json":   referencesJSON,
		"now":               nowStr,
	}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: update constitution: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: update constitution returned no records")
	}

	return recordToConstitution(result.Records[0])
}

// CheckViolation checks a spec against constitution constraints.
// Returns empty violations for now (full implementation in Slice 5).
func (s *Store) CheckViolation(ctx context.Context, specSlug string) ([]*specv1.Violation, error) {
	// Verify the spec exists.
	_, err := s.GetSpec(ctx, specSlug)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, fmt.Errorf("memgraph: check violation spec %q: %w", specSlug, storage.ErrSpecNotFound)
		}
		return nil, fmt.Errorf("memgraph: check violation: %w", err)
	}

	// Verify a constitution exists.
	_, err = s.GetConstitution(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrConstitutionNotFound) {
			return nil, fmt.Errorf("memgraph: check violation: %w", storage.ErrConstitutionNotFound)
		}
		return nil, fmt.Errorf("memgraph: check violation: %w", err)
	}

	// Full violation checking comes in Slice 5; return empty for now.
	return []*specv1.Violation{}, nil
}

// marshalJSON marshals v to a JSON string. Returns "{}" for nil values.
func marshalJSON(v any) (string, error) {
	if v == nil {
		return "{}", nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal json: %w", err)
	}
	return string(b), nil
}

// recordToConstitution converts a neo4j record (positional) to a *specv1.Constitution.
// Column order must match the RETURN clause in GetConstitution / UpdateConstitution.
func recordToConstitution(rec *neo4j.Record) (*specv1.Constitution, error) {
	id, err := recordString(rec, 0, "id")
	if err != nil {
		return nil, err
	}
	layerStr, err := recordString(rec, 1, "layer")
	if err != nil {
		return nil, err
	}
	name, err := recordString(rec, 2, "name")
	if err != nil {
		return nil, err
	}
	version, err := recordInt64(rec, 3, "version")
	if err != nil {
		return nil, err
	}
	techJSON, err := recordString(rec, 4, "tech_json")
	if err != nil {
		return nil, err
	}
	principlesJSON, err := recordString(rec, 5, "principles_json")
	if err != nil {
		return nil, err
	}
	processJSON, err := recordString(rec, 6, "process_json")
	if err != nil {
		return nil, err
	}
	constraintsJSON, err := recordString(rec, 7, "constraints_json")
	if err != nil {
		return nil, err
	}
	antipatternsJSON, err := recordString(rec, 8, "antipatterns_json")
	if err != nil {
		return nil, err
	}
	referencesJSON, err := recordString(rec, 9, "references_json")
	if err != nil {
		return nil, err
	}
	createdAtStr, err := recordString(rec, 10, "created_at")
	if err != nil {
		return nil, err
	}
	updatedAtStr, err := recordString(rec, 11, "updated_at")
	if err != nil {
		return nil, err
	}

	createdAt, err := parseRFC3339("created_at", createdAtStr)
	if err != nil {
		return nil, err
	}
	updatedAt, err := parseRFC3339("updated_at", updatedAtStr)
	if err != nil {
		return nil, err
	}

	layerVal, ok := specv1.ConstitutionLayer_value[layerStr]
	if !ok {
		layerVal = int32(specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED)
	}

	var tech specv1.TechConfig
	if techJSON != "" && techJSON != "{}" {
		if err := json.Unmarshal([]byte(techJSON), &tech); err != nil {
			return nil, fmt.Errorf("memgraph: unmarshal tech: %w", err)
		}
	}

	var principles []*specv1.Principle
	if principlesJSON != "" && principlesJSON != "null" {
		if err := json.Unmarshal([]byte(principlesJSON), &principles); err != nil {
			return nil, fmt.Errorf("memgraph: unmarshal principles: %w", err)
		}
	}

	var process specv1.ProcessConfig
	if processJSON != "" && processJSON != "{}" {
		if err := json.Unmarshal([]byte(processJSON), &process); err != nil {
			return nil, fmt.Errorf("memgraph: unmarshal process: %w", err)
		}
	}

	var constraints []string
	if constraintsJSON != "" && constraintsJSON != "null" {
		if err := json.Unmarshal([]byte(constraintsJSON), &constraints); err != nil {
			return nil, fmt.Errorf("memgraph: unmarshal constraints: %w", err)
		}
	}

	var antipatterns []*specv1.Antipattern
	if antipatternsJSON != "" && antipatternsJSON != "null" {
		if err := json.Unmarshal([]byte(antipatternsJSON), &antipatterns); err != nil {
			return nil, fmt.Errorf("memgraph: unmarshal antipatterns: %w", err)
		}
	}

	var references []*specv1.Reference
	if referencesJSON != "" && referencesJSON != "null" {
		if err := json.Unmarshal([]byte(referencesJSON), &references); err != nil {
			return nil, fmt.Errorf("memgraph: unmarshal references: %w", err)
		}
	}

	c := &specv1.Constitution{
		Id:           id,
		Layer:        specv1.ConstitutionLayer(layerVal),
		Name:         name,
		Version:      int32(version), //nolint:gosec // version values are small positive integers
		Constraints:  constraints,
		Antipatterns: antipatterns,
		References:   references,
		CreatedAt:    timestamppb.New(createdAt),
		UpdatedAt:    timestamppb.New(updatedAt),
	}

	// Only set pointer fields if non-empty to avoid spurious non-nil empty structs.
	if techJSON != "{}" && techJSON != "" {
		c.Tech = &tech
	}
	if processJSON != "{}" && processJSON != "" {
		c.Process = &process
	}
	if len(principles) > 0 {
		c.Principles = principles
	}

	return c, nil
}
