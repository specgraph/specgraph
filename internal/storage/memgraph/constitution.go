// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/specgraph/specgraph/internal/storage"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// layerFromString maps stored layer strings (both legacy proto enum names and
// new domain-type values) to the corresponding ConstitutionLayer constant.
var layerFromString = map[string]storage.ConstitutionLayer{
	// Legacy proto enum format (existing Memgraph data).
	"CONSTITUTION_LAYER_USER":    storage.ConstitutionLayerUser,
	"CONSTITUTION_LAYER_ORG":     storage.ConstitutionLayerOrg,
	"CONSTITUTION_LAYER_PROJECT": storage.ConstitutionLayerProject,
	"CONSTITUTION_LAYER_DOMAIN":  storage.ConstitutionLayerDomain,
	// Domain-type format (new data written by this code).
	"user":    storage.ConstitutionLayerUser,
	"org":     storage.ConstitutionLayerOrg,
	"project": storage.ConstitutionLayerProject,
	"domain":  storage.ConstitutionLayerDomain,
}

// GetConstitution returns the active constitution node.
func (s *Store) GetConstitution(ctx context.Context) (*storage.Constitution, error) {
	query := `
		MATCH (p:Project {slug: $project})-[:HAS_CONSTITUTION]->(c:Constitution)
		RETURN c.id, c.layer, c.name, c.version, c.tech_json,
		       c.principles_json, c.process_json, c.constraints_json,
		       c.antipatterns_json, c.references_json,
		       c.created_at, c.updated_at
	`

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, s.projectParam(), neo4j.EagerResultTransformer)
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
func (s *Store) UpdateConstitution(ctx context.Context, constitution *storage.Constitution) (*storage.Constitution, error) {
	now := s.nowTime()
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

	id := constitution.ID
	if id == "" {
		id = newID("con")
	}

	query := `
		MATCH (p:Project {slug: $project})
		MERGE (p)-[:HAS_CONSTITUTION]->(c:Constitution)
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
	params := mergeParams(s.projectParam(), map[string]any{
		"id":                id,
		"layer":             string(constitution.Layer),
		"name":              constitution.Name,
		"tech_json":         techJSON,
		"principles_json":   principlesJSON,
		"process_json":      processJSON,
		"constraints_json":  constraintsJSON,
		"antipatterns_json": antipatternsJSON,
		"references_json":   referencesJSON,
		"now":               nowStr,
	})

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: update constitution: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: update constitution returned no records")
	}

	return recordToConstitution(result.Records[0])
}

// marshalJSON marshals v to a JSON string. Nil pointers produce "null"; nil slices produce "null".
func marshalJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal json: %w", err)
	}
	return string(b), nil
}

// recordStringByName extracts a string value from a neo4j record by column name.
func recordStringByName(rec *neo4j.Record, key string) (string, error) {
	val, ok := rec.Get(key)
	if !ok {
		return "", fmt.Errorf("memgraph: column %q not found in record", key)
	}
	s, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("memgraph: column %q: expected string, got %T", key, val)
	}
	return s, nil
}

// recordInt64ByName extracts an int64 value from a neo4j record by column name.
func recordInt64ByName(rec *neo4j.Record, key string) (int64, error) {
	val, ok := rec.Get(key)
	if !ok {
		return 0, fmt.Errorf("memgraph: column %q not found in record", key)
	}
	n, ok := val.(int64)
	if !ok {
		return 0, fmt.Errorf("memgraph: column %q: expected int64, got %T", key, val)
	}
	return n, nil
}

// unmarshalIfPresent unmarshals jsonStr into dest if it contains meaningful data.
// Considers "", "{}", "[]", and "null" as empty sentinels that should be skipped.
func unmarshalIfPresent(jsonStr, field string, dest any) error {
	switch jsonStr {
	case "", "{}", "[]", "null":
		return nil
	}
	if err := json.Unmarshal([]byte(jsonStr), dest); err != nil {
		return fmt.Errorf("memgraph: unmarshal %s: %w", field, err)
	}
	return nil
}

// recordToConstitution converts a neo4j record to a *storage.Constitution using named column access.
func recordToConstitution(rec *neo4j.Record) (*storage.Constitution, error) {
	id, err := recordStringByName(rec, "c.id")
	if err != nil {
		return nil, err
	}
	layerStr, err := recordStringByName(rec, "c.layer")
	if err != nil {
		return nil, err
	}
	name, err := recordStringByName(rec, "c.name")
	if err != nil {
		return nil, err
	}
	version, err := recordInt64ByName(rec, "c.version")
	if err != nil {
		return nil, err
	}
	techJSON, err := recordStringByName(rec, "c.tech_json")
	if err != nil {
		return nil, err
	}
	principlesJSON, err := recordStringByName(rec, "c.principles_json")
	if err != nil {
		return nil, err
	}
	processJSON, err := recordStringByName(rec, "c.process_json")
	if err != nil {
		return nil, err
	}
	constraintsJSON, err := recordStringByName(rec, "c.constraints_json")
	if err != nil {
		return nil, err
	}
	antipatternsJSON, err := recordStringByName(rec, "c.antipatterns_json")
	if err != nil {
		return nil, err
	}
	referencesJSON, err := recordStringByName(rec, "c.references_json")
	if err != nil {
		return nil, err
	}
	createdAtStr, err := recordStringByName(rec, "c.created_at")
	if err != nil {
		return nil, err
	}
	updatedAtStr, err := recordStringByName(rec, "c.updated_at")
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

	var tech storage.TechStack
	if err := unmarshalIfPresent(techJSON, "tech", &tech); err != nil {
		return nil, err
	}

	var principles []storage.Principle
	if err := unmarshalIfPresent(principlesJSON, "principles", &principles); err != nil {
		return nil, err
	}

	var process storage.ProcessConfig
	if err := unmarshalIfPresent(processJSON, "process", &process); err != nil {
		return nil, err
	}

	var constraints []string
	if err := unmarshalIfPresent(constraintsJSON, "constraints", &constraints); err != nil {
		return nil, err
	}

	var antipatterns []storage.Antipattern
	if err := unmarshalIfPresent(antipatternsJSON, "antipatterns", &antipatterns); err != nil {
		return nil, err
	}

	var references []storage.Reference
	if err := unmarshalIfPresent(referencesJSON, "references", &references); err != nil {
		return nil, err
	}

	c := &storage.Constitution{
		ID:          id,
		Layer:       layerFromString[layerStr],
		Name:        name,
		Version:     int32(version), //nolint:gosec // version values are small positive integers
		Constraints: constraints,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}

	// Only set pointer fields when the struct was actually populated.
	if tech.Languages != nil || len(tech.Frameworks) > 0 || len(tech.Infrastructure) > 0 || len(tech.APIStandards) > 0 || len(tech.Data) > 0 {
		c.Tech = &tech
	}
	if process.SpecReview != "" || process.SecurityReview != nil || process.Deployment != nil || process.Documentation != nil {
		c.Process = &process
	}
	if len(principles) > 0 {
		c.Principles = principles
	}
	if len(antipatterns) > 0 {
		c.Antipatterns = antipatterns
	}
	if len(references) > 0 {
		c.References = references
	}

	return c, nil
}
